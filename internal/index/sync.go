package index

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/backend/cloud"
	"github.com/srerickson/ocfl/ocflv1"
	"gocloud.dev/blob"
)

const txCapObjRoot = 1000 // number of object root indexing queries per transaction

// SyncObjectRoots scans the storage root for object root directories, adds them
// to the index (updated indexed_at for any existing entries), and removes any
// object roots in the index that no longer exist in the storage root.
func (idx *Index) SyncObjectRoots(ctx context.Context) error {
	count := 0
	method := "default"
	var err error
	idx.log.Info("updating object paths from storage root. This may take a while ...", "root", idx.root)
	defer func() {
		idx.log.Info("object path update complete", "object_roots", count, "method", method, "root", idx.root)
	}()
	startSync := time.Now()
	switch fsys := idx.FS.(type) {
	case *cloud.FS:
		method = "list-keys"
		count, err = idx.cloudSyncObjecRoots(ctx, fsys)
	default:
		method = "default"
		count, err = idx.defaultSyncObjecRoots(ctx)
	}
	if err != nil {
		return err
	}
	tx, err := idx.NewTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.RemoveObjectsBefore(ctx, startSync); err != nil {
		return err
	}
	return tx.Commit()
}

func (idx *Index) defaultSyncObjecRoots(ctx context.Context) (int, error) {
	store, err := ocflv1.GetStore(ctx, idx.FS, idx.root)
	if err != nil {
		return 0, err
	}
	tx, err := idx.NewTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	found := 0
	eachObj := func(obj *ocflv1.Object) error {
		_, root := obj.Root()
		// The indexed object root path should be relatvive to the storage root
		r := ObjectRoot{Path: strings.TrimPrefix(root, idx.root+"/")}
		if err := tx.IndexObjectRoot(ctx, time.Now(), r); err != nil {
			return err
		}
		found++
		if found%txCapObjRoot == 0 {
			// commit and start a new transaction
			if err := tx.Commit(); err != nil {
				return err
			}
			tx, err = idx.NewTx(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err := store.ScanObjects(ctx, eachObj, &ocflv1.ScanObjectsOpts{Concurrency: idx.scanConc}); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return found, nil
}

func (idx *Index) cloudSyncObjecRoots(ctx context.Context, fsys *cloud.FS) (int, error) {
	iter := fsys.List(&blob.ListOptions{
		Prefix: idx.root,
	})
	tx, err := idx.NewTx(ctx)
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
		objRoot := strings.TrimPrefix(path.Dir(item.Key), idx.root+"/")
		if err := tx.IndexObjectRoot(ctx, time.Now(), ObjectRoot{Path: objRoot}); err != nil {
			return 0, err
		}
		found++
		if found%txCapObjRoot == 0 {
			// commit and start a new transaction
			if err := tx.Commit(); err != nil {
				return 0, err
			}
			tx, err = idx.NewTx(ctx)
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
