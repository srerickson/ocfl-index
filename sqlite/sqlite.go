package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/internal/digest"
	"github.com/srerickson/ocfl-index/sqlite/sqlc"
	"github.com/srerickson/ocfl/object"
	"github.com/srerickson/ocfl/ocflv1"
)

const (
	tablePrefix = "ocfl_index_"
)

var (
	// expected schema for index file
	schemaVer = sqlc.OcflIndexSchema{Major: 0, Minor: 1}

	//go:embed schema.sql
	querySchema string

	//go:embed find_path_node.sql
	queryGetPathNode string

	queryListTables string = `SELECT name FROM sqlite_master WHERE type='table';`
)

type Index struct {
	sql.DB
}

func New(db *sql.DB) *Index {
	return &Index{DB: *db}
}

func (db *Index) IndexInventory(ctx context.Context, inv *ocflv1.Inventory) error {
	if err := inv.Validate().Err(); err != nil {
		return fmt.Errorf("inventory is invalid and cannot be indexed: %w", err)
	}
	errFn := func(err error) error {
		return fmt.Errorf("indexing inventory for %s: %w", inv.ID, err)
	}
	root, err := digest.InventoryTree(inv)
	if err != nil {
		return errFn(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errFn(err)
	}
	defer tx.Rollback()
	queries := sqlc.New(&db.DB).WithTx(tx)
	nodeID, err := addIndexNodes(ctx, queries, root, 0, "")
	if err != nil {
		return errFn(err)
	}
	objID, _, err := upsertObjectNode(ctx, queries, inv.ID, nodeID, inv.Head)
	if err != nil {
		return errFn(err)
	}
	// replace existing version entries
	err = queries.DeleteObjectVersions(ctx, objID)
	if err != nil {
		return errFn(err)
	}
	for vnum, version := range inv.Versions {
		params := sqlc.InsertObjectVersionParams{
			ObjectID: objID,
			Name:     vnum.String(),
			Num:      int64(vnum.Num()),
			Message:  version.Message,
			Created:  version.Created,
		}
		if version.User != nil {
			params.UserAddress = version.User.Address
			params.UserName = version.User.Name
		}
		_, err := queries.InsertObjectVersion(ctx, params)
		if err != nil {
			return err
		}
	}
	// add all content paths
	for sum := range inv.Manifest.AllDigests() {
		byts, err := hex.DecodeString(sum)
		if err != nil {
			return err
		}
		paths := inv.Manifest.DigestPaths(sum)
		params := sqlc.InsertContentPathIgnoreParams{
			Sum:      byts,
			FilePath: paths[0],
			ObjectID: objID,
		}
		err = queries.InsertContentPathIgnore(ctx, params)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (idx *Index) AllObjects(ctx context.Context) ([]*index.ObjectMeta, error) {
	qry := sqlc.New(idx)
	rows, err := qry.ListObjects(ctx)
	if err != nil {
		return nil, err
	}
	objects := make([]*index.ObjectMeta, len(rows))
	for i := range rows {
		obj := &index.ObjectMeta{}
		obj.HeadCreated = rows[i].Created
		err := object.ParseVNum(rows[i].Head, &obj.Head)
		if err != nil {
			return nil, err // head info in index is invalid
		}
		obj.ID = rows[i].Uri
		objects[i] = obj
	}
	return objects, nil
}

func (idx *Index) GetVersions(ctx context.Context, objID string) ([]*index.VersionMeta, error) {
	qry := sqlc.New(idx)
	rows, err := qry.ListObjectVersions(ctx, objID)
	if err != nil {
		return nil, err
	}
	vers := make([]*index.VersionMeta, len(rows))
	for i := 0; i < len(rows); i++ {
		ver := &index.VersionMeta{
			Message: rows[i].Message,
			Created: rows[i].Created,
		}
		err := object.ParseVNum(rows[i].Name, &ver.Num)
		if err != nil {
			return nil, err
		}
		if rows[i].UserName != "" {
			ver.User = &ocflv1.User{
				Name:    rows[i].UserName,
				Address: rows[i].UserAddress,
			}
		}
		vers[i] = ver
	}
	return vers, nil
}

func (db *Index) GetContent(ctx context.Context, objID string, vnum object.VNum, p string) (*index.ContentMeta, error) {
	p = path.Clean(p)
	if !fs.ValidPath(p) {
		return nil, fmt.Errorf("invalid path: %s", p)
	}
	var (
		cont  index.ContentMeta
		qry   = sqlc.New(&db.DB)
		intID int64 // internal sql id
		sum   []byte
	)
	if p == "." {
		// different query for object's direct children
		row, err := qry.ObjectVersionNode(ctx, sqlc.ObjectVersionNodeParams{
			Uri:  objID,
			Name: vnum.String(),
		})
		if err != nil {
			return nil, err
		}
		intID = row.ID
		sum = row.Sum
		cont.IsDir = row.Dir
	} else {
		fullP := path.Join(vnum.String(), p)
		args := []any{objID, fullP}
		row := db.QueryRowContext(ctx, queryGetPathNode, args...)
		if err := row.Scan(&intID, &sum, &cont.IsDir); err != nil {
			return nil, fmt.Errorf("content for %s: %s: %w", objID, fullP, err)
		}
	}
	if cont.IsDir {
		rows, err := qry.NodeChildren(ctx, intID)
		if err != nil {
			return nil, err
		}
		cont.Children = make([]index.DirEntry, len(rows))
		for i, r := range rows {
			cont.Children[i] = index.DirEntry{
				IsDir: r.Dir,
				Name:  r.Name,
			}
		}
		return &cont, nil
	}
	p, err := qry.GetContentPath(ctx, sqlc.GetContentPathParams{
		Uri: objID,
		Sum: sum,
	})
	if err != nil {
		return nil, fmt.Errorf("content for %d: %s: %w", intID, sum, err)
	}
	cont.ContentPath = p

	return &cont, nil
}

func (db *Index) GetSchemaVersion(ctx context.Context) (int, int, error) {
	eMsg := "can't determine OCFL-Index schema version"
	qry := sqlc.New(db)
	ver, err := qry.GetSchemaVersion(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", eMsg, err)
	}
	return int(ver.Major), int(ver.Minor), nil
}

// CreateTable creates all tables index tables. If erase is true any existing
// "ocfl_index_" tables are erase.
func (db *Index) MigrateSchema(ctx context.Context, erase bool) (bool, error) {
	tables, err := db.existingTables(ctx)
	if err != nil {
		return false, err
	}
	if len(tables) > 0 {
		// check schema version
		qry := sqlc.New(db)
		schema, err := qry.GetSchemaVersion(ctx)
		if err != nil {
			return false, err
		}
		if schema == schemaVer {
			return false, nil
		}
		if !erase {
			return false, fmt.Errorf("database uses schema v%d.%d, this version of ocfl-index requires v%d.%d ",
				schema.Major, schema.Minor, schemaVer.Major, schemaVer.Minor,
			)
		}
		for _, t := range tables {
			_, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s;`, t))
			if err != nil {
				return false, err
			}
		}
	}
	_, err = db.ExecContext(ctx, querySchema)
	if err != nil {
		return false, fmt.Errorf("create table: %w", err)
	}
	return true, nil
}

// existingTables returns list of table names in the database with the "ocfl_index_" prefix
func (idx *Index) existingTables(ctx context.Context) ([]string, error) {
	rows, err := idx.QueryContext(ctx, queryListTables)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if strings.HasPrefix(t, tablePrefix) {
			tables = append(tables, t)
		}
	}
	return tables, nil
}

// addIndexNodes adds the node and all its descendants to the index. Unless parentID is 0, a name entry
// is also created linking the top-level node to the parent.
func addIndexNodes(ctx context.Context, tx *sqlc.Queries, node *digest.Tree, parentID int64, name string) (int64, error) {
	nodeID, isNew, err := getInsertNode(ctx, tx, node.Val(), node.IsDir())
	if err != nil {
		return 0, err
	}
	if parentID != 0 {
		err = tx.InsertNameIgnore(ctx, sqlc.InsertNameIgnoreParams{
			NodeID:   nodeID,
			ParentID: parentID,
			Name:     name,
		})
		if err != nil {
			return 0, err
		}
	}
	if !isNew {
		return nodeID, nil
	}
	for _, n := range node.Children() {
		child, err := node.Get(n)
		if err != nil {
			panic(err)
		}
		_, err = addIndexNodes(ctx, tx, child, nodeID, n)
		if err != nil {
			return 0, err
		}
	}
	return nodeID, nil
}

// getInsertNode gets or inserts the node for sum/dir. If the node is created, the
// returned boolean is true.
func getInsertNode(ctx context.Context, qry *sqlc.Queries, sum []byte, dir bool) (int64, bool, error) {
	id, err := qry.GetNodeSum(ctx, sqlc.GetNodeSumParams{
		Sum: sum,
		Dir: dir,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
		id, err = qry.InsertNode(ctx, sqlc.InsertNodeParams{
			Sum: sum,
			Dir: dir,
		})
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	}
	return id, false, nil
}

func upsertObjectNode(ctx context.Context, qry *sqlc.Queries, uri string, nodeID int64, head object.VNum) (int64, bool, error) {
	obj, err := qry.GetObjectURI(ctx, uri)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
		id, err := qry.InsertObject(ctx, sqlc.InsertObjectParams{
			Uri:    uri,
			NodeID: nodeID,
			Head:   head.String(),
		})
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	}
	if obj.NodeID != nodeID {
		err = qry.UpdateObject(ctx, sqlc.UpdateObjectParams{
			NodeID: nodeID,
			ID:     obj.ID,
			Head:   head.String(),
		})
		if err != nil {
			return 0, false, err
		}
	}
	return obj.ID, false, nil
}

var _ index.Interface = (*Index)(nil)
