-- sqlc definitions

--
-- Index Schema Version
-- 
-- name: GetSchemaVersion :one
SELECT * FROM ocfl_index_schema LIMIT 1;


--
-- OCFL Object Roots
--

-- name: GetObjectRoot :one
SELECT * from ocfl_index_object_roots WHERE path = ?;

-- name: UpsertObjectRoot :one
INSERT INTO ocfl_index_object_roots (path, indexed_at) VALUES (?1, ?2) 
    ON CONFLICT(path) DO UPDATE SET indexed_at=?2
RETURNING *;

-- name: ListObjectRoots :many 
SELECT * FROM ocfl_index_object_roots roots 
WHERE roots.path > ?1 ORDER BY roots.path ASC LIMIT ?2;

-- name: DeleteObjectRootsBefore :exec
DELETE FROM ocfl_index_object_roots WHERE indexed_at < ?1;

-- name: CountObjectRoots :one
SELECT COUNT(id) from ocfl_index_object_roots;

-- name: GetObjectRootLastIndexedAt :one
SELECT indexed_at from ocfl_index_object_roots ORDER BY indexed_at DESC LIMIT 1;

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

-- name: ListInventoriesPrefix :many
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
ORDER BY invs.ocfl_id ASC LIMIT ?3;

-- name: ListInventories :many
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
ORDER BY invs.ocfl_id ASC LIMIT ?2;

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