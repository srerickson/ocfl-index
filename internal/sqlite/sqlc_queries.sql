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
-- OCFL Object
-- 
-- name: InsertObject :execlastid
INSERT INTO ocfl_index_objects (
    ocfl_id, 
    spec, 
    digest_algorithm, 
    inventory_digest, 
    root_path, 
    head
    ) values (?, ?, ?, ?, ?, ?);

-- name: GetObjectID :one
SELECT * FROM ocfl_index_objects WHERE ocfl_id = ?;

-- name: GetObjectPath :one
SELECT * FROM ocfl_index_objects WHERE root_path = ?;

-- name: CountObjects :one
SELECT COUNT(id) from ocfl_index_objects;

-- name: DebugAllObjects :many
SELECT * from ocfl_index_objects;

-- name: UpdateObject :exec
UPDATE ocfl_index_objects SET 
    spec = ?, 
    digest_algorithm = ?, 
    inventory_digest = ?, 
    root_path = ?,
    head = ?,
    ocfl_id = ?
    WHERE id = ?;

-- name: DeleteObject :exec
DELETE from ocfl_index_objects WHERE id = ?;


--
-- OCFL Object Versions
--
-- name: InsertObjectVersion :execlastid
INSERT INTO ocfl_index_object_versions 
    (object_id, name, message, created, user_name, user_address, node_id, num)
    VALUES (?1,?2,?3,?4,?5,?6,?7, CAST(LTRIM(?2,'v') AS INT));

-- name: ListObjectVersions :many
SELECT versions.*, nodes.size size FROM ocfl_index_object_versions versions
INNER JOIN ocfl_index_nodes nodes ON nodes.id = versions.node_id
INNER JOIN ocfl_index_objects objects ON objects.id = versions.object_id
WHERE objects.ocfl_id = ? ORDER BY versions.num ASC;

-- name: GetObjectVersion :one
SELECT * from ocfl_index_object_versions WHERE object_id = ?1 and num = ?2;

-- name: DebugAllVersions :many
SELECT * from ocfl_index_object_versions;

-- name: DeleteObjectVersions :exec
DELETE from ocfl_index_object_versions WHERE object_id = ?;


--
-- Nodes
--
-- name: InsertNode :execlastid
INSERT INTO ocfl_index_nodes (sum, dir, size) values (?, ?, ?);

-- name: GetNodeSum :one 
SELECT id from ocfl_index_nodes WHERE sum = ? AND dir = ?;

-- name: NodeDirChildren :many
SELECT child.id, names.name, child.dir, child.sum, child.size FROM ocfl_index_nodes child
INNER JOIN ocfl_index_names names ON child.id = names.node_id
WHERE names.parent_id = ?1 AND names.name > ?2 ORDER BY names.name ASC LIMIT ?3;

-- name: DebugAllNodes :many
SELECT * FROM ocfl_index_nodes;


--
-- Names
--
-- name: InsertNameIgnore :exec
INSERT OR IGNORE INTO ocfl_index_names (name, node_id, parent_id) values (?,?,?);

-- name: DebugAllNames :many
SELECT * FROM ocfl_index_names;

--
-- Content Paths
--
-- name: InsertContentPathIgnore :exec
INSERT OR IGNORE INTO ocfl_index_content_paths (object_id, node_id, file_path) VALUES (
    ?,
    (SELECT id FROM ocfl_index_nodes WHERE sum = ? AND dir IS FALSE LIMIT 1),
    ?);

-- name: GetContentPath :one
SELECT cont.file_path, objs.root_path from ocfl_index_content_paths cont
INNER JOIN ocfl_index_objects objs on cont.object_id = objs.id
INNER JOIN ocfl_index_nodes nodes on nodes.id = cont.node_id  AND nodes.dir IS FALSE
WHERE nodes.sum = ? LIMIT 1; 