package index

import (
	"context"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
)

// Backend is an interface that can be implemented for different databases for
// storing the indexing.
type Backend interface {
	// Get/Set Storage Root details in the index
	SetStoreInfo(ctx context.Context, root string, desc string, spec ocfl.Spec) error
	GetStoreSummary(ctx context.Context) (StoreSummary, error)

	// Set StorageRoot' IndexedAt timestamp to 'now'
	SetStoreIndexedAt(ctx context.Context) error

	// IndexObject adds an object to the index. It requires the object root path
	// relative to the indexes storage root, pointer to the object's root
	// inventory.
	IndexObject(ctx context.Context, obj *IndexingObject) error

	// All OCFL Objects in the index
	ListObjects(ctx context.Context, order ObjectSort, limit int, cursor string) (*ObjectList, error)
	GetObject(ctx context.Context, objectID string) (*Object, error)
	GetObjectByPath(ctx context.Context, rootPath string) (*Object, error)

	// GetObjectState returns a path list representing files and directories in an
	// object version state (i.e., the "logical state").
	GetObjectState(ctx context.Context, objectID string, vnum ocfl.VNum, base string, recursive bool, limit int, cursor string) (*PathInfo, error)

	// GetContentPath returns the path to a file with digest sum. The path is relative to
	// the storage root's ocfl.FS.
	GetContentPath(ctx context.Context, sum string) (string, error)
}

type StoreSummary struct {
	RootPath    string
	Description string
	Spec        ocfl.Spec
	NumObjects  int
	IndexedAt   time.Time
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

type ObjectList struct {
	Objects    []ObjectListItem
	NextCursor string
}

// ObjectListItem is short-form object details for object lists
type ObjectListItem struct {
	ID          string    // OCFL Object ID
	Head        ocfl.VNum // most recent version
	Spec        ocfl.Spec // Object's OCFL Spec version
	V1Created   time.Time // date of first version
	HeadCreated time.Time // date of most recent version
}

// Object is detailed information about an object, as stored in the index.
type Object struct {
	ID              string    // OCFL object ID
	Spec            ocfl.Spec // object's OCFL Spec version
	Head            ocfl.VNum // object's most recent versio
	DigestAlgorithm string    // from inventory
	InventoryDigest string    // from inventory sidecar
	RootPath        string    // object path relative to storage root
	Versions        []*ObjectVersion
}

// ObjectVersion represents indexed OCFL object version metadata
type ObjectVersion struct {
	Num     ocfl.VNum    // Version number
	Message string       // Version message
	Created time.Time    // Version create datetime
	User    *ocflv1.User // Version user information
	Size    int64        // total size of files in version (calculated)
}

// PathInfo represents information about a logical path in an objects version state
type PathInfo struct {
	Children   []PathItem
	Sum        string
	IsDir      bool
	Size       int64
	NextCursor string
}

// PathItem is an entry in a PathList
type PathItem struct {
	Name  string
	Sum   string
	IsDir bool
	Size  int64
}
