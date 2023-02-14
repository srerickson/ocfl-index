-- sqlc definitions

--
-- Index Schema Version
-- 
-- name: GetSchemaVersion :one
SELECT * FROM ocfl_index_schema LIMIT 1;


-- 
-- OCFL Storage Root
-- table has only one row (id = 1).
--
-- name: SetStorageRoot :exec
UPDATE ocfl_index_storage_root SET 
    description = ?,
    root_path = ?,
    spec = ?
WHERE id = 1;

-- name: SetStorageRootIndexed :exec
UPDATE ocfl_index_storage_root SET 
    indexed_at=DATETIME('now')
WHERE id = 1;

-- name: GetStorageRoot :one
SELECT * FROM ocfl_index_storage_root WHERE id = 1;

--
-- OCFL Object Roots
--

-- name: GetObjectRoot :one
SELECT * from ocfl_index_object_roots WHERE path = ?;

-- name: UpsertObjectRoot :one
INSERT INTO ocfl_index_object_roots (path, indexed_at) VALUES (?1, ?2) 
    ON CONFLICT(path) DO UPDATE SET indexed_at=?2
RETURNING *;

-- name: DebugAllObjectRoots :many
SELECT * from ocfl_index_object_roots;

--
-- OCFL Object Inventory
-- 
-- name: UpsertInventory :one
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
RETURNING *;

-- name: GetInventoryRowID :one
SELECT id from ocfl_index_inventories where ocfl_id = ?;

-- name: GetInventoryID :one
SELECT invs.*, objs.path FROM ocfl_index_inventories invs
INNER JOIN ocfl_index_object_roots objs ON objs.id = invs.root_id
WHERE ocfl_id = ?;

-- name: GetInventoryPath :one
SELECT invs.*, objs.path FROM ocfl_index_inventories invs
INNER JOIN ocfl_index_object_roots objs ON objs.id = invs.root_id
WHERE objs.path = ?;

-- name: CountInventories :one
SELECT COUNT(id) from ocfl_index_inventories;

-- name: DebugAllInventories :many
SELECT * from ocfl_index_inventories;

-- name: UpdateInventory :exec
UPDATE ocfl_index_inventories SET 
    spec = ?, 
    digest_algorithm = ?, 
    inventory_digest = ?, 
    head = ?,
    ocfl_id = ?,
    indexed_at = ?
    WHERE id = ?;

-- name: DeleteInventory :exec
DELETE from ocfl_index_inventories WHERE id = ?;


--
-- OCFL Object Versions
--
-- name: InsertVersion :execlastid
INSERT INTO ocfl_index_versions 
    (inventory_id, name, message, created, user_name, user_address, node_id, num)
    VALUES (?1,?2,?3,?4,?5,?6,?7, CAST(LTRIM(?2,'v') AS INT));

-- name: ListVersions :many
SELECT versions.*, nodes.size size FROM ocfl_index_versions versions
INNER JOIN ocfl_index_nodes nodes ON nodes.id = versions.node_id
INNER JOIN ocfl_index_inventories objects ON objects.id = versions.inventory_id
WHERE objects.ocfl_id = ? ORDER BY versions.num ASC;

-- name: GetVersion :one
SELECT * from ocfl_index_versions WHERE inventory_id = ?1 and num = ?2;

-- name: DebugAllVersions :many
SELECT * from ocfl_index_versions;

-- name: DeleteVersions :exec
DELETE from ocfl_index_versions WHERE inventory_id = ?;


--
-- Nodes
--
-- name: InsertNode :execlastid
INSERT INTO ocfl_index_nodes (sum, dir, size) values (?, ?, ?);

-- name: GetNodeSum :one 
SELECT * from ocfl_index_nodes WHERE sum = ? AND dir = ?;

-- name: SetNodeSize :exec
UPDATE ocfl_index_nodes SET size = ? WHERE sum = ? AND dir = ?;


-- name: NodeDirChildren :many
SELECT child.id, names.name, child.dir, child.sum, child.size FROM ocfl_index_nodes child
INNER JOIN ocfl_index_names names ON child.id = names.node_id
WHERE names.parent_id = ?1 AND names.name > ?2 ORDER BY names.name ASC LIMIT ?3;

-- name: DebugAllNodes :many
SELECT * FROM ocfl_index_nodes;


--
-- Names
--
-- name: InsertIgnoreName :exec
INSERT OR IGNORE INTO ocfl_index_names (name, node_id, parent_id) values (?,?,?);

-- name: DebugAllNames :many
SELECT * FROM ocfl_index_names;

--
-- Content Paths
--
-- name: InsertIgnoreContentPath :exec
INSERT OR IGNORE INTO ocfl_index_content_paths (inventory_id, node_id, file_path) VALUES (
    ?,
    (SELECT id FROM ocfl_index_nodes WHERE sum = ? AND dir IS FALSE LIMIT 1),
    ?);

-- name: GetContentPath :one
SELECT cont.file_path, objs.path from ocfl_index_content_paths cont
INNER JOIN ocfl_index_inventories invs ON cont.inventory_id = invs.id
INNER JOIN ocfl_index_object_roots objs ON invs.root_id = objs.id
INNER JOIN ocfl_index_nodes nodes on nodes.id = cont.node_id  AND nodes.dir IS FALSE
WHERE nodes.sum = ? LIMIT 1; 

-- name: ListObjectContentSize :many
SELECT cont.file_path, nodes.size from ocfl_index_content_paths cont
INNER JOIN ocfl_index_nodes nodes on nodes.id = cont.node_id
INNER JOIN ocfl_index_inventories invs ON cont.inventory_id = invs.id
WHERE invs.ocfl_id = ? AND nodes.size IS NOT NULL;