package index

import (
	"context"
	"errors"
	"fmt"
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

type Interface interface {
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
	GetContent(ctx context.Context, objectID string, vnum ocfl.VNum, name string) (*PathResult, error)
	GetContentPath(ctx context.Context, sum string) (string, error)

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

// PathResult represent content at a logical path in the object
type PathResult struct {
	ID       string     `json:"object_id"`
	Version  ocfl.VNum  `json:"version"`            // 'v2'
	Path     string     `json:"path"`               // logical path
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

//
// IndexStore()
//

func IndexStore(ctx context.Context, idx Interface, fsys ocfl.FS, root string, opts ...Option) error {
	conf := indexStoreOptions{
		concurrency: 4,
		log:         logr.Discard(),
	}
	for _, o := range opts {
		o(&conf)
	}
	if conf.concurrency < 1 {
		conf.concurrency = 1
	}
	store, err := ocflv1.GetStore(ctx, fsys, root)
	if err != nil {
		return fmt.Errorf("reading storage root: %w", err)
	}
	conf.log.Info("starting object scan", "root", root, "concurrenct", conf.concurrency)
	objPaths, err := store.ScanObjects(ctx, &ocflv1.ScanObjectsOpts{
		Strict:      false,
		Concurrency: conf.concurrency,
	})
	if err != nil {
		return fmt.Errorf("scanning storage root: %w", err)
	}
	total := len(objPaths)
	conf.log.Info("indexing objects", "root", root, "object_count", total)
	err = indexStore(ctx, idx, store, objPaths, conf.concurrency)
	if err != nil {
		return fmt.Errorf("indexing storage root: %w", err)
	}
	conf.log.Info("indexing complete", "root", root)
	return nil
}

type indexStoreOptions struct {
	concurrency int
	log         logr.Logger
}

type Option func(*indexStoreOptions)

func WithConcurrency(c int) Option {
	return func(opt *indexStoreOptions) {
		opt.concurrency = c
	}
}

func WithLogger(l logr.Logger) Option {
	return func(opt *indexStoreOptions) {
		opt.log = l
	}
}

// concurrent indexing for objects paths in store
func indexStore(ctx context.Context, idx Interface, store *ocflv1.Store, paths map[string]ocfl.Spec, workers int) error {
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
