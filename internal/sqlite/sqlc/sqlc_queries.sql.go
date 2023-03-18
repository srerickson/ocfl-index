// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.17.2
// source: sqlc_queries.sql

package sqlc

import (
	"context"
	"database/sql"
	"time"
)

const countInventories = `-- name: CountInventories :one
SELECT COUNT(id) from ocfl_index_inventories
`

func (q *Queries) CountInventories(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, countInventories)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const countObjectRoots = `-- name: CountObjectRoots :one
SELECT COUNT(id) from ocfl_index_object_roots
`

func (q *Queries) CountObjectRoots(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, countObjectRoots)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const debugAllInventories = `-- name: DebugAllInventories :many
SELECT id, root_id, ocfl_id, spec, digest_algorithm, inventory_digest, head, indexed_at from ocfl_index_inventories
`

func (q *Queries) DebugAllInventories(ctx context.Context) ([]OcflIndexInventory, error) {
	rows, err := q.db.QueryContext(ctx, debugAllInventories)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OcflIndexInventory
	for rows.Next() {
		var i OcflIndexInventory
		if err := rows.Scan(
			&i.ID,
			&i.RootID,
			&i.OcflID,
			&i.Spec,
			&i.DigestAlgorithm,
			&i.InventoryDigest,
			&i.Head,
			&i.IndexedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const debugAllNames = `-- name: DebugAllNames :many
SELECT name, node_id, parent_id FROM ocfl_index_names
`

func (q *Queries) DebugAllNames(ctx context.Context) ([]OcflIndexName, error) {
	rows, err := q.db.QueryContext(ctx, debugAllNames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OcflIndexName
	for rows.Next() {
		var i OcflIndexName
		if err := rows.Scan(&i.Name, &i.NodeID, &i.ParentID); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const debugAllNodes = `-- name: DebugAllNodes :many
SELECT id, dir, sum, size FROM ocfl_index_nodes
`

func (q *Queries) DebugAllNodes(ctx context.Context) ([]OcflIndexNode, error) {
	rows, err := q.db.QueryContext(ctx, debugAllNodes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OcflIndexNode
	for rows.Next() {
		var i OcflIndexNode
		if err := rows.Scan(
			&i.ID,
			&i.Dir,
			&i.Sum,
			&i.Size,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const debugAllObjectRoots = `-- name: DebugAllObjectRoots :many
SELECT id, path, indexed_at from ocfl_index_object_roots
`

func (q *Queries) DebugAllObjectRoots(ctx context.Context) ([]OcflIndexObjectRoot, error) {
	rows, err := q.db.QueryContext(ctx, debugAllObjectRoots)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OcflIndexObjectRoot
	for rows.Next() {
		var i OcflIndexObjectRoot
		if err := rows.Scan(&i.ID, &i.Path, &i.IndexedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const debugAllVersions = `-- name: DebugAllVersions :many
SELECT inventory_id, num, name, message, created, user_name, user_address, node_id from ocfl_index_versions
`

func (q *Queries) DebugAllVersions(ctx context.Context) ([]OcflIndexVersion, error) {
	rows, err := q.db.QueryContext(ctx, debugAllVersions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OcflIndexVersion
	for rows.Next() {
		var i OcflIndexVersion
		if err := rows.Scan(
			&i.InventoryID,
			&i.Num,
			&i.Name,
			&i.Message,
			&i.Created,
			&i.UserName,
			&i.UserAddress,
			&i.NodeID,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const deleteInventory = `-- name: DeleteInventory :exec
DELETE from ocfl_index_inventories WHERE id = ?
`

func (q *Queries) DeleteInventory(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, deleteInventory, id)
	return err
}

const deleteObjectRootsBefore = `-- name: DeleteObjectRootsBefore :exec
DELETE FROM ocfl_index_object_roots WHERE indexed_at < ?1
`

func (q *Queries) DeleteObjectRootsBefore(ctx context.Context, indexedAt time.Time) error {
	_, err := q.db.ExecContext(ctx, deleteObjectRootsBefore, indexedAt)
	return err
}

const deleteVersions = `-- name: DeleteVersions :exec
DELETE from ocfl_index_versions WHERE inventory_id = ?
`

func (q *Queries) DeleteVersions(ctx context.Context, inventoryID int64) error {
	_, err := q.db.ExecContext(ctx, deleteVersions, inventoryID)
	return err
}

const getContentPath = `-- name: GetContentPath :one
SELECT cont.file_path, objs.path from ocfl_index_content_paths cont
INNER JOIN ocfl_index_inventories invs ON cont.inventory_id = invs.id
INNER JOIN ocfl_index_object_roots objs ON invs.root_id = objs.id
INNER JOIN ocfl_index_nodes nodes on nodes.id = cont.node_id  AND nodes.dir IS FALSE
WHERE nodes.sum = ? LIMIT 1
`

type GetContentPathRow struct {
	FilePath string
	Path     string
}

func (q *Queries) GetContentPath(ctx context.Context, sum []byte) (GetContentPathRow, error) {
	row := q.db.QueryRowContext(ctx, getContentPath, sum)
	var i GetContentPathRow
	err := row.Scan(&i.FilePath, &i.Path)
	return i, err
}

const getInventoryID = `-- name: GetInventoryID :one
SELECT invs.id, invs.root_id, invs.ocfl_id, invs.spec, invs.digest_algorithm, invs.inventory_digest, invs.head, invs.indexed_at, objs.path FROM ocfl_index_inventories invs
INNER JOIN ocfl_index_object_roots objs ON objs.id = invs.root_id
WHERE ocfl_id = ?
`

type GetInventoryIDRow struct {
	ID              int64
	RootID          int64
	OcflID          string
	Spec            string
	DigestAlgorithm string
	InventoryDigest string
	Head            string
	IndexedAt       time.Time
	Path            string
}

func (q *Queries) GetInventoryID(ctx context.Context, ocflID string) (GetInventoryIDRow, error) {
	row := q.db.QueryRowContext(ctx, getInventoryID, ocflID)
	var i GetInventoryIDRow
	err := row.Scan(
		&i.ID,
		&i.RootID,
		&i.OcflID,
		&i.Spec,
		&i.DigestAlgorithm,
		&i.InventoryDigest,
		&i.Head,
		&i.IndexedAt,
		&i.Path,
	)
	return i, err
}

const getInventoryPath = `-- name: GetInventoryPath :one
SELECT invs.id, invs.root_id, invs.ocfl_id, invs.spec, invs.digest_algorithm, invs.inventory_digest, invs.head, invs.indexed_at, objs.path FROM ocfl_index_inventories invs
INNER JOIN ocfl_index_object_roots objs ON objs.id = invs.root_id
WHERE objs.path = ?
`

type GetInventoryPathRow struct {
	ID              int64
	RootID          int64
	OcflID          string
	Spec            string
	DigestAlgorithm string
	InventoryDigest string
	Head            string
	IndexedAt       time.Time
	Path            string
}

func (q *Queries) GetInventoryPath(ctx context.Context, path string) (GetInventoryPathRow, error) {
	row := q.db.QueryRowContext(ctx, getInventoryPath, path)
	var i GetInventoryPathRow
	err := row.Scan(
		&i.ID,
		&i.RootID,
		&i.OcflID,
		&i.Spec,
		&i.DigestAlgorithm,
		&i.InventoryDigest,
		&i.Head,
		&i.IndexedAt,
		&i.Path,
	)
	return i, err
}

const getInventoryRowID = `-- name: GetInventoryRowID :one
SELECT id from ocfl_index_inventories where ocfl_id = ?
`

func (q *Queries) GetInventoryRowID(ctx context.Context, ocflID string) (int64, error) {
	row := q.db.QueryRowContext(ctx, getInventoryRowID, ocflID)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const getNodeSum = `-- name: GetNodeSum :one
SELECT id, dir, sum, size from ocfl_index_nodes WHERE sum = ? AND dir = ?
`

type GetNodeSumParams struct {
	Sum []byte
	Dir bool
}

func (q *Queries) GetNodeSum(ctx context.Context, arg GetNodeSumParams) (OcflIndexNode, error) {
	row := q.db.QueryRowContext(ctx, getNodeSum, arg.Sum, arg.Dir)
	var i OcflIndexNode
	err := row.Scan(
		&i.ID,
		&i.Dir,
		&i.Sum,
		&i.Size,
	)
	return i, err
}

const getObjectRoot = `-- name: GetObjectRoot :one

SELECT id, path, indexed_at from ocfl_index_object_roots WHERE path = ?
`

// OCFL Object Roots
func (q *Queries) GetObjectRoot(ctx context.Context, path string) (OcflIndexObjectRoot, error) {
	row := q.db.QueryRowContext(ctx, getObjectRoot, path)
	var i OcflIndexObjectRoot
	err := row.Scan(&i.ID, &i.Path, &i.IndexedAt)
	return i, err
}

const getObjectRootLastIndexedAt = `-- name: GetObjectRootLastIndexedAt :one
SELECT indexed_at from ocfl_index_object_roots ORDER BY indexed_at DESC LIMIT 1
`

func (q *Queries) GetObjectRootLastIndexedAt(ctx context.Context) (time.Time, error) {
	row := q.db.QueryRowContext(ctx, getObjectRootLastIndexedAt)
	var indexed_at time.Time
	err := row.Scan(&indexed_at)
	return indexed_at, err
}

const getSchemaVersion = `-- name: GetSchemaVersion :one

SELECT major, minor FROM ocfl_index_schema LIMIT 1
`

// sqlc definitions
//
// Index Schema Version
func (q *Queries) GetSchemaVersion(ctx context.Context) (OcflIndexSchema, error) {
	row := q.db.QueryRowContext(ctx, getSchemaVersion)
	var i OcflIndexSchema
	err := row.Scan(&i.Major, &i.Minor)
	return i, err
}

const getVersion = `-- name: GetVersion :one
SELECT inventory_id, num, name, message, created, user_name, user_address, node_id from ocfl_index_versions WHERE inventory_id = ?1 and num = ?2
`

type GetVersionParams struct {
	InventoryID int64
	Num         int64
}

func (q *Queries) GetVersion(ctx context.Context, arg GetVersionParams) (OcflIndexVersion, error) {
	row := q.db.QueryRowContext(ctx, getVersion, arg.InventoryID, arg.Num)
	var i OcflIndexVersion
	err := row.Scan(
		&i.InventoryID,
		&i.Num,
		&i.Name,
		&i.Message,
		&i.Created,
		&i.UserName,
		&i.UserAddress,
		&i.NodeID,
	)
	return i, err
}

const insertIgnoreContentPath = `-- name: InsertIgnoreContentPath :exec
INSERT OR IGNORE INTO ocfl_index_content_paths (inventory_id, node_id, file_path) VALUES (
    ?,
    (SELECT id FROM ocfl_index_nodes WHERE sum = ? AND dir IS FALSE LIMIT 1),
    ?)
`

type InsertIgnoreContentPathParams struct {
	InventoryID int64
	Sum         []byte
	FilePath    string
}

// Content Paths
func (q *Queries) InsertIgnoreContentPath(ctx context.Context, arg InsertIgnoreContentPathParams) error {
	_, err := q.db.ExecContext(ctx, insertIgnoreContentPath, arg.InventoryID, arg.Sum, arg.FilePath)
	return err
}

const insertIgnoreName = `-- name: InsertIgnoreName :exec
INSERT OR IGNORE INTO ocfl_index_names (name, node_id, parent_id) values (?,?,?)
`

type InsertIgnoreNameParams struct {
	Name     string
	NodeID   int64
	ParentID int64
}

// Names
func (q *Queries) InsertIgnoreName(ctx context.Context, arg InsertIgnoreNameParams) error {
	_, err := q.db.ExecContext(ctx, insertIgnoreName, arg.Name, arg.NodeID, arg.ParentID)
	return err
}

const insertNode = `-- name: InsertNode :execlastid
INSERT INTO ocfl_index_nodes (sum, dir, size) values (?, ?, ?)
`

type InsertNodeParams struct {
	Sum  []byte
	Dir  bool
	Size sql.NullInt64
}

// Nodes
func (q *Queries) InsertNode(ctx context.Context, arg InsertNodeParams) (int64, error) {
	result, err := q.db.ExecContext(ctx, insertNode, arg.Sum, arg.Dir, arg.Size)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

const insertVersion = `-- name: InsertVersion :execlastid
INSERT INTO ocfl_index_versions 
    (inventory_id, name, message, created, user_name, user_address, node_id, num)
    VALUES (?1,?2,?3,?4,?5,?6,?7, CAST(LTRIM(?2,'v') AS INT))
`

type InsertVersionParams struct {
	InventoryID int64
	Name        string
	Message     string
	Created     time.Time
	UserName    string
	UserAddress string
	NodeID      int64
}

// OCFL Object Versions
func (q *Queries) InsertVersion(ctx context.Context, arg InsertVersionParams) (int64, error) {
	result, err := q.db.ExecContext(ctx, insertVersion,
		arg.InventoryID,
		arg.Name,
		arg.Message,
		arg.Created,
		arg.UserName,
		arg.UserAddress,
		arg.NodeID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

const listInventories = `-- name: ListInventories :many
SELECT 
    invs.id,
    invs.ocfl_id,
    root.path,
    invs.spec,
    invs.head,
    v1.created v1_created,
    head.created head_created
FROM ocfl_index_inventories invs
INNER JOIN ocfl_index_object_roots root
    ON invs.root_id = root.id
INNER JOIN ocfl_index_versions head
    ON invs.id = head.inventory_id AND invs.head = head.name
INNER JOIN ocfl_index_versions v1
    ON invs.id = v1.inventory_id AND v1.num = 1
WHERE invs.ocfl_id > ?1
ORDER BY invs.ocfl_id ASC LIMIT ?2
`

type ListInventoriesParams struct {
	OcflID string
	Limit  int64
}

type ListInventoriesRow struct {
	ID        int64
	OcflID    string
	Path      string
	Spec      string
	Head      string
	Created   time.Time
	Created_2 time.Time
}

func (q *Queries) ListInventories(ctx context.Context, arg ListInventoriesParams) ([]ListInventoriesRow, error) {
	rows, err := q.db.QueryContext(ctx, listInventories, arg.OcflID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListInventoriesRow
	for rows.Next() {
		var i ListInventoriesRow
		if err := rows.Scan(
			&i.ID,
			&i.OcflID,
			&i.Path,
			&i.Spec,
			&i.Head,
			&i.Created,
			&i.Created_2,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listInventoriesPrefix = `-- name: ListInventoriesPrefix :many
SELECT 
    invs.id,
    invs.ocfl_id,
    root.path,
    invs.spec,
    invs.head,
    v1.created v1_created,
    head.created head_created
FROM ocfl_index_inventories invs
INNER JOIN ocfl_index_object_roots root
    ON invs.root_id = root.id
INNER JOIN ocfl_index_versions head
    ON invs.id = head.inventory_id AND invs.head = head.name
INNER JOIN ocfl_index_versions v1
    ON invs.id = v1.inventory_id AND v1.num = 1
WHERE invs.ocfl_id > ?1 AND invs.ocfl_id LIKE ?2 || '%' ESCAPE '\'
ORDER BY invs.ocfl_id ASC LIMIT ?3
`

type ListInventoriesPrefixParams struct {
	OcflID   string
	OcflID_2 string
	Limit    int64
}

type ListInventoriesPrefixRow struct {
	ID        int64
	OcflID    string
	Path      string
	Spec      string
	Head      string
	Created   time.Time
	Created_2 time.Time
}

func (q *Queries) ListInventoriesPrefix(ctx context.Context, arg ListInventoriesPrefixParams) ([]ListInventoriesPrefixRow, error) {
	rows, err := q.db.QueryContext(ctx, listInventoriesPrefix, arg.OcflID, arg.OcflID_2, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListInventoriesPrefixRow
	for rows.Next() {
		var i ListInventoriesPrefixRow
		if err := rows.Scan(
			&i.ID,
			&i.OcflID,
			&i.Path,
			&i.Spec,
			&i.Head,
			&i.Created,
			&i.Created_2,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listObjectContentSize = `-- name: ListObjectContentSize :many
SELECT cont.file_path, nodes.size from ocfl_index_content_paths cont
INNER JOIN ocfl_index_nodes nodes on nodes.id = cont.node_id
INNER JOIN ocfl_index_inventories invs ON cont.inventory_id = invs.id
WHERE invs.ocfl_id = ? AND nodes.size IS NOT NULL
`

type ListObjectContentSizeRow struct {
	FilePath string
	Size     sql.NullInt64
}

func (q *Queries) ListObjectContentSize(ctx context.Context, ocflID string) ([]ListObjectContentSizeRow, error) {
	rows, err := q.db.QueryContext(ctx, listObjectContentSize, ocflID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListObjectContentSizeRow
	for rows.Next() {
		var i ListObjectContentSizeRow
		if err := rows.Scan(&i.FilePath, &i.Size); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listObjectRoots = `-- name: ListObjectRoots :many
SELECT id, path, indexed_at FROM ocfl_index_object_roots roots 
WHERE roots.path > ?1 ORDER BY roots.path ASC LIMIT ?2
`

type ListObjectRootsParams struct {
	Path  string
	Limit int64
}

func (q *Queries) ListObjectRoots(ctx context.Context, arg ListObjectRootsParams) ([]OcflIndexObjectRoot, error) {
	rows, err := q.db.QueryContext(ctx, listObjectRoots, arg.Path, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OcflIndexObjectRoot
	for rows.Next() {
		var i OcflIndexObjectRoot
		if err := rows.Scan(&i.ID, &i.Path, &i.IndexedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listVersions = `-- name: ListVersions :many
SELECT versions.inventory_id, versions.num, versions.name, versions.message, versions.created, versions.user_name, versions.user_address, versions.node_id, nodes.size size FROM ocfl_index_versions versions
INNER JOIN ocfl_index_nodes nodes ON nodes.id = versions.node_id
INNER JOIN ocfl_index_inventories objects ON objects.id = versions.inventory_id
WHERE objects.ocfl_id = ? ORDER BY versions.num ASC
`

type ListVersionsRow struct {
	InventoryID int64
	Num         int64
	Name        string
	Message     string
	Created     time.Time
	UserName    string
	UserAddress string
	NodeID      int64
	Size        sql.NullInt64
}

func (q *Queries) ListVersions(ctx context.Context, ocflID string) ([]ListVersionsRow, error) {
	rows, err := q.db.QueryContext(ctx, listVersions, ocflID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListVersionsRow
	for rows.Next() {
		var i ListVersionsRow
		if err := rows.Scan(
			&i.InventoryID,
			&i.Num,
			&i.Name,
			&i.Message,
			&i.Created,
			&i.UserName,
			&i.UserAddress,
			&i.NodeID,
			&i.Size,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const nodeDirChildren = `-- name: NodeDirChildren :many
SELECT child.id, names.name, child.dir, child.sum, child.size FROM ocfl_index_nodes child
INNER JOIN ocfl_index_names names ON child.id = names.node_id
WHERE names.parent_id = ?1 AND names.name > ?2 ORDER BY names.name ASC LIMIT ?3
`

type NodeDirChildrenParams struct {
	ParentID int64
	Name     string
	Limit    int64
}

type NodeDirChildrenRow struct {
	ID   int64
	Name string
	Dir  bool
	Sum  []byte
	Size sql.NullInt64
}

func (q *Queries) NodeDirChildren(ctx context.Context, arg NodeDirChildrenParams) ([]NodeDirChildrenRow, error) {
	rows, err := q.db.QueryContext(ctx, nodeDirChildren, arg.ParentID, arg.Name, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []NodeDirChildrenRow
	for rows.Next() {
		var i NodeDirChildrenRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Dir,
			&i.Sum,
			&i.Size,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setNodeSize = `-- name: SetNodeSize :exec
UPDATE ocfl_index_nodes SET size = ? WHERE sum = ? AND dir = ?
`

type SetNodeSizeParams struct {
	Size sql.NullInt64
	Sum  []byte
	Dir  bool
}

func (q *Queries) SetNodeSize(ctx context.Context, arg SetNodeSizeParams) error {
	_, err := q.db.ExecContext(ctx, setNodeSize, arg.Size, arg.Sum, arg.Dir)
	return err
}

const updateInventory = `-- name: UpdateInventory :exec
UPDATE ocfl_index_inventories SET 
    spec = ?, 
    digest_algorithm = ?, 
    inventory_digest = ?, 
    head = ?,
    ocfl_id = ?,
    indexed_at = ?
    WHERE id = ?
`

type UpdateInventoryParams struct {
	Spec            string
	DigestAlgorithm string
	InventoryDigest string
	Head            string
	OcflID          string
	IndexedAt       time.Time
	ID              int64
}

func (q *Queries) UpdateInventory(ctx context.Context, arg UpdateInventoryParams) error {
	_, err := q.db.ExecContext(ctx, updateInventory,
		arg.Spec,
		arg.DigestAlgorithm,
		arg.InventoryDigest,
		arg.Head,
		arg.OcflID,
		arg.IndexedAt,
		arg.ID,
	)
	return err
}

const upsertInventory = `-- name: UpsertInventory :one
INSERT INTO ocfl_index_inventories (
    ocfl_id, 
    root_id,
    spec, 
    digest_algorithm, 
    inventory_digest, 
    head,
    indexed_at
) values (?1, ?2, ?3, ?4, ?5, ?6, ?7)
    ON CONFLICT(ocfl_id) DO UPDATE SET 
    root_id=?2,
    spec=?3, 
    digest_algorithm=?4, 
    inventory_digest=?5, 
    head=?6,
    indexed_at=?7
RETURNING id, root_id, ocfl_id, spec, digest_algorithm, inventory_digest, head, indexed_at
`

type UpsertInventoryParams struct {
	OcflID          string
	RootID          int64
	Spec            string
	DigestAlgorithm string
	InventoryDigest string
	Head            string
	IndexedAt       time.Time
}

// OCFL Object Inventory
func (q *Queries) UpsertInventory(ctx context.Context, arg UpsertInventoryParams) (OcflIndexInventory, error) {
	row := q.db.QueryRowContext(ctx, upsertInventory,
		arg.OcflID,
		arg.RootID,
		arg.Spec,
		arg.DigestAlgorithm,
		arg.InventoryDigest,
		arg.Head,
		arg.IndexedAt,
	)
	var i OcflIndexInventory
	err := row.Scan(
		&i.ID,
		&i.RootID,
		&i.OcflID,
		&i.Spec,
		&i.DigestAlgorithm,
		&i.InventoryDigest,
		&i.Head,
		&i.IndexedAt,
	)
	return i, err
}

const upsertObjectRoot = `-- name: UpsertObjectRoot :one
INSERT INTO ocfl_index_object_roots (path, indexed_at) VALUES (?1, ?2) 
    ON CONFLICT(path) DO UPDATE SET indexed_at=?2
RETURNING id, path, indexed_at
`

type UpsertObjectRootParams struct {
	Path      string
	IndexedAt time.Time
}

func (q *Queries) UpsertObjectRoot(ctx context.Context, arg UpsertObjectRootParams) (OcflIndexObjectRoot, error) {
	row := q.db.QueryRowContext(ctx, upsertObjectRoot, arg.Path, arg.IndexedAt)
	var i OcflIndexObjectRoot
	err := row.Scan(&i.ID, &i.Path, &i.IndexedAt)
	return i, err
}
