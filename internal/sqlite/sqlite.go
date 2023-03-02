package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl-index/internal/sqlite/sqlc"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

const (
	tablePrefix = "ocfl_index_"
)

var (
	// expected schema for index file
	schemaVer = sqlc.OcflIndexSchema{Major: 0, Minor: 3}

	//go:embed schema.sql
	querySchema string

	//go:embed get_path_object.sql
	queryGetPathObject string

	////go:embed get_recursive_node.sql
	//queryGetPathRecursive string

	queryListTables string = `SELECT name FROM sqlite_master WHERE type='table';`
)

// Backend is a sqlite-based implementation of index.Backend
type Backend struct {
	sql.DB
}

var _ index.Backend = (*Backend)(nil)

// Open returns a new Backend using connection string conf, which is passed
// directory to sql.Open. Open does not confirm the database schema
// or
func Open(conf string) (*Backend, error) {
	db, err := sql.Open("sqlite", conf)
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA case_sensitive_like=ON;")
	return &Backend{DB: *db}, nil
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

// InitSchema checks the schema of the sqlite database and initializes it.
func (db *Backend) InitSchema(ctx context.Context) (bool, error) {
	tables, err := db.existingTables(ctx)
	if err != nil {
		return false, err
	}
	if len(tables) > 0 {
		qry := sqlc.New(db)
		schema, err := qry.GetSchemaVersion(ctx)
		if err != nil {
			return false, err
		}
		if schema == schemaVer {
			return false, nil
		}
		return false, fmt.Errorf("database uses schema v%d.%d, this version of ocfl-index requires v%d.%d ",
			schema.Major, schema.Minor, schemaVer.Major, schemaVer.Minor,
		)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("starting new transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, querySchema); err != nil {
		return false, fmt.Errorf("create table: %w", err)
	}
	return true, tx.Commit()
}

func (db *Backend) GetStoreSummary(ctx context.Context) (index.StoreSummary, error) {
	qry := sqlc.New(&db.DB)
	row, err := qry.GetStorageRoot(ctx)
	if err != nil {
		return index.StoreSummary{}, err
	}
	count, err := qry.CountInventories(ctx)
	if err != nil {
		return index.StoreSummary{}, err
	}
	summ := index.StoreSummary{
		Description: row.Description,
		RootPath:    row.RootPath,
		NumObjects:  int(count),
	}
	if row.IndexedAt.Valid {
		summ.IndexedAt = row.IndexedAt.Time
	}
	if row.Spec != "" {
		if err := ocfl.ParseSpec(row.Spec, &summ.Spec); err != nil {
			return index.StoreSummary{}, err
		}
	}
	return summ, nil
}

// Consider removing this
func (db *Backend) SetStoreInfo(ctx context.Context, root string, desc string, spec ocfl.Spec) error {
	return sqlc.New(&db.DB).SetStorageRoot(ctx, sqlc.SetStorageRootParams{
		Description: desc,
		RootPath:    root,
		Spec:        spec.String(),
	})
}

func (db *Backend) SetStoreIndexedAt(ctx context.Context) error {
	return sqlc.New(&db.DB).SetStorageRootIndexed(ctx)
}

func (db *Backend) IndexObjectRoot(ctx context.Context, root string, idxAt time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting new transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := db.indexObjectRootTx(ctx, sqlc.New(&db.DB).WithTx(tx), root, idxAt); err != nil {
		return fmt.Errorf("indexing object root: %w", err)

	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing changes: %w", err)
	}
	return nil
}

func (db *Backend) IndexObjectInventory(ctx context.Context, root string, idxAt time.Time, inv *ocflv1.Inventory) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting new transaction: %w", err)
	}
	defer tx.Rollback()
	rootrow, err := db.indexObjectRootTx(ctx, sqlc.New(&db.DB).WithTx(tx), root, idxAt)
	if err != nil {
		return fmt.Errorf("indexing object root: %w", err)
	}
	if err := db.indexInventoryTx(ctx, sqlc.New(&db.DB).WithTx(tx), rootrow, idxAt, inv, nil); err != nil {
		return fmt.Errorf("indexing inventory: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing changes: %w", err)
	}
	return nil
}

func (db *Backend) IndexObjectInventorySize(ctx context.Context, root string, idxAt time.Time, inv *ocflv1.Inventory, sizes map[string]int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting new transaction: %w", err)
	}
	defer tx.Rollback()
	qryTx := sqlc.New(&db.DB).WithTx(tx)
	rootrow, err := db.indexObjectRootTx(ctx, qryTx, root, idxAt)
	if err != nil {
		return fmt.Errorf("indexing object root: %w", err)
	}
	if sizes == nil {
		sizes = make(map[string]int64)
	}
	// add existing file sizes to sizes
	rows, err := qryTx.ListObjectContentSize(ctx, inv.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("geting existing file sizes from index: %w", err)
	}
	for _, r := range rows {
		if _, ok := sizes[r.FilePath]; !ok {
			sizes[r.FilePath] = r.Size.Int64
		}
	}
	if err := db.indexInventoryTx(ctx, sqlc.New(&db.DB).WithTx(tx), rootrow, idxAt, inv, sizes); err != nil {
		return fmt.Errorf("indexing inventory: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing changes: %w", err)
	}
	return nil
}

