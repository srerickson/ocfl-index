package sqlite

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl-index/internal/sqlite/sqlc"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

type Tx struct {
	db *Backend
	tx *sql.Tx
}

var _ index.BackendTx = (*Tx)(nil)

// NewTX creates a new transaction for the backend
func (db *Backend) NewTx(ctx context.Context) (index.BackendTx, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{db: db, tx: tx}, nil
}

// Rollback aborts the transaction
func (tx *Tx) Rollback() error {
	return tx.tx.Rollback()
}

// Commit commits the transaction
func (tx *Tx) Commit() error {
	return tx.tx.Commit()
}

func (tx *Tx) GetObjectByPath(ctx context.Context, p string) (*index.Object, error) {
	qryTx := sqlc.New(tx.db).WithTx(tx.tx)
	return getObjectByPathTx(ctx, qryTx, p)
}

func (tx *Tx) ListObjectRoots(ctx context.Context, limit int, cursor string) (*index.ObjectRootList, error) {
	qryTx := sqlc.New(tx.db).WithTx(tx.tx)
	return listObjectRootsTx(ctx, qryTx, limit, cursor)
}

func (tx *Tx) IndexObjectRoot(ctx context.Context, indexedAt time.Time, roots ...index.ObjectRoot) error {
	qryTx := sqlc.New(tx.db).WithTx(tx.tx)
	for _, r := range roots {
		if _, err := indexObjectRootTx(ctx, qryTx, r.Path, indexedAt); err != nil {
			return fmt.Errorf("indexing object root: %w", err)
		}
	}
	return nil
}

func (tx *Tx) IndexObjectInventory(ctx context.Context, idxAt time.Time, inv ...index.ObjectInventory) error {
	qry := sqlc.New(tx.db).WithTx(tx.tx)
	for i := range inv {
		rootrow, err := indexObjectRootTx(ctx, qry, inv[i].Path, idxAt)
		if err != nil {
			return fmt.Errorf("indexing object root: %w", err)
		}
		if err := indexInventoryTx(ctx, qry, rootrow, idxAt, inv[i].Inventory, nil); err != nil {
			return fmt.Errorf("indexing inventory: %w", err)
		}
	}
	return nil
}

// Remove all objects with indexed_at values older than before.
func (tx *Tx) RemoveObjectsBefore(ctx context.Context, indexedBefore time.Time) error {
	qry := sqlc.New(tx.db).WithTx(tx.tx)
	// indexed_at is always UTC
	return qry.DeleteObjectRootsBefore(ctx, indexedBefore.UTC())
}

func indexObjectRootTx(ctx context.Context, qry *sqlc.Queries, root string, idxAt time.Time) (int64, error) {
	if root == "" {
		return 0, fmt.Errorf("object root is required: %w", index.ErrInvalidArgs)
	}
	if idxAt.IsZero() {
		idxAt = time.Now()
	}
	idxobj, err := qry.UpsertObjectRoot(ctx, sqlc.UpsertObjectRootParams{
		Path:      root,
		IndexedAt: idxAt.UTC(), // always use UTC
	})
	if err != nil {
		return 0, fmt.Errorf("upsert object root: %w", err)
	}
	return idxobj.ID, nil
}

// index the inventory. To index without filesize checks, sizes must be nil.
func indexInventoryTx(ctx context.Context, qry *sqlc.Queries, rootRow int64, idxAt time.Time, inv *ocflv1.Inventory, sizes map[string]int64) error {
	idxInv, err := qry.UpsertInventory(ctx, sqlc.UpsertInventoryParams{
		OcflID:          inv.ID,
		RootID:          rootRow,
		Head:            inv.Head.String(),
		Spec:            inv.Type.Spec.String(),
		DigestAlgorithm: inv.DigestAlgorithm,
		InventoryDigest: inv.Digest(),
	})
	if err != nil {
		return err
	}
	invrow := idxInv.ID
	// versions table
	for vnum, ver := range inv.Versions {
		// sizes may be nil
		tree, err := index.PathTree(inv, vnum, sizes)
		if err != nil {
			return fmt.Errorf("building tree from version state: %w", err)
		}
		if sizes != nil && !tree.Val.HasSize {
			// the tree's root node should have size for entire version state
			return fmt.Errorf("incomplete content file size information for '%s'", vnum)
		}
		nodeRow, err := addPathtreeNodes(ctx, qry, tree)
		if err != nil {
			return fmt.Errorf("indexing version state: %w", err)
		}
		if err := insertVersion(ctx, qry, vnum, ver, invrow, nodeRow); err != nil {
			return fmt.Errorf("indexing inventory versions: %w", err)
		}
	}
	// content paths
	if err := insertContent(ctx, qry, inv.Manifest, invrow); err != nil {
		return fmt.Errorf("indexing content files: %w", err)
	}
	return nil
}

