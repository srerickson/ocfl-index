-- all files bellow a certain directory node
WITH RECURSIVE
    -- [node (parent) -> name -> node(child)]
    paths(id, path, sum, dir) AS (
        SELECT names.node_id, names.name, childs.sum, childs.dir
        FROM ocfl_index_names names
        INNER JOIN ocfl_index_nodes childs ON names.node_id = nodes.id
        WHERE names.parent_id=?1
    UNION
        SELECT names.node_id, paths.path || '/' || names.name, nodes.sum, nodes.dir
        FROM ocfl_index_names names
        INNER JOIN paths ON names.parent_id = paths.id
        INNER JOIN ocfl_index_nodes nodes ON names.node_id = nodes.id
    )
SELECT path, dir, sum 
FROM paths 
WHERE paths > ?2 ORDER BY path ASC LIMIT ?3;