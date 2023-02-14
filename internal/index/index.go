package index

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
)

// set during with build with
// -ldflags -X 'github.com/srerickson/ocfl-index/internal/index.Version=v0.0.X'
var Version = "devel"

var ErrNotFound = errors.New("not found")
var ErrMissingValue = errors.New("missing value")
var ErrInvalidArgs = errors.New("invalid arguments")
var ErrIndexValue = errors.New("unexpected value in index, possible corruption")

// Index provides indexing for an OCFL Storage Root
type Index struct {
	Backend            // index database
	ocfl.FS            // storage root fs
	root        string // storage root directory
	concurrency int
	log         logr.Logger
	store       *ocflv1.Store
}

// NewIndex returns a new Index for OCFL storage root at root in fsys. An indexing
// backend implementation (currently, sqlite) is also required.
func NewIndex(db Backend, fsys ocfl.FS, root string, opts ...Option) *Index {
	idx := &Index{
		Backend:     db,
		FS:          fsys,
		root:        root,
		concurrency: runtime.GOMAXPROCS(-1),
		log:         logr.Discard(),
	}
	for _, o := range opts {
		o(idx)
	}
	return idx
}

// Option is used by NewIndex to configure the Index
type Option func(*Index)

func WithConcurrency(c int) Option {
	return func(opt *Index) {
		opt.concurrency = c
	}
}

func WithLogger(l logr.Logger) Option {
	return func(opt *Index) {
		opt.log = l
	}
}

// IndexMode values are used to represent how extensive an Indexing operation
// should be
type IndexMode uint8

const (
	// Index object root directories
	ModeObjectDirs IndexMode = iota
	// Index object root directories and inventories
	ModeInventories
	// Index object root directories, inventories, and file sizes
	ModeFileSizes
)

func (l IndexMode) String() string {
	switch l {
	case ModeObjectDirs:
		return "objectRoots"
	case ModeInventories:
		return "objectRoots,inventories"
	case ModeFileSizes:
		return "objectRoots,inventories,fileSizes"
	}
	return "invalid"
}

func (idx *Index) DoIndex(ctx context.Context, mode IndexMode, paths ...string) error {
	store, err := ocflv1.GetStore(ctx, idx.FS, idx.root)
	if err != nil {
		return err
	}
	idx.store = store
	// store the storage root's info in the database -- do we need to do this? Why not just keep the values in idx?
	if err := idx.SetStoreInfo(ctx, idx.root, store.Description(), store.Spec()); err != nil {
		return err
	}
	idx.log.Info("indexing storage root...", "path", idx.root, "concurrency", idx.concurrency, "indexingMode", mode)
	numObjs := 0
	scanFn := func(obj *ocflv1.Object) error {
		numObjs++
		return idx.indexObject(ctx, obj, mode)
	}
	if err := store.ScanObjects(ctx, scanFn, &ocflv1.ScanObjectsOpts{
		Strict:      false,
		Concurrency: idx.concurrency,
	}); err != nil {
		return fmt.Errorf("indexing storage root: %w", err)
	}
	idx.SetStoreIndexedAt(ctx)
	idx.log.Info("indexing complete", "path", idx.root)
	return nil
}

func (idx Index) indexObject(ctx context.Context, obj *ocflv1.Object, mode IndexMode) error {
	fsys, root := obj.Root()
	idx.log.Info("indexing object", "path", root, mode, mode)
	prev, err := idx.GetObjectByPath(ctx, root)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("getting previously indexed object for path='%s'", root)
	}
	if mode == ModeObjectDirs {
		return idx.IndexObjectRoot(ctx, root, time.Now())
	}
	// inventory indexing: check that sidecar has changed
	sidecar, err := obj.InventorySidecar(ctx)
	if err != nil {
		return err
	}
	if prev != nil && strings.EqualFold(sidecar, prev.InventoryDigest) && mode == ModeInventories {
		// The inventory sidecar digest matches the previously indexed
		// value. We don't need to index the inventory, so downgrade
		// this object root indexing
		idx.log.Info("skipping inventory indexing because sidecar digest is unchanged", "path", root)
		return idx.IndexObjectRoot(ctx, root, time.Now())
	}
	inv, err := obj.Inventory(ctx)
	if err != nil {
		return err
	}
	if mode == ModeInventories {
		return idx.IndexObjectInventory(ctx, root, time.Now(), inv)
	}
	// file size indexing final case.
	sizes, err := getSizes(ctx, fsys, root, prev, inv)
	if err != nil {
		return fmt.Errorf("while scanning object content size: %w", err)
	}
	return idx.IndexObjectInventorySize(ctx, root, time.Now(), inv, sizes)
}

// build FileSizes list. If available, use size information from prev to figure
// out new version content directories that need to be scanned. If we only scan
// the content files for the later versions, and the version state refers to
// files from a previous version, we will have partial size information for that
// version... need to figure out how to merge the existing size information into
// the new pathtree.
func getSizes(ctx context.Context, fsys ocfl.FS, root string, prev *Object, inv *ocflv1.Inventory) (map[string]int64, error) {
	lastSizeV := 0
	if prev != nil {
		for _, v := range prev.Versions {
			if v.HasSize && v.Num.Num() > lastSizeV {
				lastSizeV = v.Num.Num() - 1
			}
		}
	}
	// versions to scan
	toscan := inv.Head.VNumSeq()[lastSizeV:]
	// map source files -> size
	sizes := map[string]int64{}
	for _, vnum := range toscan {
		// This approach to scanning an object's content for file size information
		// feels too low-level. It requires too much knowledge about the internal
		// structure of an OCFL object. It would be nice for the ocflv1 package
		// to provide an api that abstracts some of this.
		prefix := path.Join(root, vnum.String(), inv.ContentDirectory)
		fn := func(name string, dirent fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name = strings.TrimPrefix(name, root+"/")
			info, err := dirent.Info()
			if err != nil {
				return err
			}
			sizes[name] = info.Size()
			return nil
		}
		if err := ocfl.EachFile(ctx, fsys, prefix, fn); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// OK if content directory doesn't exist.. skip this version
				continue
			}
			return nil, err
		}
	}
	return sizes, nil
}
