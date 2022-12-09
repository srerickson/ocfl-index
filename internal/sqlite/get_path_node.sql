-- returns node using digest of parent and relative path
WITH RECURSIVE
    paths(id, path) AS (
        SELECT names.node_id, names.name
        FROM ocfl_index_names names
        INNER JOIN ocfl_index_nodes child ON names.node_id = child.id
        INNER JOIN ocfl_index_nodes parent ON names.parent_id = parent.id
        WHERE parent.sum = ?1
        AND parent.dir IS true
        AND ?2 || '/' LIKE names.name || '/' ||'%'
    UNION
        SELECT names.node_id, paths.path || "/" || names.name
        FROM ocfl_index_names names
        INNER JOIN paths ON names.parent_id = paths.id
        INNER JOIN ocfl_index_nodes nodes ON names.node_id = nodes.id
        WHERE ?2 LIKE paths.path || "/" || names.name || '%'
    )
SELECT paths.*, hex(nodes.sum), nodes.dir, content.file_path from paths 
INNER JOIN ocfl_index_nodes nodes ON nodes.id = paths.id
LEFT JOIN ocfl_index_content_paths content ON content.node_id = paths.id
where paths.path = ?2 LIMIT 1;
