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
	"github.com/srerickson/ocfl-index/internal/sqlite/sqlc"
	"github.com/srerickson/ocfl/ocflv1"
	_ "modernc.org/sqlite"
)

const (
	tablePrefix    = "ocfl_index_"
	sqliteSettings = "_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared"
	defaultLimit   = 1000
)

var (
	// expected schema for index file
	// keep in sync with schema.sql
	schemaVer = sqlc.OcflIndexSchema{Major: 0, Minor: 4}

	//go:embed schema.sql
	querySchema string

	//go:embed get_node_by_path.sql
	queryGetNodeByPath string

	//go:embed get_node_children.sql
	queryGetNodeChildren string

	queryListTables string = `SELECT name FROM sqlite_master WHERE type='table';`
)

// Backend is a sqlite-based implementation of index.Backend
type Backend struct {
	sql.DB
}

var _ index.Backend = (*Backend)(nil)

// Open returns a new Backend using connection string conf, which is passed
// directory to sql.Open. The conf string should include the format:
//
//	file:name.sql?_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared
func Open(conf string) (*Backend, error) {
	db, err := sql.Open("sqlite", conf)
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA case_sensitive_like=ON;")
	db.Exec("PRAGMA foreign_keys=ON;")
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

func (db *Backend) GetIndexSummary(ctx context.Context) (index.IndexSummary, error) {
	qry := sqlc.New(&db.DB)
	invs, err := qry.CountInventories(ctx)
	if err != nil {
		return index.IndexSummary{}, err
	}
	objs, err := qry.CountObjectRoots(ctx)
	if err != nil {
		return index.IndexSummary{}, err
	}
	last, err := qry.GetObjectRootLastIndexedAt(ctx)
	if err != nil {
		return index.IndexSummary{}, err
	}
	summ := index.IndexSummary{
		NumInventories: int(invs),
		NumObjects:     int(objs),
		UpdatedAt:      last,
	}
	return summ, nil
}

// List entries for object roots table
func (db *Backend) ListObjectRoots(ctx context.Context, limit int, cursor string) (*index.ObjectRootList, error) {
	return listObjectRootsTx(ctx, sqlc.New(db), limit, cursor)
}

func listObjectRootsTx(ctx context.Context, qry *sqlc.Queries, limit int, cursor string) (*index.ObjectRootList, error) {
	if limit < 1 || limit > 1000 {
		limit = defaultLimit
	}
	// add 1 to limit to see if there are more items
	roots, err := qry.ListObjectRoots(ctx, sqlc.ListObjectRootsParams{Path: cursor, Limit: int64(limit + 1)})
	if err != nil {
		return nil, err
	}
	result := &index.ObjectRootList{}
	if len(roots) == limit+1 {
		result.ObjectRoots = make([]index.ObjectRootListItem, limit)
		// cursor is the path of the last item in the results
		result.NextCursor = roots[limit-1].Path
	} else {
		result.ObjectRoots = make([]index.ObjectRootListItem, len(roots))
	}
	for i := range result.ObjectRoots {
		result.ObjectRoots[i] = index.ObjectRootListItem{
			Path:      roots[i].Path,
			IndexedAt: roots[i].IndexedAt,
		}
	}
	return result, nil
}

// We can't use sqlc here because we need to alter the query for different sort/cursor values.
func (idx *Backend) ListObjects(ctx context.Context, sort index.ObjectSort, limit int, cursor string) (*index.ObjectList, error) {
	if limit < 1 || limit > 1000 {
		limit = defaultLimit
	}
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
	ret, err := asIndexInventory(ctx, qry, (*sqlc.GetInventoryPathRow)(&obj))
	if err != nil {
		return nil, fmt.Errorf("while getting object id '%s': %w", objID, err)
	}
	return ret, nil
}

func (db *Backend) GetObjectByPath(ctx context.Context, p string) (*index.Object, error) {
	return getObjectByPathTx(ctx, sqlc.New(db), p)
}

func getObjectByPathTx(ctx context.Context, tx *sqlc.Queries, p string) (*index.Object, error) {
	obj, err := tx.GetInventoryPath(ctx, p)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("object path '%s': %w", p, index.ErrNotFound)
		}
		return nil, err
	}
	ret, err := asIndexInventory(ctx, tx, &obj)
	if err != nil {
		return nil, fmt.Errorf("while getting object path '%s': %w", p, err)
	}
	return ret, nil
}

func asIndexInventory(ctx context.Context, qry *sqlc.Queries, sqlInv *sqlc.GetInventoryPathRow) (*index.Object, error) {
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

func (db *Backend) GetObjectState(ctx context.Context, id string, v ocfl.VNum, p string, recursive bool, lim int, cur string) (*index.PathInfo, error) {
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
	row := db.QueryRowContext(ctx, queryGetNodeByPath, id, vStr, p)
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
	limParam := int64(lim + 1) // limit+1 to check for next page
	var err error
	if recursive {
		// result includes all descendants of the node
		result.Children, err = db.getNodeChildrenAll(ctx, baseNode.id, limParam, cur)
	} else {
		// result includes imediate children of the node
		result.Children, err = db.getNodeChildren(ctx, qry, baseNode.id, limParam, cur)
	}
	if err != nil {
		errFn(err)
	}
	// check if there are additional results
	if l := len(result.Children); l == lim+1 {
		result.Children = result.Children[:lim]
		result.NextCursor = result.Children[lim-1].Name
	}
	return result, nil
}

func (db *Backend) getNodeChildren(ctx context.Context, qry *sqlc.Queries, nodeID int64, limit int64, cursor string) ([]index.PathItem, error) {
	rows, err := qry.NodeDirChildren(ctx, sqlc.NodeDirChildrenParams{
		ParentID: nodeID,
		Name:     cursor,
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}
	children := make([]index.PathItem, len(rows))
	for i, r := range rows {
		item := index.PathItem{
			IsDir:   r.Dir,
			Name:    r.Name,
			Sum:     hex.EncodeToString(r.Sum),
			Size:    r.Size.Int64,
			HasSize: r.Size.Valid,
		}
		children[i] = item
	}
	return children, nil
}

func (db *Backend) getNodeChildrenAll(ctx context.Context, nodeID int64, limit int64, cursor string) ([]index.PathItem, error) {
	var paths []index.PathItem
	rows, err := db.QueryContext(ctx, queryGetNodeChildren, nodeID, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			p        index.PathItem
			id       int64
			sumBytes []byte
			size     sql.NullInt64
		)
		// paths.id, paths.path, nodes.sum, nodes.size
		if err := rows.Scan(&id, &p.Name, &sumBytes, &size); err != nil {
			return nil, err
		}
		p.Sum = hex.EncodeToString(sumBytes)
		p.Size = size.Int64
		p.HasSize = size.Valid
		paths = append(paths, p)
	}
	return paths, nil
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