// insertVersion adds a new row to the versions table as part of the object
// indexing. invRow is the row id for the parent inventory, nodeID is the root
// node for the version state. Currently, rows in the version table are never
// updated. If a row exists for the version, an error is returned if indexed
// values don't match those in the given version.
func insertVersion(ctx context.Context, qry *sqlc.Queries, vnum ocfl.VNum, ver *ocflv1.Version, invRow int64, nodeID int64) error {
	idxV, err := qry.GetVersion(ctx, sqlc.GetVersionParams{
		InventoryID: invRow,
		Num:         int64(vnum.Num()),
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		params := sqlc.InsertVersionParams{
			InventoryID: invRow,
			Name:        vnum.String(),
			Message:     ver.Message,
			Created:     ver.Created.UTC(), // location prevents errors reading from db
			NodeID:      nodeID,
		}
		if ver.User != nil {
			params.UserAddress = ver.User.Address
			params.UserName = ver.User.Name
		}
		if _, err := qry.InsertVersion(ctx, params); err != nil {
			return err
		}
		return nil
	}
	if idxV.NodeID != nodeID {
		return fmt.Errorf("state for version '%s' has changed: %w", vnum, index.ErrIndexValue)
	}
	if idxV.Message != ver.Message {
		return fmt.Errorf("message for version '%s' has changed: %w", vnum, index.ErrIndexValue)
	}
	if idxV.Created.Unix() != ver.Created.Unix() {
		return fmt.Errorf("created timestamp for version '%s' has changed: %w", vnum, index.ErrIndexValue)
	}
	if ver.User != nil {
		if idxV.UserAddress != ver.User.Address {
			return fmt.Errorf("user address for version '%s' has changed: %w", vnum, index.ErrIndexValue)
		}
		if idxV.UserName != ver.User.Name {
			return fmt.Errorf("user name for version '%s' has changed: %w", vnum, index.ErrIndexValue)
		}
	}
	return nil
}

func insertContent(ctx context.Context, tx *sqlc.Queries, man *digest.Map, invID int64) error {
	return man.EachPath(func(name, digest string) error {
		sum, err := hex.DecodeString(digest)
		if err != nil {
			return err
		}
		params := sqlc.InsertIgnoreContentPathParams{
			Sum:         sum,
			FilePath:    name,
			InventoryID: invID,
		}
		if err := tx.InsertIgnoreContentPath(ctx, params); err != nil {
			return fmt.Errorf("adding content path: '%s'", name)
		}
		return nil
	})
}

// addPathtreeNodes adds all values in the pathtree to the index, both names and nodes. It returns the
// rows id for the node representing the tree's root
func addPathtreeNodes(ctx context.Context, tx *sqlc.Queries, tree *pathtree.Node[index.IndexingVal]) (int64, error) {
	return addPathtreeChild(ctx, tx, tree, 0, "")
}

// recursive implementation for addPathtree
func addPathtreeChild(ctx context.Context, tx *sqlc.Queries, tree *pathtree.Node[index.IndexingVal], parentID int64, name string) (int64, error) {
	nodeID, isNew, err := getSetNode(ctx, tx, tree.Val, tree.IsDir())
	if err != nil {
		return 0, err
	}
	if parentID != 0 {
		// for non-root nodes, make sure name exists connecting parentID and nodeID
		err = tx.InsertIgnoreName(ctx, sqlc.InsertIgnoreNameParams{
			NodeID:   nodeID,
			ParentID: parentID,
			Name:     name,
		})
		if err != nil {
			return 0, err
		}
	}
	if !isNew {
		// if getInserNode didn't create a new node, the children have already
		// been created.
		return nodeID, nil
	}
	for _, e := range tree.DirEntries() {
		n := e.Name()
		child := tree.Child(n)
		if _, err = addPathtreeChild(ctx, tx, child, nodeID, n); err != nil {
			return 0, err
		}
	}
	return nodeID, nil
}

// getSetNode gets, inserts, and possibly updates the node for sum/dir,
// returning the rowid and a boolean indicating if the node was created or
// updated. An update only occurs in the case that val includes size information
// but the indexed node does not.
func getSetNode(ctx context.Context, qry *sqlc.Queries, val index.IndexingVal, isdir bool) (int64, bool, error) {
	node, err := qry.GetNodeSum(ctx, sqlc.GetNodeSumParams{
		Sum: val.Sum,
		Dir: isdir,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
		id, err := qry.InsertNode(ctx, sqlc.InsertNodeParams{
			Size: sql.NullInt64{Int64: val.Size, Valid: val.HasSize},
			Sum:  val.Sum,
			Dir:  isdir,
		})
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	}
	if val.HasSize && !node.Size.Valid {
		// update node size when possible
		qry.SetNodeSize(ctx, sqlc.SetNodeSizeParams{
			Size: sql.NullInt64{Int64: val.Size, Valid: val.HasSize},
			Sum:  val.Sum,
			Dir:  isdir,
		})
	}
	return node.ID, false, nil
}
