-- get all files below a certain node
-- ?1: node digest
-- ?2: cursor for pagination
-- ?3: page limit
WITH RECURSIVE
    paths(id, path) AS (
        SELECT names.node_id, names.name
        FROM ocfl_index_names names
        WHERE names.parent_id = ?1
    UNION
        SELECT names.node_id, paths.path || '/' || names.name
        FROM paths
        INNER JOIN ocfl_index_names names ON names.parent_id = paths.id
    )
SELECT paths.id, paths.path, nodes.sum, nodes.size
FROM paths 
INNER JOIN ocfl_index_nodes nodes ON paths.id = nodes.id
WHERE nodes.dir = FALSE AND paths.path > ?2 ORDER BY path ASC LIMIT ?3;