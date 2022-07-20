-- query to get node for a given path in an object
WITH RECURSIVE
    paths(id, path, sum, dir) AS (
        SELECT names.node_id, names.name, child.sum, child.dir
        FROM ocfl_index_names names
        INNER JOIN ocfl_index_objects objects ON names.parent_id = objects.node_id
        INNER JOIN ocfl_index_nodes child ON names.node_id = child.id
        WHERE objects.uri = 'example-zotero-library' and names.name = 'v1'
    UNION
        SELECT names.node_id, paths.path || "/" || names.name, nodes.sum, nodes.dir
        FROM ocfl_index_names names
        INNER JOIN paths ON names.parent_id = paths.id
        INNER JOIN ocfl_index_nodes nodes ON names.node_id = nodes.id
    )
SELECT path from paths where dir is false;