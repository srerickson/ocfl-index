package index

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/pipeline"
	"github.com/srerickson/ocfl/backend/cloud"
	"github.com/srerickson/ocfl/ocflv1"
	"gocloud.dev/blob"
)

const txCapInv = 10       // number of inventory inserts per transaction
const txCapObjRoot = 1000 // number of object root inserts transaction

// set during with build with
// -ldflags -X 'github.com/srerickson/ocfl-index/internal/index.Version=v0.0.X'
var Version = "devel"

var ErrNotFound = errors.New("not found")
var ErrMissingValue = errors.New("missing value")
var ErrInvalidArgs = errors.New("invalid arguments")
var ErrIndexValue = errors.New("unexpected value in index, possible corruption")

// Index provides indexing for an OCFL Storage Root
type Index struct {
	Backend
}

type ReindexOptions struct {
	FS        ocfl.FS // storage root fs
	RootPath  string  // storage root directory
	ScanConc  int     // concurrency for readdir-based object scanning
	ParseConc int     // concurrency for inventory parsers
	Log       logr.Logger
}

// Reindex is updates the index database
func (idx *Index) Reindex(ctx context.Context, opts *ReindexOptions) error {
	if opts.Log.GetSink() == nil {
		opts.Log = logr.Discard()
	}
	if err := idx.indexObjectRoots(ctx, opts); err != nil {
		return fmt.Errorf("updating the object path index: %w", err)
	}
	if err := idx.IndexInventories(ctx, opts); err != nil {
		return fmt.Errorf("indexing inventories index: %w", err)
	}
	return nil
}

// indexObjectRoots scans the storage root for object root directories, adds them
// to the index (updated indexed_at for any existing entries), and removes any
// object roots in the index that no longer exist in the storage root.
func (idx *Index) indexObjectRoots(ctx context.Context, opts *ReindexOptions) error {
	count := 0
	method := "default"
	var err error
	opts.Log.Info("updating object paths from storage root. This may take a while ...", "root", opts.RootPath)
	defer func() {
		opts.Log.Info("object path update complete", "object_roots", count, "method", method, "root", opts.RootPath)
	}()
	startSync := time.Now()
	switch fsys := opts.FS.(type) {
	case *cloud.FS:
		method = "list-keys"
		count, err = cloudSyncObjecRoots(ctx, idx.Backend, fsys, opts.RootPath)
	default:
		method = "default"
		count, err = defaultSyncObjecRoots(ctx, idx.Backend, opts.FS, opts.RootPath, opts.ScanConc)
	}
	if err != nil {
		return err
	}
	tx, err := idx.Backend.NewTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.RemoveObjectsBefore(ctx, startSync); err != nil {
		return err
	}
	return tx.Commit()
}

func defaultSyncObjecRoots(ctx context.Context, db Backend, fsys ocfl.FS, root string, conc int) (int, error) {
	store, err := ocflv1.GetStore(ctx, fsys, root)
	if err != nil {
		return 0, err
	}
	tx, err := db.NewTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	found := 0
	eachObj := func(obj *ocflv1.Object) error {
		_, root := obj.Root()
		// The indexed object root path should be relatvive to the storage root
		r := ObjectRoot{Path: strings.TrimPrefix(root, root+"/")}
		if err := tx.IndexObjectRoot(ctx, time.Now(), r); err != nil {
			return err
		}
		found++
		if found%txCapObjRoot == 0 {
			// commit and start a new transaction
			if err := tx.Commit(); err != nil {
				return err
			}
			tx, err = db.NewTx(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err := store.ScanObjects(ctx, eachObj, &ocflv1.ScanObjectsOpts{Concurrency: conc}); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return found, nil
}

func cloudSyncObjecRoots(ctx context.Context, db Backend, fsys *cloud.FS, root string) (int, error) {
	iter := fsys.List(&blob.ListOptions{
		Prefix: root,
	})
	tx, err := db.NewTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	found := 0
	var decl ocfl.Declaration
	for {
		item, err := iter.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, err
		}
		if err := ocfl.ParseDeclaration(path.Base(item.Key), &decl); err != nil {
			continue
		}
		if decl.Type != ocfl.DeclObject {
			continue
		}
		// item key's directory is an object root: index the path relative to the
		// storage root.
		objRoot := strings.TrimPrefix(path.Dir(item.Key), root+"/")
		if err := tx.IndexObjectRoot(ctx, time.Now(), ObjectRoot{Path: objRoot}); err != nil {
			return 0, err
		}
		found++
		if found%txCapObjRoot == 0 {
			// commit and start a new transaction
			if err := tx.Commit(); err != nil {
				return 0, err
			}
			tx, err = db.NewTx(ctx)
			if err != nil {
				return found, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return found, err
	}
	return found, nil
}

func (idx *Index) IndexInventories(ctx context.Context, opts *ReindexOptions) error {
	store, err := ocflv1.GetStore(ctx, opts.FS, opts.RootPath)
	if err != nil {
		return err
	}
	// store the storage root's info in the database -- do we need to do this?
	// Why not just keep the values in idx?
	if err := idx.SetStoreInfo(ctx, opts.RootPath, store.Description(), store.Spec()); err != nil {
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
	opts.Log.Info("indexing inventories ...", "path", opts.RootPath, "inventory_workers", opts.ParseConc)
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
	parse := func(objPath string) (*indexJob, error) {
		// objPath is relative to the storage root
		tx := <-txCh
		prev, err := tx.GetObjectByPath(ctx, objPath)
		if err != nil && !errors.Is(err, ErrNotFound) {
			txCh <- tx
			return nil, err
		}
		txCh <- tx
		invPath := path.Join(opts.RootPath, objPath)
		job := newIndexJob(ctx, prev, opts.FS, invPath)
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
	if err := pipeline.Run(scan, parse, index, opts.ParseConc); err != nil {
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
	opts.Log.Info("indexing complete", "path", opts.RootPath, "new_updated", numObjs)
	return nil
}

type indexJob struct {
	sidecar string
	inv     *ocflv1.Inventory
	prev    *Object // existing index entry
	err     error   // error during inventory parse
}

func newIndexJob(ctx context.Context, prev *Object, fsys ocfl.FS, invDir string) *indexJob {
	// TODO: read sidecar for object and compare to avoid reading inventory unnecessarily.
	// if prev != nil {
	// }
	job := &indexJob{prev: prev}
	invPath := path.Join(invDir, "inventory.json")
	inv, vErrs := ocflv1.ValidateInventory(ctx, fsys, invPath, nil)
	if err := vErrs.Err(); err != nil {
		// don't quit if the inventory has errors
		job.err = err
		return job
	}
	job.inv = inv
	job.sidecar = inv.Digest()
	return job
}
