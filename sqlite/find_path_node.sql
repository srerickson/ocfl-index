-- query to find node id for a given path (example: 'storage/VQD8QWCK/CAALogo.gif').
-- In this version, we also need to specify the root id
WITH RECURSIVE
    paths(id, path, sum, dir) AS (
        SELECT names.node_id, names.name, child.sum, child.dir
        FROM ocfl_index_names names
        INNER JOIN ocfl_index_objects objects ON names.parent_id = objects.node_id
        INNER JOIN ocfl_index_nodes child ON names.node_id = child.id
        WHERE objects.uri = ?1
        AND ?2 LIKE names.name || '/' ||'%'
    UNION
        SELECT names.node_id, paths.path || "/" || names.name, nodes.sum, nodes.dir
        FROM ocfl_index_names names
        INNER JOIN paths ON names.parent_id = paths.id
        INNER JOIN ocfl_index_nodes nodes ON names.node_id = nodes.id
        WHERE ?2 LIKE paths.path || "/" || names.name || '%'
    )
SELECT id, sum, dir from paths where path = ?2;