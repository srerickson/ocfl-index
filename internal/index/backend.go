package index

import (
	"context"
	"errors"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrMissingValue = errors.New("missing value")
	ErrInvalidArgs  = errors.New("invalid arguments")
	ErrIndexValue   = errors.New("unexpected value in index, possible corruption")
)

// Backend is an interface that can be implemented for different databases for
// storing the indexing.
type Backend interface {
	NewTx(context.Context) (BackendTx, error)

	// GetIndexSummary returns stats on indexed objects
	GetIndexSummary(ctx context.Context) (IndexSummary, error)

	// ListObjectRoots is used to iterate over the object root directories in the index.
	// Paths in the returned list are relative to the storage root.
	ListObjectRoots(ctx context.Context, limit int, cursor string) (*ObjectRootList, error)

	// All OCFL Object in the index
	ListObjects(ctx context.Context, order ObjectSort, limit int, cursor string) (*ObjectList, error)
	GetObject(ctx context.Context, objectID string) (*Object, error)
	GetObjectByPath(ctx context.Context, rootPath string) (*Object, error)

	// GetObjectState returns a path list representing files and directories in an
	// object version state (i.e., the "logical state").
	GetObjectState(ctx context.Context, objectID string, vnum ocfl.VNum, base string, recursive bool, limit int, cursor string) (*PathInfo, error)

	// GetContentPath returns the path to a file with digest sum. The path is relative to
	// the storage root.
	GetContentPath(ctx context.Context, sum string) (string, error)
}

type BackendTx interface {
	Rollback() error
	Commit() error

	// IndexObjectRoot adds the object root directory to the index, effectively
	// declaring that an object exists at the path without fully indexing its
	// inventory. This is a minimal indexing operation and all other Index
	// methods include it. The object root is relative to the storage root path.
	// If root is already present in the index, its indexed timestamp is updated
	// to idxAt (which should typically be time.Now()) and nil is returned. The
	// timestamp, idxAt, is truncated to the nearest second and converted to UTC
	// before being stored in the index.
	IndexObjectRoot(ctx context.Context, idxAt time.Time, roots ...ObjectRoot) error

	// IndexObjectInventory performs the same index operations as IndexObjectRoot and,
	// additionally, indexes the inventory, inv, which should be the root
	// inventory of the OCFL object at the path root. If an inventory with same
	// ID as inv exists in the index, it is replaced by inv.
	IndexObjectInventory(ctx context.Context, idxAt time.Time, invs ...ObjectInventory) error

	//
	RemoveObjectsBefore(ctx context.Context, indexedBefore time.Time) error

	// GetObjectByPath returns the path to a file with digest sum. The path is relative to
	// the storage root.
	GetObjectByPath(ctx context.Context, p string) (*Object, error)
	ListObjectRoots(ctx context.Context, limit int, cursor string) (*ObjectRootList, error)
}

type IndexSummary struct {
	NumInventories int       // number of indexed inventories
	NumObjects     int       // number of known object roots
	UpdatedAt      time.Time // datetime of last index update
}

type ObjectSort uint8

const (
	ASC ObjectSort = iota
	DESC
)
const (
	SortID          = ObjectSort(iota << 1)
	SortV1Created   = ObjectSort(iota << 1)
	SortHeadCreated = ObjectSort(iota << 1)
)

func (s ObjectSort) Desc() bool {
	return s&DESC == DESC
}

type ObjectRoot struct {
	// Object root's directory relative to the storage root.
	Path string
}

type ObjectInventory struct {
	// Object root's directory relative to the storage root.
	Path      string
	Inventory *ocflv1.Inventory
}

type ObjectRootList struct {
	ObjectRoots []ObjectRootListItem
	NextCursor  string
}

type ObjectRootListItem struct {
	// Object root's directory relative to the storage root.
	Path      string
	IndexedAt time.Time
}

type ObjectList struct {
	Objects    []ObjectListItem
	NextCursor string
}

// ObjectListItem is short-form object details for object lists
type ObjectListItem struct {
	RootPath    string    // object path relative to storage root
	ID          string    // OCFL Object ID
	Head        ocfl.VNum // most recent version
	Spec        ocfl.Spec // Object's OCFL Spec version
	V1Created   time.Time // date of first version
	HeadCreated time.Time // date of most recent version
}

// Object is detailed information about an object, as stored in the index.
type Object struct {
	RootPath        string    // object path relative to storage root
	ID              string    // OCFL object ID
	Spec            ocfl.Spec // object's OCFL Spec version
	Head            ocfl.VNum // object's most recent versio
	DigestAlgorithm string    // from inventory
	InventoryDigest string    // from inventory sidecar
	Versions        []*ObjectVersion
}

// ObjectVersion represents indexed OCFL object version metadata
type ObjectVersion struct {
	Num     ocfl.VNum    // Version number
	Message string       // Version message
	Created time.Time    // Version create datetime
	User    *ocflv1.User // Version user information
	Size    int64        // total size of files in version (calculated)
	HasSize bool
}

// PathInfo represents information about a logical path in an objects version state
type PathInfo struct {
	Children   []PathItem
	Sum        string
	IsDir      bool
	Size       int64
	HasSize    bool
	NextCursor string
}

// PathItem is an entry in a PathList
type PathItem struct {
	Name    string
	Sum     string
	IsDir   bool
	Size    int64
	HasSize bool
}
