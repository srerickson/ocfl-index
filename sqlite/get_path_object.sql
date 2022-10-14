-- returns node using object id and path
WITH RECURSIVE
    paths(id, path) AS (
        SELECT names.node_id, names.name
        FROM ocfl_index_names names
        INNER JOIN ocfl_index_objects objects ON names.parent_id = objects.node_id
        INNER JOIN ocfl_index_nodes child ON names.node_id = child.id
        WHERE objects.ocfl_id = ?1
        AND ?2 || '/' LIKE names.name || '/' ||'%'
    UNION
        SELECT names.node_id, paths.path || '/' || names.name
        FROM ocfl_index_names names
        INNER JOIN paths ON names.parent_id = paths.id
        INNER JOIN ocfl_index_nodes nodes ON names.node_id = nodes.id
        WHERE ?2 LIKE paths.path || '/' || names.name || '%'
    )
SELECT paths.id, nodes.sum, nodes.dir FROM paths
    INNER JOIN ocfl_index_nodes nodes ON paths.id = nodes.id
WHERE paths.path = ?2 LIMIT 1;