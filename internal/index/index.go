package index

import (
	"context"
	"errors"
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/pipeline"
	"github.com/srerickson/ocfl/ocflv1"
)

const txCapInv = 10

// set during with build with
// -ldflags -X 'github.com/srerickson/ocfl-index/internal/index.Version=v0.0.X'
var Version = "devel"

var ErrNotFound = errors.New("not found")
var ErrMissingValue = errors.New("missing value")
var ErrInvalidArgs = errors.New("invalid arguments")
var ErrIndexValue = errors.New("unexpected value in index, possible corruption")

// Index provides indexing for an OCFL Storage Root
type Index struct {
	Backend          // index database
	ocfl.FS          // storage root fs
	root      string // storage root directory
	scanConc  int
	parseConc int
	log       logr.Logger
	store     *ocflv1.Store
}

// NewIndex returns a new Index for OCFL storage root at root in fsys. An indexing
// backend implementation (currently, sqlite) is also required.
func NewIndex(db Backend, fsys ocfl.FS, root string, opts ...Option) *Index {
	numcpu := runtime.NumCPU()
	idx := &Index{
		Backend:   db,
		FS:        fsys,
		root:      root,
		scanConc:  numcpu,
		parseConc: numcpu,
		log:       logr.Discard(),
	}
	for _, o := range opts {
		o(idx)
	}
	return idx
}

// Option is used by NewIndex to configure the Index
type Option func(*Index)

func WithObjectScanConc(c int) Option {
	return func(opt *Index) {
		opt.scanConc = c
	}
}

func WithInventoryParseConc(c int) Option {
	return func(opt *Index) {
		opt.parseConc = c
	}
}

func WithLogger(l logr.Logger) Option {
	return func(opt *Index) {
		opt.log = l
	}
}

func (idx *Index) IndexInventories(ctx context.Context, paths ...string) error {
	store, err := ocflv1.GetStore(ctx, idx.FS, idx.root)
	if err != nil {
		return err
	}
	idx.store = store
	// store the storage root's info in the database -- do we need to do this?
	// Why not just keep the values in idx?
	if err := idx.SetStoreInfo(ctx, idx.root, store.Description(), store.Spec()); err != nil {
		return err
	}
	// txCh is used to share the database transcation across multiple go
	// routines.
	txCh := make(chan BackendTx, 1)
	defer func() {
		tx := <-txCh
		tx.Rollback()
		close(txCh)
	}()
	{
		// new transaction in NewTx
		tx, err := idx.NewTx(ctx)
		if err != nil {
			return err
		}
		txCh <- tx
	}
	idx.log.Info("indexing inventories ...", "path", idx.root, "inventory_workers", idx.parseConc)
	numObjs := 0
	// three-phase pipeline for indexing: scan for object roots; parse
	// inventories; do indexing.
	scan := func(add func(string) error) error {
		cursor := ""
		for {
			tx := <-txCh
			roots, err := tx.ListObjectRoots(ctx, 0, cursor)
			if err != nil {
				txCh <- tx
				return err
			}
			txCh <- tx
			for _, r := range roots.ObjectRoots {
				if add(r.Path); err != nil {
					return nil
				}
			}
			if roots.NextCursor == "" {
				break
			}
			cursor = roots.NextCursor
		}
		return nil
	}
	// parse inventories function (run in multiple go routines)
	parse := func(root string) (*indexJob, error) {
		job, err := idx.newIndexJob(ctx, root, txCh)
		if err != nil {
			return nil, fmt.Errorf("preparing to index '%s': %w", root, err)
		}
		return job, nil
	}
	// index update function (single go routine)
	index := func(root string, job *indexJob, err error) error {
		if err != nil {
			return err
		}
		if job.inv == nil {
			return nil // inventory had errors, skip it
		}
		if job.prev != nil && job.sidecar != "" && job.prev.InventoryDigest == job.sidecar {
			// unchanged skip it
			return nil
		}
		numObjs++
		objInvs := ObjectInventory{Path: root, Inventory: job.inv}
		// index inventories
		tx := <-txCh
		defer func() {
			txCh <- tx
		}()
		if err := tx.IndexObjectInventory(ctx, time.Now(), objInvs); err != nil {
			return err
		}
		if numObjs%txCapInv == 0 {
			var err error
			if err = tx.Commit(); err != nil {
				return err
			}
			// set tx as a new transaction
			if tx, err = idx.NewTx(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	if err := pipeline.Run(scan, parse, index, idx.parseConc); err != nil {
		return fmt.Errorf("during indexing: %w", err)
	}
	{
		// commit any pending changes
		tx := <-txCh
		if err := tx.Commit(); err != nil {
			txCh <- tx
			return err
		}
		txCh <- tx
	}
	idx.log.Info("indexing complete", "path", idx.root, "new_updated", numObjs)
	return nil
}

type indexJob struct {
	sidecar string
	inv     *ocflv1.Inventory
	prev    *Object // previous indexed value
}

func (idx *Index) newIndexJob(ctx context.Context, root string, txCh chan BackendTx) (*indexJob, error) {
	var job indexJob
	var err error
	invPath := path.Join(idx.root, root, "inventory.json")
	inv, vErrs := ocflv1.ValidateInventory(ctx, idx, invPath, nil)
	if err := vErrs.Err(); err != nil {
		// don't quit if the inventory has errors
		idx.log.Error(err, "inventory has errors", "path", invPath)
		return &job, nil
	}
	job.inv = inv
	job.sidecar = inv.Digest()
	tx := <-txCh
	defer func() {
		txCh <- tx
	}()
	job.prev, err = tx.GetObjectByPath(ctx, root)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("getting previously indexed object for path='%s': %w", root, err)
	}
	return &job, nil
}

// build FileSizes list. If available, use size information from prev to figure
// out new version content directories that need to be scanned. If we only scan
// the content files for the later versions, and the version state refers to
// files from a previous version, we will have partial size information for that
// version... need to figure out how to merge the existing size information into
// the new pathtree.
// func getSizes(ctx context.Context, fsys ocfl.FS, root string, prev *Object, inv *ocflv1.Inventory) (map[string]int64, error) {
// 	lastSizeV := 0
// 	if prev != nil {
// 		for _, v := range prev.Versions {
// 			if v.HasSize && v.Num.Num() > lastSizeV {
// 				lastSizeV = v.Num.Num() - 1
// 			}
// 		}
// 	}
// 	// versions to scan
// 	toscan := inv.Head.AsHead()[lastSizeV:]
// 	// map source files -> size
// 	sizes := map[string]int64{}
// 	for _, vnum := range toscan {
// 		// This approach to scanning an object's content for file size information
// 		// feels too low-level. It requires too much knowledge about the internal
// 		// structure of an OCFL object. It would be nice for the ocflv1 package
// 		// to provide an api that abstracts some of this.
// 		prefix := path.Join(root, vnum.String(), inv.ContentDirectory)
// 		fn := func(name string, dirent fs.DirEntry, err error) error {
// 			if err != nil {
// 				return err
// 			}
// 			name = strings.TrimPrefix(name, root+"/")
// 			info, err := dirent.Info()
// 			if err != nil {
// 				return err
// 			}
// 			sizes[name] = info.Size()
// 			return nil
// 		}
// 		if err := ocfl.EachFile(ctx, fsys, prefix, fn); err != nil {
// 			if errors.Is(err, fs.ErrNotExist) {
// 				// OK if content directory doesn't exist.. skip this version
// 				continue
// 			}
// 			return nil, err
// 		}
// 	}
// 	return sizes, nil
// }
