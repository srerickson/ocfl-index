PRAGMA foreign_keys = 1;

create table ocfl_index_schema (
    major INTEGER NOT NULL,
    minor INTEGER NOT NULL,
    PRIMARY KEY (major, minor)
);
INSERT INTO ocfl_index_schema (major, minor) values (0,1);

create table ocfl_index_objects (
    id INTEGER PRIMARY KEY,
    uri TEXT NOT NULL,
    head TEXT NOT NULL,
    node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
    UNIQUE(uri)
);

create table ocfl_index_object_versions (
    object_id INTEGER NOT NULL REFERENCES ocfl_index_objects(id),
    num INTEGER NOT NULL,
    name TEXT NOT NULL,
    message TEXT NOT NULL,
    created DATETIME NOT NULL,
    user_name TEXT NOT NULL,
    user_address TEXT NOT NULL,
    PRIMARY KEY(object_id, num),
    UNIQUE(object_id, name)
);

create table ocfl_index_nodes (
  id INTEGER PRIMARY KEY,
  dir boolean NOT NULL,
  sum BLOB NOT NULL,
  UNIQUE(sum, dir)
);

create table ocfl_index_names (
  node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  name TEXT NOT NULL,
  parent_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  UNIQUE(node_id, name, parent_id),
  PRIMARY KEY(parent_id, name)
);

CREATE TABLE ocfl_index_content_paths (
  object_id INTEGER NOT NULL REFERENCES ocfl_index_objects(id),
  node_id INTEGER NOT NULL REFERENCES ocfl_index_nodes(id),
  file_path TEXT NOT NULL,
  PRIMARY KEY(object_id, node_id)
);