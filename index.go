package index

import (
	"context"
	"time"

	"github.com/srerickson/ocfl/object"
	"github.com/srerickson/ocfl/ocflv1"
)

type Interface interface {
	Close() error
	GetSchemaVersion(ctx context.Context) (int, int, error)
	MigrateSchema(ctx context.Context, erase bool) (bool, error)
	IndexInventory(ctx context.Context, inv *ocflv1.Inventory) error
	AllObjects(ctx context.Context) ([]*ObjectMeta, error)
	GetVersions(ctx context.Context, objectID string) ([]*VersionMeta, error)
	GetContent(ctx context.Context, objectID string, vnum object.VNum, name string) (*ContentMeta, error)

	// TODO
	//DeleteObject(ctx context.Context, objectID string) error
	//GarbageCollect(ctx context.Context) error
	//HealthChecks(ctx) (Stats, error)
}

type VersionMeta struct {
	Num     object.VNum
	Message string
	Created time.Time
	User    *ocflv1.User
}

type ObjectMeta struct {
	ID          string
	Head        object.VNum
	HeadCreated time.Time
}

type ContentMeta struct {
	IsDir       bool
	ContentPath string
	Children    []DirEntry
}

type DirEntry struct {
	Name  string
	IsDir bool
}
