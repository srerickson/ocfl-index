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
	_, err = db.ExecContext(ctx, querySchema)
	if err != nil {
		return false, fmt.Errorf("create table: %w", err)
	}
	return true, nil
}

func (db *Backend) GetStoreSummary(ctx context.Context) (index.StoreSummary, error) {
	qry := sqlc.New(&db.DB)
	row, err := qry.GetStorageRoot(ctx)
	if err != nil {
		return index.StoreSummary{}, err
	}
	count, err := qry.CountObjects(ctx)
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

// IndexObject
func (db *Backend) IndexObject(ctx context.Context, vals *index.IndexingObject) error {
	errFn := func(err error) error {
		return fmt.Errorf("indexing inventory for %s: %w", vals.Obj.ID, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errFn(err)
	}
	defer tx.Rollback()
	queries := sqlc.New(&db.DB).WithTx(tx)
	objRowID, changes, err := upsertObject(ctx, queries, vals.Obj)
	if err != nil {
		return errFn(err)
	}
	if !changes {
		// the inventory digest for the object is unchanged
		return nil
	}
	// add versions
	for _, version := range vals.Obj.Versions {
		vRoot := vals.State[version.Num]
		vRootID, err := addIndexNodes(ctx, queries, vRoot, 0, "")
		if err != nil {
			return errFn(err)
		}
		if err := insertContent(ctx, queries, vRoot, objRowID); err != nil {
			return errFn(err)
		}
		if err := insertVersion(ctx, queries, version, objRowID, vRootID); err != nil {
			return errFn(err)
		}
	}
	return tx.Commit()
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
		FROM ocfl_index_objects objects
		INNER JOIN ocfl_index_object_versions head
			ON objects.id = head.object_id AND objects.head = head.name
		INNER JOIN ocfl_index_object_versions v1
			ON objects.id = v1.object_id AND v1.num = 1
		%s LIMIT ?;`
	switch sort {
	case index.SortV1Created:
		// Something like this:
		// SELECT
		//     head.created || objects.id AS cursor, -- cursor is unique (date+id)
		//     objects.ocfl_id,
		//     v1.created v1_created,
		//     head.created head_created
		// FROM ocfl_index_objects objects
		// INNER JOIN ocfl_index_object_versions head
		//     ON objects.id = head.object_id AND objects.head = head.name
		// INNER JOIN ocfl_index_object_versions v1
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

func (idx *Backend) GetObject(ctx context.Context, objID string) (*index.Object, error) {
	qry := sqlc.New(idx)
	obj, err := qry.GetObjectID(ctx, objID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("object id '%s': %w", objID, index.ErrNotFound)
		}
	}
	ret, err := idx.asIndexObject(ctx, &obj)
	if err != nil {
		return nil, fmt.Errorf("while getting object id '%s': %w", objID, err)
	}
	return ret, nil
}

func (idx *Backend) GetObjectByPath(ctx context.Context, p string) (*index.Object, error) {
	qry := sqlc.New(idx)
	obj, err := qry.GetObjectPath(ctx, p)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("object path '%s': %w", p, index.ErrNotFound)
		}
		return nil, err
	}
	ret, err := idx.asIndexObject(ctx, &obj)
	if err != nil {
		return nil, fmt.Errorf("while getting object path '%s': %w", p, err)
	}
	return ret, nil
}

func (idx *Backend) asIndexObject(ctx context.Context, sqlObj *sqlc.OcflIndexObject) (*index.Object, error) {
	qry := sqlc.New(idx)
	rows, err := qry.ListObjectVersions(ctx, sqlObj.OcflID)
	if err != nil {
		return nil, err
	}
	vers := make([]*index.ObjectVersion, len(rows))
	for i := 0; i < len(rows); i++ {
		ver := &index.ObjectVersion{
			Message: rows[i].Message,
			Created: rows[i].Created,
			Size:    rows[i].Size,
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
		ID:              sqlObj.OcflID,
		DigestAlgorithm: sqlObj.DigestAlgorithm,
		InventoryDigest: sqlObj.InventoryDigest,
		RootPath:        sqlObj.RootPath,
		Versions:        vers,
	}
	if err := ocfl.ParseVNum(sqlObj.Head, &result.Head); err != nil {
		return nil, err
	}
	if err := ocfl.ParseSpec(sqlObj.Spec, &result.Spec); err != nil {
		return nil, err
	}
	return result, nil
}

func (db *Backend) GetObjectState(ctx context.Context, id string, v ocfl.VNum, p string, recr bool, lim int, cur string) (*index.PathInfo, error) {
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
	if !v.Empty() {
		vStr = v.String()
	}
	var baseNode = struct {
		id     int64
		sumbyt []byte
		size   int64
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
		Sum:   hex.EncodeToString(baseNode.sumbyt),
		IsDir: baseNode.isdir,
		Size:  baseNode.size,
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
		result.Children[i] = index.PathItem{
			IsDir: r.Dir,
			Name:  r.Name,
			Size:  r.Size,
			Sum:   hex.EncodeToString(r.Sum),
		}
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
	return path.Join(result.RootPath, result.FilePath), nil
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
	nodeID, isNew, err := getInsertNode(ctx, tx, node.Val.Sum, node.IsDir(), node.Val.Size)
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
		child := node.Child(n)
		if _, err = addIndexNodes(ctx, tx, child, nodeID, n); err != nil {
			return 0, err
		}
	}
	return nodeID, nil
}

// getInsertNode gets or inserts the node for sum/dir. If the node is created, the
// returned boolean is true.
func getInsertNode(ctx context.Context, qry *sqlc.Queries, sum []byte, dir bool, size int64) (int64, bool, error) {
	id, err := qry.GetNodeSum(ctx, sqlc.GetNodeSumParams{
		Sum: sum,
		Dir: dir,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
		id, err = qry.InsertNode(ctx, sqlc.InsertNodeParams{
			Size: size,
			Sum:  sum,
			Dir:  dir,
		})
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	}
	return id, false, nil
}

// upsertObject inserts or updates objects values in the database. It returns
// the object's internal rowid, a booleen indicating if an insert or update
// occured, and an error value. If the object exists and the InventoryDigest is
// unchanged, the boolean is false.
func upsertObject(ctx context.Context, qry *sqlc.Queries, vals *index.Object) (int64, bool, error) {
	obj, err := qry.GetObjectID(ctx, vals.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, false, err
		}
		id, err := qry.InsertObject(ctx, sqlc.InsertObjectParams{
			OcflID:          vals.ID,
			Head:            vals.Head.String(),
			Spec:            vals.Spec.String(),
			DigestAlgorithm: vals.DigestAlgorithm,
			RootPath:        vals.RootPath,
			InventoryDigest: vals.InventoryDigest,
		})
		if err != nil {
			return 0, false, err
		}
		return id, true, nil
	}
	if obj.InventoryDigest != vals.InventoryDigest {
		err = qry.UpdateObject(ctx, sqlc.UpdateObjectParams{
			OcflID:          vals.ID,
			Head:            vals.Head.String(),
			Spec:            vals.Spec.String(),
			DigestAlgorithm: vals.DigestAlgorithm,
			RootPath:        vals.RootPath,
			InventoryDigest: vals.InventoryDigest,
			ID:              obj.ID, // internal id
		})
		if err != nil {
			return 0, false, err
		}
		return obj.ID, true, nil
	}
	return obj.ID, false, nil
}

func insertVersion(ctx context.Context, qry *sqlc.Queries, ver *index.ObjectVersion, objID int64, nodeID int64) error {
	idxV, err := qry.GetObjectVersion(ctx, sqlc.GetObjectVersionParams{
		ObjectID: objID,
		Num:      int64(ver.Num.Num()),
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		params := sqlc.InsertObjectVersionParams{
			ObjectID: objID,
			Name:     ver.Num.String(),
			Message:  ver.Message,
			Created:  ver.Created.UTC(), // location prevents errors reading from db
			NodeID:   nodeID,
		}
		if ver.User != nil {
			params.UserAddress = ver.User.Address
			params.UserName = ver.User.Name
		}
		if _, err := qry.InsertObjectVersion(ctx, params); err != nil {
			return err
		}
		return nil
	}
	if idxV.NodeID != nodeID {
		return fmt.Errorf("the content for version '%s' has changed since it was last indexed", ver.Num)
	}
	return nil
}

func insertContent(ctx context.Context, tx *sqlc.Queries, node *pathtree.Node[index.IndexingVal], objID int64) error {
	return pathtree.Walk(node, func(name string, node *pathtree.Node[index.IndexingVal]) error {
		params := sqlc.InsertContentPathIgnoreParams{
			Sum:      node.Val.Sum,
			FilePath: node.Val.Path,
			ObjectID: objID,
		}
		if err := tx.InsertContentPathIgnore(ctx, params); err != nil {
			return fmt.Errorf("adding content path: '%s'", node.Val.Path)
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

func (db *Backend) DEBUG_AllObjects(ctx context.Context) ([]sqlc.OcflIndexObject, error) {
	qry := sqlc.New(db)
	return qry.DebugAllObjects(ctx)
}

func (db *Backend) DEBUG_AllVersions(ctx context.Context) ([]sqlc.OcflIndexObjectVersion, error) {
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
