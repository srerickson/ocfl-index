PRAGMA foreign_keys = 1;

create table ocfl_index_schema (
    major INTEGER NOT NULL,
    minor INTEGER NOT NULL,
    PRIMARY KEY (major, minor)
);
-- only one row
INSERT INTO ocfl_index_schema (major, minor) values (0,2);

-- only support one storage root per index.
create table ocfl_index_storage_root (
  id INTEGER PRIMARY KEY,
  root_path TEXT NOT NULL, -- TODO: remove b/c typical value is "."
  description TEXT NOT NULL, -- storage root description
  spec TEXT NOT NULL, -- storage root's OCFL spec version
  indexed_at DATETIME, -- date of index
  --validated_at DATETIME NOT NULL DEFAULT "", -- date of last validation
  UNIQUE(root_path)
);
-- only one storage root per database for now
INSERT INTO ocfl_index_storage_root (id, root_path, description, spec) VALUES (1, "", "", "");


-- OCFL Objects
create table ocfl_index_objects (
    id INTEGER PRIMARY KEY, -- internal identifier
    ocfl_id TEXT NOT NULL, -- OCFL Object ID
    spec TEXT NOT NULL, -- OCFL specification version
    digest_algorithm TEXT NOT NULL, -- Inventory digest algorithm
    inventory_digest TEXT NOT NULL, -- Inventory checksum
    root_path TEXT NOT NULL, -- object path relative to storage root's path
    head TEXT NOT NULL, -- version number (e.g., 'v4')
    UNIQUE(ocfl_id),
    UNIQUE(root_path)
);


-- OCFL Object Versions
create table ocfl_index_object_versions (
    object_id INTEGER NOT NULL REFERENCES ocfl_index_objects(id),
    num INTEGER NOT NULL, -- version num (1,2,3): CAST(LTRIM(name,'v') AS INT));
    name TEXT NOT NULL, -- version string (e.g. 'v4')
    message TEXT NOT NULL, -- 'message' field from inventory
    created DATETIME NOT NULL, -- 'created' field from inventory
    user_name TEXT NOT NULL, -- user 'name' field from inventory
    user_address TEXT NOT NULL, -- user 'address' field from inventory
    node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id), -- root node for the version
    PRIMARY KEY(object_id, num),
    UNIQUE(object_id, name)
);

-- A node represents some unique content, identified by a checksum and a
-- file/directory status. If the node is a file, the checksum corresponds to the
-- digest from an inventory. If the node is a directory, the checksum correspond
-- to a recursive digest of the of the directory's contents. Multiple
-- 'ocfl_index_names' can refer to the same node.
create table ocfl_index_nodes (
  id INTEGER PRIMARY KEY, -- internal id
  dir boolean NOT NULL, -- node is a directory, not a file
  sum BLOB NOT NULL, -- digest (raw bytes)
  size INTEGER NOT NULL DEFAULT 0,
  UNIQUE(sum, dir)
);

-- A name is represents a logical path element (file or directory). They are
-- also 'edges' between parent nodes and child nodes. 
create table ocfl_index_names (
  node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  name TEXT NOT NULL,
  parent_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  UNIQUE(node_id, name, parent_id),
  PRIMARY KEY(parent_id, name)
);

-- A content path is a reference to a specific stored file. Node entries that
-- are files should have corresponding content_paths. Content paths are scoped
-- to an ocfl object.
CREATE TABLE ocfl_index_content_paths (
  object_id INTEGER NOT NULL REFERENCES ocfl_index_objects(id),
  node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  file_path TEXT NOT NULL, -- path relative to the object path
  PRIMARY KEY(object_id, node_id)
);