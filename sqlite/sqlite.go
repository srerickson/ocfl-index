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

	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl-index/sqlite/sqlc"
	"github.com/srerickson/ocfl/ocflv1"
)

const (
	tablePrefix = "ocfl_index_"
)

var (
	// expected schema for index file
	schemaVer = sqlc.OcflIndexSchema{Major: 0, Minor: 2}

	//go:embed schema.sql
	querySchema string

	//go:embed get_path_object.sql
	queryGetPathObject string

	queryListTables string = `SELECT name FROM sqlite_master WHERE type='table';`
)

// Backend is a sqlite-based implementation of index.Backend
type Backend struct {
	sql.DB
}

func New(conf string) (*Backend, error) {
	db, err := sql.Open("sqlite", conf)
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA case_sensitive_like=ON;")
	return &Backend{DB: *db}, nil
}

func (db *Backend) GetStorageRootDescription(ctx context.Context) (string, error) {
	return sqlc.New(&db.DB).GetStorageRootDescription(ctx)
}

func (db *Backend) SetStorageRootDescription(ctx context.Context, desc string) error {
	return sqlc.New(&db.DB).SetStorageRootDescription(ctx, desc)
}

func (db *Backend) IndexObject(ctx context.Context, root string, inv *ocflv1.Inventory) error {
	if err := inv.Validate().Err(); err != nil {
		return fmt.Errorf("inventory is invalid and cannot be indexed: %w", err)
	}
	errFn := func(err error) error {
		return fmt.Errorf("indexing inventory for %s: %w", inv.ID, err)
	}
	tree, err := index.InventoryTree(inv)
	if err != nil {
		return errFn(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errFn(err)
	}
	defer tx.Rollback()
	queries := sqlc.New(&db.DB).WithTx(tx)
	rootNodeID, err := addIndexNodes(ctx, queries, &tree.Node, 0, "")
	if err != nil {
		return errFn(err)
	}
	objID, _, err := upsertObjectNode(ctx, queries, inv.ID, root, rootNodeID, inv.Head)
	if err != nil {
		return errFn(err)
	}
	if err := insertContent(ctx, queries, &tree.Node, objID); err != nil {
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
			Created:  version.Created.UTC(), // location prevents errors reading from db
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
	return tx.Commit()
}

func (idx *Backend) AllObjects(ctx context.Context) (*index.ListObjectsResult, error) {
	qry := sqlc.New(idx)
	rows, err := qry.ListObjects(ctx)
	if err != nil {
		return nil, err
	}
	objects := make([]*index.ObjectMeta, len(rows))
	for i := range rows {
		obj := &index.ObjectMeta{}
		obj.HeadCreated = rows[i].Created
		err := ocfl.ParseVNum(rows[i].Head, &obj.Head)
		if err != nil {
			return nil, err // head info in index is invalid
		}
		obj.ID = rows[i].OcflID
		objects[i] = obj
	}
	result := &index.ListObjectsResult{
		Objects: objects,
	}
	return result, nil
}

func (idx *Backend) GetObject(ctx context.Context, objID string) (*index.ObjectResult, error) {
	qry := sqlc.New(idx)
	obj, err := qry.GetObjectID(ctx, objID)
	if err != nil {
		return nil, err
	}
	rows, err := qry.ListObjectVersions(ctx, objID)
	if err != nil {
		return nil, err
	}
	vers := make([]*index.VersionMeta, len(rows))
	for i := 0; i < len(rows); i++ {
		ver := &index.VersionMeta{
			ID:      objID,
			Message: rows[i].Message,
			Created: rows[i].Created,
		}
		err := ocfl.ParseVNum(rows[i].Name, &ver.Version)
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
	result := &index.ObjectResult{
		ID:       objID,
		Head:     obj.Head,
		RootPath: obj.RootPath,
		Versions: vers,
	}
	return result, nil
}

func (db *Backend) GetContent(ctx context.Context, objID string, vnum ocfl.VNum, p string) (*index.ContentResult, error) {
	p = path.Clean(p)
	if !fs.ValidPath(p) {
		return nil, fmt.Errorf("invalid path: %s", p)
	}
	fullP := path.Join(vnum.String(), p)
	var node = struct {
		id     int64
		sumbyt []byte
		isdir  bool
	}{}
	qry := sqlc.New(&db.DB)
	errFn := func(err error) error {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%s: %s: %w", objID, fullP, index.ErrNotFound)
		}
		return fmt.Errorf("%s: %s: %w", objID, fullP, err)
	}

	row := db.QueryRowContext(ctx, queryGetPathObject, objID, fullP)
	err := row.Scan(&node.id, &node.sumbyt, &node.isdir)
	if err != nil {
		return nil, errFn(err)
	}
	result := index.ContentResult{
		Sum:   hex.EncodeToString(node.sumbyt),
		IsDir: node.isdir,
	}
	if result.IsDir {
		rows, err := qry.NodeDirChildren(ctx, node.id)
		if err != nil {
			// require directory node to have children?
			return nil, errFn(err)
		}
		result.Children = make([]index.DirEntry, len(rows))
		for i, r := range rows {
			result.Children[i] = index.DirEntry{
				IsDir: r.Dir,
				Name:  r.Name,
				Sum:   hex.EncodeToString(r.Sum),
			}
		}
		return &result, nil
	}
	return &result, nil
}

func (db *Backend) GetContentPath(ctx context.Context, sum string) (string, error) {
	qry := sqlc.New(&db.DB)
	bytes, err := hex.DecodeString(sum)
	if err != nil {
		return "", err
	}
	result, err := qry.GetContentPath(ctx, bytes)
	if err != nil {
		return "", err
	}
	return path.Join(result.RootPath, result.FilePath), nil
}

func (db *Backend) GetDirChildren(ctx context.Context, sum string) ([]index.DirEntry, error) {
	qry := sqlc.New(&db.DB)
	bytes, err := hex.DecodeString(sum)
	if err != nil {
		return nil, err
	}
	rows, err := qry.NodeDirChildrenSum(ctx, bytes)
	if err != nil {
		return nil, err
	}
	results := make([]index.DirEntry, len(rows))
	for i := range rows {
		results[i] = index.DirEntry{
			Name:  rows[i].Name,
			Sum:   hex.EncodeToString(rows[i].Sum),
			IsDir: rows[i].Dir,
		}
	}
	return results, nil
}

func (db *Backend) GetSchemaVersion(ctx context.Context) (int, int, error) {
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
func (db *Backend) MigrateSchema(ctx context.Context, erase bool) (bool, error) {
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
func (idx *Backend) existingTables(ctx context.Context) ([]string, error) {
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

// addIndexNodes adds the node and all its descendants to the index. Unless
// parentID is 0, a name entry is also created linking the top-level node to the
// parent.
func addIndexNodes(ctx context.Context, tx *sqlc.Queries, node *pathtree.Node[index.IndexingVal], parentID int64, name string) (int64, error) {
	nodeID, isNew, err := getInsertNode(ctx, tx, node.Val.Sum, node.IsDir())
	if err != nil {
		return 0, err
	}
	if parentID != 0 {
		// even if getInserNode didn't create a new node, we still need to add
		// a new named 'edge' connecting parentID and nodeID.
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
		// if getInserNode didn't create a new node, the children have already
		// been created.
		return nodeID, nil
	}
	for _, e := range node.DirEntries() {
		n := e.Name()
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

func upsertObjectNode(ctx context.Context, qry *sqlc.Queries, objID string, objRoot string, nodeID int64, head ocfl.VNum) (int64, bool, error) {
	obj, err := qry.GetObjectID(ctx, objID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
		id, err := qry.InsertObject(ctx, sqlc.InsertObjectParams{
			OcflID:   objID,
			RootPath: objRoot,
			NodeID:   nodeID,
			Head:     head.String(),
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

func insertContent(ctx context.Context, tx *sqlc.Queries, node *pathtree.Node[index.IndexingVal], objID int64) error {
	return pathtree.Walk(*node, func(name string, isdir bool, val index.IndexingVal) error {
		params := sqlc.InsertContentPathIgnoreParams{
			Sum:      val.Sum,
			FilePath: val.Path,
			ObjectID: objID,
		}
		if err := tx.InsertContentPathIgnore(ctx, params); err != nil {
			return fmt.Errorf("adding content path: '%s'", val.Path)
		}
		return nil
	})
}

var _ index.Backend = (*Backend)(nil)