func (db *Backend) indexObjectRootTx(ctx context.Context, tx *sqlc.Queries, root string, idxAt time.Time) (int64, error) {
	if root == "" {
		return 0, fmt.Errorf("object root is required: %w", index.ErrInvalidArgs)
	}
	if idxAt.IsZero() {
		idxAt = time.Now()
	}
	idxAt = idxAt.Truncate(time.Second).UTC()
	idxobj, err := tx.UpsertObjectRoot(ctx, sqlc.UpsertObjectRootParams{
		Path:      root,
		IndexedAt: idxAt,
	})
	if err != nil {
		return 0, fmt.Errorf("upsert object root: %w", err)
	}
	return idxobj.ID, nil
}

// index the inventory. To index without filesize checks, sizes must be nil.
func (db *Backend) indexInventoryTx(ctx context.Context, tx *sqlc.Queries, rootRow int64, idxAt time.Time, inv *ocflv1.Inventory, sizes map[string]int64) error {
	idxInv, err := tx.UpsertInventory(ctx, sqlc.UpsertInventoryParams{
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
		nodeRow, err := addPathtreeNodes(ctx, tx, tree)
		if err != nil {
			return fmt.Errorf("indexing version state: %w", err)
		}
		if err := insertVersion(ctx, tx, vnum, ver, invrow, nodeRow); err != nil {
			return fmt.Errorf("indexing inventory versions: %w", err)
		}
	}
	// content paths
	if err := insertContent(ctx, tx, inv.Manifest, invrow); err != nil {
		return fmt.Errorf("indexing content files: %w", err)
	}
	return nil
}

// We can't use sqlc here because we need to alter the query for different sort/cursor values.
func (idx *Backend) ListObjects(ctx context.Context, sort index.ObjectSort, limit int, cursor string) (*index.ObjectList, error) {
	// TODO implement additional sorts
	// TODO check limit value
	// TODO parse cursor
	cursorID, _, err := parseCursor(cursor)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	template := `SELECT 
			objects.id,
			objects.ocfl_id,
			objects.spec,
			objects.head,
			v1.created v1_created,
			head.created head_created
		FROM ocfl_index_inventories objects
		INNER JOIN ocfl_index_versions head
			ON objects.id = head.inventory_id AND objects.head = head.name
		INNER JOIN ocfl_index_versions v1
			ON objects.id = v1.inventory_id AND v1.num = 1
		%s LIMIT ?;`
	switch sort {
	case index.SortV1Created:
		// Something like this:
		// SELECT
		//     head.created || objects.id AS cursor, -- cursor is unique (date+id)
		//     objects.ocfl_id,
		//     v1.created v1_created,
		//     head.created head_created
		// FROM ocfl_index_inventories objects
		// INNER JOIN ocfl_index_versions head
		//     ON objects.id = head.object_id AND objects.head = head.name
		// INNER JOIN ocfl_index_versions v1
		//     ON objects.id = v1.object_id AND v1.num = 1
		// WHERE cursor < '2022-11-10 06:50:29.08237092 +0000 UTC66603' ORDER BY cursor DESC LIMIT 500;
		return nil, errors.New("v1 created sort not implemented")
	case index.SortHeadCreated:
		return nil, errors.New("head created sort not implemented")
	default:
		// Sort by ID
		if cursor == "" {
			where := "ORDER BY objects.ocfl_id"
			if sort.Desc() {
				where += " DESC"
			}
			rows, err = idx.QueryContext(ctx, fmt.Sprintf(template, where), limit)
			break
		}
		var where string
		if sort.Desc() {
			where = "WHERE objects.ocfl_id < ? ORDER BY objects.ocfl_id DESC"
		} else {
			where = "WHERE objects.ocfl_id > ? ORDER BY objects.ocfl_id"
		}
		rows, err = idx.QueryContext(ctx, fmt.Sprintf(template, where), cursorID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("during list objects query: %w", err)
	}
	defer rows.Close()
	objects := make([]index.ObjectListItem, 0, limit)
	for rows.Next() {
		var id int64
		var spec, head string
		var obj index.ObjectListItem
		if err := rows.Scan(
			&id,
			&obj.ID,
			&spec,
			&head,
			&obj.V1Created,
			&obj.HeadCreated,
		); err != nil {
			return nil, err
		}
		if err := ocfl.ParseVNum(head, &obj.Head); err != nil {
			return nil, err
		}
		if err := ocfl.ParseSpec(spec, &obj.Spec); err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	list := &index.ObjectList{Objects: objects}
	if l := len(objects); l > 0 {
		obj := objects[l-1]
		var t time.Time
		if sort&index.SortHeadCreated > 0 {
			t = obj.HeadCreated
		} else if sort&index.SortV1Created > 0 {
			t = obj.V1Created
		}
		list.NextCursor = newCursor(obj.ID, t)
	}
	return list, nil
}

func (db *Backend) GetObject(ctx context.Context, objID string) (*index.Object, error) {
	qry := sqlc.New(db)
	obj, err := qry.GetInventoryID(ctx, objID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("object id '%s': %w", objID, index.ErrNotFound)
		}
	}
	ret, err := db.asIndexInventory(ctx, (*sqlc.GetInventoryPathRow)(&obj))
	if err != nil {
		return nil, fmt.Errorf("while getting object id '%s': %w", objID, err)
	}
	return ret, nil
}

func (db *Backend) GetObjectByPath(ctx context.Context, p string) (*index.Object, error) {
	qry := sqlc.New(db)
	obj, err := qry.GetInventoryPath(ctx, p)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("object path '%s': %w", p, index.ErrNotFound)
		}
		return nil, err
	}
	ret, err := db.asIndexInventory(ctx, &obj)
	if err != nil {
		return nil, fmt.Errorf("while getting object path '%s': %w", p, err)
	}
	return ret, nil
}

