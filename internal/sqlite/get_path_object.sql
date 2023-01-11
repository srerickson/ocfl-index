-- returns node info using: 
-- ?1: objectid
-- ?2: version
-- ?3: path (which may be '.')
WITH RECURSIVE
    paths(id, path) AS (
        SELECT versions.node_id, '.'
        FROM ocfl_index_object_versions versions
        INNER JOIN ocfl_index_objects objects ON versions.object_id = objects.id
        WHERE objects.ocfl_id = ?1
        -- if version is '', use objects.head
        AND versions.name = COALESCE(NULLIF(?2,''), objects.head)
    UNION
        SELECT 
            names.node_id,
            -- if paths.path is '.', no joining slash for next path
            COALESCE(NULLIF(paths.path || '/','./'),'') || names.name as next_path
        FROM ocfl_index_names names
        INNER JOIN paths ON names.parent_id = paths.id
        INNER JOIN ocfl_index_nodes nodes ON names.node_id = nodes.id
        WHERE ?3 = next_path 
            OR ?3 LIKE next_path || '/' || '%'
    )
SELECT paths.id, nodes.sum, nodes.dir, nodes.size FROM paths
    INNER JOIN ocfl_index_nodes nodes ON paths.id = nodes.id
WHERE paths.path = ?3 LIMIT 1;