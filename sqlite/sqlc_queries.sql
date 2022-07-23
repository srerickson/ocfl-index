-- sqlc definitions

--
-- Index Schema Version
-- 
-- name: GetSchemaVersion :one
SELECT * FROM ocfl_index_schema LIMIT 1;


--
-- OCFL Object
-- 
-- name: InsertObject :execlastid
INSERT INTO ocfl_index_objects (uri, node_id, head) values (?, ?, ?);

-- name: GetObjectURI :one
SELECT * FROM ocfl_index_objects WHERE uri = ?;

-- name: ListObjects :many
SELECT 
    objects.id, 
    objects.uri,
    objects.head,
    versions.created version_created, 
    objects.node_id object_node_id,
    names.node_id head_node_id
FROM ocfl_index_objects objects
INNER JOIN ocfl_index_object_versions versions 
    ON objects.id = versions.object_id AND objects.head = versions.name
INNER JOIN ocfl_index_names names 
    ON names.parent_id = objects.node_id AND names.name = objects.head
ORDER BY versions.created DESC;

-- name: UpdateObject :exec
UPDATE ocfl_index_objects SET node_id = ?, head = ? WHERE id = ?;

-- name: DeleteObject :exec
DELETE from ocfl_index_objects WHERE id = ?;


--
-- OCFL Object Versions
--
-- name: InsertObjectVersion :execlastid
INSERT INTO ocfl_index_object_versions 
    (object_id, num, name, message, created, user_name, user_address)
    VALUES (?,?,?,?,?,?,?);

-- name: ListObjectVersions :many
SELECT versions.*, names.node_id node_id FROM ocfl_index_object_versions versions
INNER JOIN ocfl_index_objects objects ON objects.id = versions.object_id
INNER JOIN ocfl_index_names names ON names.parent_id = objects.node_id AND names.name = versions.name
WHERE objects.uri = ? ORDER BY versions.num ASC;

-- name: DeleteObjectVersions :exec
DELETE from ocfl_index_object_versions WHERE object_id = ?;


--
-- Nodes
--
-- name: InsertNode :execlastid
INSERT INTO ocfl_index_nodes (sum, dir) values (?, ?);

-- name: GetNodeSum :one 
SELECT id from ocfl_index_nodes WHERE sum = ? AND dir = ?;

-- name: NodeChildren :many
SELECT child.id, names.name, child.dir, child.sum FROM ocfl_index_nodes child
INNER JOIN ocfl_index_names names ON child.id = names.node_id
WHERE names.parent_id = ?;


--
-- Names
--
-- name: InsertNameIgnore :exec
INSERT OR IGNORE INTO ocfl_index_names (name, node_id, parent_id) values (?,?,?);


--
-- Content Paths
--
-- name: InsertContentPathIgnore :exec
INSERT OR IGNORE INTO ocfl_index_content_paths (object_id, node_id, file_path) VALUES (
    ?,
    (SELECT id FROM ocfl_index_nodes WHERE sum = ? AND dir IS FALSE LIMIT 1),
    ?);

-- name: GetContentPath :one
SELECT file_path from ocfl_index_content_paths paths
INNER JOIN ocfl_index_objects objects on paths.object_id = objects.id
INNER JOIN ocfl_index_nodes nodes on nodes.id = paths.node_id
WHERE objects.uri = ? AND nodes.sum = ?; 
