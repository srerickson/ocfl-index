package index

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
)

// set during with build with
// -ldflags -X 'github.com/srerickson/ocfl-index.Version=v0.0.X'
var Version = "devel"

var ErrNotFound = errors.New("not found")
var ErrMissingValue = errors.New("missing value")

// Service provides indexing for an OCFL Storage Root
type Service struct {
	// set by NewService()
	Backend
	fsys        ocfl.FS
	root        string
	concurrency int
	log         logr.Logger

	// set by Init()
	store *ocflv1.Store
}

// Option is used by NewService to configure the Service
type Option func(*Service)

func WithConcurrency(c int) Option {
	return func(opt *Service) {
		opt.concurrency = c
	}
}

func WithLogger(l logr.Logger) Option {
	return func(opt *Service) {
		opt.log = l
	}
}

// NewService returns a new Service for OCFL storage root at root in fsys. An indexing
// backend implementation (currently, sqlite) is also required.
func NewService(db Backend, fsys ocfl.FS, root string, opts ...Option) *Service {
	srv := &Service{
		Backend:     db,
		fsys:        fsys,
		root:        root,
		concurrency: runtime.GOMAXPROCS(-1),
		log:         logr.Discard(),
	}
	for _, o := range opts {
		o(srv)
	}
	return srv
}

func (srv *Service) Init(ctx context.Context) error {
	store, err := ocflv1.GetStore(ctx, srv.fsys, srv.root)
	if err != nil {
		return err
	}
	srv.store = store
	return nil
}

// DoIndex() indexes the storage root associated with the service.
func (srv Service) DoIndex(ctx context.Context) error {
	srv.log.Info("starting object scan", "root", srv.root, "concurrenct", srv.concurrency)
	objPaths, err := srv.store.ScanObjects(ctx, &ocflv1.ScanObjectsOpts{
		Strict:      false,
		Concurrency: srv.concurrency,
	})
	if err != nil {
		return fmt.Errorf("scanning storage root: %w", err)
	}
	total := len(objPaths)
	srv.log.Info("indexing objects", "root", srv.root, "object_count", total)
	if err := indexStore(ctx, srv.Backend, srv.store, objPaths, srv.concurrency); err != nil {
		return fmt.Errorf("indexing storage root: %w", err)
	}
	srv.log.Info("indexing complete", "root", srv.root)
	return nil
}

func (srv Service) OpenFile(ctx context.Context, name string) (fs.File, error) {
	return srv.fsys.OpenFile(ctx, path.Join(srv.root, name))
}

// concurrent indexing for objects paths in store
func indexStore(ctx context.Context, idx Backend, store *ocflv1.Store, paths map[string]ocfl.Spec, workers int) error {
	type job struct {
		path string
		inv  *ocflv1.Inventory
		err  error
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	in := make(chan (*job))
	go func() {
		defer close(in)
	L:
		for p := range paths {
			select {
			case in <- &job{path: p}:
			case <-ctx.Done():
				break L
			}
		}
	}()
	out := make(chan (*job))
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range in {
				obj, err := store.GetObjectPath(ctx, j.path)
				if err != nil {
					j.err = err
					out <- j
					continue
				}
				j.inv, err = obj.Inventory(ctx)
				if err != nil {
					j.err = err
					out <- j
					continue
				}
				out <- j
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	var returnErr error
	var i int
	for j := range out {
		i++
		if j.err != nil {
			returnErr = j.err
			break
		}
		err := idx.IndexObject(ctx, j.path, j.inv)
		if err != nil {
			returnErr = j.err
			break
		}
	}
	cancel()
	return returnErr
}

// Backend is an interface that can be implemented for different databases for
// storing the indexing.
type Backend interface {
	GetSchemaVersion(ctx context.Context) (int, int, error)
	MigrateSchema(ctx context.Context, erase bool) (bool, error)

	// Set description for storage root in the index
	SetStorageRootDescription(ctx context.Context, desc string) error

	// IndexObject adds an object to the index. It requires the object root path
	// relative to the indexes storage root, pointer to the object's root
	// inventory.
	IndexObject(ctx context.Context, objPath string, rootInv *ocflv1.Inventory) error

	// object/version/path API
	AllObjects(ctx context.Context) (*ListObjectsResult, error)
	GetObject(ctx context.Context, objectID string) (*ObjectResult, error)
	GetContent(ctx context.Context, objectID string, vnum ocfl.VNum, name string) (*ContentResult, error)

	// sum-based getters
	GetContentPath(ctx context.Context, sum string) (string, error)
	GetDirChildren(ctx context.Context, sum string) ([]DirEntry, error)

	// TODO
	//DeleteObject(ctx context.Context, objectID string) error
	//GarbageCollect(ctx context.Context) error
	//HealthChecks(ctx) (Stats, error)
}

// ListObjectsResult is an index response, suitable for json marshaling
type ListObjectsResult struct {
	Description string        `json:"description"`
	Objects     []*ObjectMeta `json:"objects"`
}

// ObjectMeta represents indexed OCFL object metadata
type ObjectMeta struct {
	ID          string    `json:"object_id"`    // OCFL Object ID
	Head        ocfl.VNum `json:"head"`         // most recent version
	HeadCreated time.Time `json:"head_created"` // date of most recent version
}

// ObjectResult is an index response, suitable for json marshaling
type ObjectResult struct {
	// OCFL Object ID
	ID       string         `json:"object_id"`
	Head     string         `json:"head"`
	RootPath string         `json:"root_path"`
	Versions []*VersionMeta `json:"versions"`
}

// VersionMeta represents indexed OCFL object version metadata
type VersionMeta struct {
	ID      string       `json:"object_id"`      // Object ID
	Version ocfl.VNum    `json:"version"`        // Version number
	Message string       `json:"message"`        // Version message
	Created time.Time    `json:"created"`        // Version create datetime
	User    *ocflv1.User `json:"user,omitempty"` // Version user information
}

// ContentResult represent content at a logical path in the object
type ContentResult struct {
	Sum      string     `json:"digest"`             // Hex encoded checksum
	IsDir    bool       `json:"dir"`                // Content is a directory
	Children []DirEntry `json:"children,omitempty"` // Content of a directory
}

// DirEntry represents an entry in a list of directory contents
type DirEntry struct {
	Name  string `json:"name"`
	Sum   string `json:"digest"`
	IsDir bool   `json:"dir"`
}