func (db *Backend) asIndexInventory(ctx context.Context, sqlInv *sqlc.GetInventoryPathRow) (*index.Object, error) {
	qry := sqlc.New(db)
	rows, err := qry.ListVersions(ctx, sqlInv.OcflID)
	if err != nil {
		return nil, err
	}
	vers := make([]*index.ObjectVersion, len(rows))
	for i := 0; i < len(rows); i++ {
		ver := &index.ObjectVersion{
			Message: rows[i].Message,
			Created: rows[i].Created,
			Size:    rows[i].Size.Int64,
			HasSize: rows[i].Size.Valid,
		}
		err := ocfl.ParseVNum(rows[i].Name, &ver.Num)
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
	result := &index.Object{
		ID:              sqlInv.OcflID,
		DigestAlgorithm: sqlInv.DigestAlgorithm,
		InventoryDigest: sqlInv.InventoryDigest,
		RootPath:        sqlInv.Path,
		Versions:        vers,
	}
	if err := ocfl.ParseVNum(sqlInv.Head, &result.Head); err != nil {
		return nil, err
	}
	if err := ocfl.ParseSpec(sqlInv.Spec, &result.Spec); err != nil {
		return nil, err
	}
	return result, nil
}

func (db *Backend) GetObjectState(ctx context.Context, id string, v ocfl.VNum, p string, _ bool, lim int, cur string) (*index.PathInfo, error) {
	if lim == 0 {
		lim = 1000
	}
	if lim > 1000 {
		lim = 1000
	}
	p = path.Clean(p)
	if !fs.ValidPath(p) {
		return nil, fmt.Errorf("invalid path: %s", p)
	}
	var vStr string
	if !v.IsZero() {
		vStr = v.String()
	}
	var baseNode = struct {
		id     int64
		sumbyt []byte
		size   sql.NullInt64
		isdir  bool
	}{}
	qry := sqlc.New(&db.DB)
	errFn := func(err error) error {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%s: %s: %w", id, p, index.ErrNotFound)
		}
		return fmt.Errorf("%s: %s: %w", id, p, err)
	}
	row := db.QueryRowContext(ctx, queryGetPathObject, id, vStr, p)
	if err := row.Scan(&baseNode.id, &baseNode.sumbyt, &baseNode.isdir, &baseNode.size); err != nil {
		return nil, errFn(err)
	}
	result := &index.PathInfo{
		Sum:     hex.EncodeToString(baseNode.sumbyt),
		IsDir:   baseNode.isdir,
		Size:    baseNode.size.Int64,
		HasSize: baseNode.size.Valid,
	}
	if !baseNode.isdir {
		return result, nil
	}
	// base is a directory: get list of children
	rows, err := qry.NodeDirChildren(ctx, sqlc.NodeDirChildrenParams{
		ParentID: baseNode.id,
		Name:     cur,
		Limit:    int64(lim + 1), // limit+1 to check for next page
	})
	if err != nil {
		return nil, errFn(err)
	}
	result.Children = make([]index.PathItem, len(rows))
	for i, r := range rows {
		item := index.PathItem{
			IsDir:   r.Dir,
			Name:    r.Name,
			Sum:     hex.EncodeToString(r.Sum),
			Size:    r.Size.Int64,
			HasSize: r.Size.Valid,
		}
		result.Children[i] = item
	}
	// check if there are additional results
	if l := len(result.Children); l == lim+1 {
		result.Children = result.Children[:lim]
		result.NextCursor = result.Children[lim-1].Name
	}
	return result, nil
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
	return path.Join(result.Path, result.FilePath), nil
}

