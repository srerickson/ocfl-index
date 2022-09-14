package index

import (
	"context"
	"errors"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
)

// set during with build with
// -ldflags -X 'github.com/srerickson/ocfl-index.Version=v0.0.X'
var Version = "devel"

var ErrNotFound = errors.New("not found")
var ErrMissingValue = errors.New("missing value")

type Interface interface {
	Close() error
	GetSchemaVersion(ctx context.Context) (int, int, error)
	MigrateSchema(ctx context.Context, erase bool) (bool, error)
	IndexInventory(ctx context.Context, inv *ocflv1.Inventory) error
	AllObjects(ctx context.Context) (*ObjectsResult, error)
	GetVersions(ctx context.Context, objectID string) (*VersionsResult, error)
	GetContent(ctx context.Context, objectID string, vnum ocfl.VNum, name string) (*ContentResult, error)

	// TODO
	//DeleteObject(ctx context.Context, objectID string) error
	//GarbageCollect(ctx context.Context) error
	//HealthChecks(ctx) (Stats, error)
}

// ObjectsResult is an index response, suitable for json marshaling
type ObjectsResult struct {
	Objects []*ObjectMeta `json:"objects"`
}

// VersionsResult is an index response, suitable for json marshaling
type VersionsResult struct {
	// OCFL Object ID
	ID       string         `json:"id"`
	Versions []*VersionMeta `json:"versions"`
}

// ContentResult is an index response, suitable for json marshaling
type ContentResult struct {
	// OCFL Object ID
	ID string `json:"id"`
	// Version number for content
	Version ocfl.VNum `json:"version"`
	// Logical path of content
	Path    string       `json:"path"`
	Content *ContentMeta `json:"content"`
}

// ObjectMeta represents indexed OCFL object metadata
type ObjectMeta struct {
	ID          string    `json:"id"`           // OCFL Object ID
	Head        ocfl.VNum `json:"head"`         // most recent version
	HeadCreated time.Time `json:"head_created"` // date of most recent version
}

// VersionMeta represents indexed OCFL object version metadata
type VersionMeta struct {
	Num     ocfl.VNum    `json:"id"`             // Version number
	Message string       `json:"message"`        // Version message
	Created time.Time    `json:"created"`        // Version create datetime
	User    *ocflv1.User `json:"user,omitempty"` // Version user information
}

// ContentMeta represents the indexed logical content of an OCFL object
type ContentMeta struct {
	IsDir       bool       `json:"dir"`                    // Content is a directory
	ContentPath string     `json:"content_path,omitempty"` // Content path for file
	Sum         string     `json:"digest"`                 // Hex encoded checksum
	Children    []DirEntry `json:"children,omitempty"`     // Content of a directory
}

// DirEntry represents an entry in a list of directory contents
type DirEntry struct {
	Name  string `json:"name"` // file or directory name
	IsDir bool   `json:"dir"`  // entry is a directory
}
