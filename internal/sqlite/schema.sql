PRAGMA foreign_keys = 1;

create table ocfl_index_schema (
    major INTEGER NOT NULL,
    minor INTEGER NOT NULL,
    PRIMARY KEY (major, minor)
);
-- only one row
INSERT INTO ocfl_index_schema (major, minor) values (0,3);

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

-- OCFL Object Root Directories
create table ocfl_index_object_roots (
  id INTEGER PRIMARY KEY,
  path TEXT NOT NULL,
  indexed_at DATETIME NOT NULL,
  UNIQUE(path)
);

-- OCFL Object Inventories
create table ocfl_index_inventories (
    id INTEGER PRIMARY KEY, -- internal identifier
    root_id INTEGER NOT NULL references ocfl_index_object_roots(id),
    ocfl_id TEXT NOT NULL, -- OCFL Object ID
    spec TEXT NOT NULL, -- OCFL specification version
    digest_algorithm TEXT NOT NULL, -- Inventory digest algorithm
    inventory_digest TEXT NOT NULL, -- Inventory checksum
    head TEXT NOT NULL, -- version number (e.g., 'v4')
    indexed_at DATETIME NOT NULL, 
    UNIQUE(ocfl_id),
    UNIQUE(root_id)
);


-- OCFL Object Versions
create table ocfl_index_versions (
    inventory_id INTEGER NOT NULL REFERENCES ocfl_index_inventories(id),
    num INTEGER NOT NULL, -- version num (1,2,3): CAST(LTRIM(name,'v') AS INT));
    name TEXT NOT NULL, -- version string (e.g. 'v4')
    message TEXT NOT NULL, -- 'message' field from inventory
    created DATETIME NOT NULL, -- 'created' field from inventory
    user_name TEXT NOT NULL, -- user 'name' field from inventory
    user_address TEXT NOT NULL, -- user 'address' field from inventory
    node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id), -- root node for the version
    PRIMARY KEY(inventory_id, num),
    UNIQUE(inventory_id, name)
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
  size INTEGER, -- can be null!
  UNIQUE(sum, dir)
);

-- A name is represents a logical path element (file or directory). They are
-- also 'edges' between parent nodes and child nodes. 
create table ocfl_index_names (
  name TEXT NOT NULL, -- the path element name sould not include "/"
  node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id), -- the named node (child)
  parent_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id), -- the parent node
  PRIMARY KEY(parent_id, name)
);

-- A content path represents a manifest entry from an inventory. Node entries
-- that are files should have corresponding content_paths. Content paths are
-- scoped to an ocfl object.
CREATE TABLE ocfl_index_content_paths (
  inventory_id INTEGER NOT NULL REFERENCES ocfl_index_inventories(id),
  node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  file_path TEXT NOT NULL, -- path relative to the object path
  PRIMARY KEY(inventory_id, node_id)
);