// existingTables returns list of table names in the database with the "ocfl_index_" prefix
func (db *Backend) existingTables(ctx context.Context) ([]string, error) {
	rows, err := db.QueryContext(ctx, queryListTables)
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

// upsertInventory adds or updates a row in the inventories table as part object
// indexing. It returns the inventory's internal rowid, a booleen indicating if
// an insert or update occured, and an error value. If the inventory exists and
// the InventoryDigest is unchanged, no changes are made and the returned
// boolean is false.
// func upsertInventory(ctx context.Context, qry *sqlc.Queries, obj *index.IndexingObject, objRow int64) (int64, bool, error) {
// 	inv, err := qry.GetInventoryID(ctx, obj.Inventory.ID)
// 	if err != nil {
// 		if !errors.Is(err, sql.ErrNoRows) {
// 			return 0, false, fmt.Errorf("get inventory: %w", err)
// 		}
// 		id, err := qry.InsertInventory(ctx, sqlc.InsertInventoryParams{
// 			OcflID:          obj.Inventory.ID,
// 			Head:            obj.Inventory.Head.String(),
// 			Spec:            obj.Inventory.Type.Spec.String(),
// 			DigestAlgorithm: obj.Inventory.DigestAlgorithm,
// 			InventoryDigest: obj.Inventory.Digest(),
// 			IndexedAt:       obj.IndexedAt,
// 			RootID:          objRow,
// 		})
// 		if err != nil {
// 			return 0, false, fmt.Errorf("inventory insert: %w", err)
// 		}
// 		return id, true, nil
// 	}
// 	if inv.InventoryDigest != obj.Inventory.Digest() {
// 		err = qry.UpdateInventory(ctx, sqlc.UpdateInventoryParams{
// 			ID:              inv.ID, // internal id
// 			Head:            obj.Inventory.Head.String(),
// 			Spec:            obj.Inventory.Type.Spec.String(),
// 			DigestAlgorithm: obj.Inventory.DigestAlgorithm,
// 			InventoryDigest: obj.Inventory.Digest(),
// 			IndexedAt:       obj.IndexedAt,
// 		})
// 		if err != nil {
// 			return 0, false, fmt.Errorf("inventory update: %w", err)
// 		}
// 		return inv.ID, true, nil
// 	}
// 	return inv.ID, false, nil
// }

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

func parseCursor(cursor string) (string, time.Time, error) {
	if cursor == "" {
		return "", time.Time{}, nil
	}
	byts, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cursor format error: %w", err)
	}
	vals, err := csv.NewReader(bytes.NewReader(byts)).Read()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cursor format error: %w", err)
	}
	if len(vals) != 2 {
		return "", time.Time{}, errors.New("cursor format error: expected two values")
	}
	id := vals[0]
	t, err := time.Parse(time.RFC3339, vals[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cursor format error: %w", err)
	}
	return id, t, nil
}

func newCursor(id string, t time.Time) string {
	byt := &bytes.Buffer{}
	w := csv.NewWriter(byt)
	if err := w.Write([]string{id, t.Format(time.RFC3339)}); err != nil {
		panic(err)
	}
	w.Flush()
	return base64.StdEncoding.EncodeToString(byt.Bytes())
}

func (db *Backend) DEBUG_AllInventories(ctx context.Context) ([]sqlc.OcflIndexInventory, error) {
	qry := sqlc.New(db)
	return qry.DebugAllInventories(ctx)
}

func (db *Backend) DEBUG_AllVersions(ctx context.Context) ([]sqlc.OcflIndexVersion, error) {
	qry := sqlc.New(db)
	return qry.DebugAllVersions(ctx)
}

func (db *Backend) DEBUG_AllNames(ctx context.Context) ([]sqlc.OcflIndexName, error) {
	qry := sqlc.New(db)
	return qry.DebugAllNames(ctx)
}

func (db *Backend) DEBUG_AllNodes(ctx context.Context) ([]sqlc.OcflIndexNode, error) {
	qry := sqlc.New(db)
	return qry.DebugAllNodes(ctx)
}

func (db *Backend) DEBUG_AllObjecRootss(ctx context.Context) ([]sqlc.OcflIndexObjectRoot, error) {
	qry := sqlc.New(db)
	return qry.DebugAllObjectRoots(ctx)
}
