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

// Indexer provides indexing for an OCFL Storage Root
type Indexer struct {
	Backend
}

type IndexOptions struct {
	FS          ocfl.FS // storage root fs
	RootPath    string  // storage root directory
	ScanConc    int     // concurrency for readdir-based object scanning
	ParseConc   int     // concurrency for inventory parsers
	Log         logr.Logger
	ObjectIDs   []string // index specific object ids only
	ObjectPaths []string // index specific object root paths only
}

// Index is updates the index database
func (idx *Indexer) Index(ctx context.Context, opts *IndexOptions) error {
	if opts.Log.GetSink() == nil {
		opts.Log = logr.Discard()
	}
	if len(opts.ObjectPaths)+len(opts.ObjectIDs) == 0 {
		// reindex everything
		if err := idx.syncObjectRoots(ctx, opts); err != nil {
			return fmt.Errorf("updating the object path index: %w", err)
		}
	}
	if err := idx.indexInventories(ctx, opts); err != nil {
		return fmt.Errorf("indexing inventories: %w", err)
	}
	return nil
}

// syncObjectRoots scans the storage root for object root directories, adds them
// to the index (updated indexed_at for any existing entries), and removes any
// object roots in the index that no longer exist in the storage root.
func (idx *Indexer) syncObjectRoots(ctx context.Context, opts *IndexOptions) error {
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

func (idx *Indexer) indexInventories(ctx context.Context, opts *IndexOptions) error {
	// TODO: store should be part of ReindexOptions
	store, err := ocflv1.GetStore(ctx, opts.FS, opts.RootPath)
	if err != nil {
		return err
	}
	// store the storage root's info in the database -- do we need to do this?
	if err := idx.SetStoreInfo(ctx, opts.RootPath, store.Description(), store.Spec()); err != nil {
		return err
	}
	indexingAll := len(opts.ObjectIDs)+len(opts.ObjectPaths) == 0
	// new transaction in NewTx
	tx, err := idx.NewTx(ctx)
	if err != nil {
		return err
	}
	// txCh is used to share the database transcation across multiple go
	// routines.
	txCh := make(chan BackendTx, 1)
	txCh <- tx
	defer func() {
		tx := <-txCh
		tx.Rollback()
		close(txCh)
	}()

	opts.Log.Info("indexing inventories ...", "path", opts.RootPath, "inventory_workers", opts.ParseConc)
	numObjs := 0
	// three-phase pipeline for indexing: add object paths; parse
	// inventories; do indexing.
	addPaths := func(addPath func(string) bool) error {
		if indexingAll {
			// reindex everyting
			return addAllObjectsPaths(ctx, addPath, txCh)
		}
		// add just paths for specified objects
		paths := make([]string, 0, len(opts.ObjectIDs)+len(opts.ObjectPaths))
		paths = append(paths, opts.ObjectPaths...)
		for _, id := range opts.ObjectIDs {
			p, err := store.ResolveID(id)
			if err != nil {
				return fmt.Errorf("cannot index object, failed to resolve path: %w", err)
			}
			paths = append(paths, p)
		}
		for _, p := range paths {
			if addPath(p); err != nil {
				break
			}
		}
		return nil
	}
	// parse inventories function (run in multiple go routines)
	parse := func(objPath string) (*indexJob, error) {
		var prev *Object // previously indexed object
		{
			tx := <-txCh
			var err error
			prev, err = tx.GetObjectByPath(ctx, objPath)
			if err != nil && !errors.Is(err, ErrNotFound) {
				txCh <- tx
				return nil, err
			}
			txCh <- tx
		}
		// TODO: read sidecar here and compare to prev's sidecar value (if
		// available). Can skip reading full inventory if sidecars match
		// validate inventory
		invPath := path.Join(opts.RootPath, objPath, "inventory.json")
		inv, vErrs := ocflv1.ValidateInventory(ctx, opts.FS, invPath, nil)
		if err := vErrs.Err(); err != nil {
			// don't quit if the inventory has errors
			return &indexJob{err: err}, nil
		}
		job := &indexJob{prev: prev, inv: inv}
		if job.inv != nil {
			job.sidecar = job.inv.Digest()
		}
		return job, nil
	}
	// index update function (single go routine)
	index := func(root string, job *indexJob, err error) error {
		if err != nil {
			return fmt.Errorf("in object '%s': %w", root, err)
		}
		if job.err != nil {
			// different behavior here depending on whether we are indexing
			// everything or select IDs. For select ids, we quit without
			// indexing additionl objects. For indexing all, we log and
			// continue.
			if !indexingAll {
				return job.err
			}
			opts.Log.Error(job.err, "object has errors", "object_path", root)
		}
		if job.inv == nil {
			// nothing to do
			return nil
		}
		if job.prev != nil && job.sidecar != "" && job.prev.InventoryDigest == job.sidecar {
			opts.Log.V(10).Info("object is unchanged", "object_path", root)
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
	if err := pipeline.Run(addPaths, parse, index, opts.ParseConc); err != nil {
		return fmt.Errorf("indexing halted prematurely: %w", err)
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

func addAllObjectsPaths(ctx context.Context, add func(string) bool, txCh chan BackendTx) error {
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

// TODO: ocfl api should expose api for this
//func getSide