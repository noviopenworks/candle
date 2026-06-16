package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS repos (
  id    INTEGER PRIMARY KEY,
  org   TEXT NOT NULL,
  name  TEXT NOT NULL,
  UNIQUE(org, name)
);
CREATE TABLE IF NOT EXISTS indexes (
  id          INTEGER PRIMARY KEY,
  repo_id     INTEGER NOT NULL REFERENCES repos(id),
  commit_sha  TEXT,
  branch      TEXT,
  graph_path  TEXT NOT NULL,
  ingested_at TEXT NOT NULL,
  UNIQUE(repo_id, commit_sha)
);
CREATE TABLE IF NOT EXISTS nodes (
  index_id        INTEGER NOT NULL REFERENCES indexes(id),
  node_id         TEXT NOT NULL,
  label           TEXT,
  file_type       TEXT,
  source_file     TEXT,
  source_location TEXT,
  source_url      TEXT,
  captured_at     TEXT,
  author          TEXT,
  contributor     TEXT,
  PRIMARY KEY (index_id, node_id)
);
CREATE TABLE IF NOT EXISTS edges (
  index_id         INTEGER NOT NULL REFERENCES indexes(id),
  source           TEXT NOT NULL,
  target           TEXT NOT NULL,
  relation         TEXT NOT NULL,
  confidence       TEXT,
  confidence_score REAL,
  weight           REAL,
  source_file      TEXT
);
CREATE TABLE IF NOT EXISTS hyperedges (
  index_id         INTEGER NOT NULL REFERENCES indexes(id),
  hyperedge_id     TEXT NOT NULL,
  label            TEXT,
  relation         TEXT,
  confidence       TEXT,
  confidence_score REAL,
  source_file      TEXT,
  PRIMARY KEY (index_id, hyperedge_id)
);
CREATE TABLE IF NOT EXISTS hyperedge_members (
  index_id     INTEGER NOT NULL,
  hyperedge_id TEXT NOT NULL,
  node_id      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_nodes_label    ON nodes(index_id, label);
CREATE INDEX IF NOT EXISTS idx_nodes_ftype    ON nodes(index_id, file_type);
CREATE INDEX IF NOT EXISTS idx_nodes_file     ON nodes(index_id, source_file);
CREATE INDEX IF NOT EXISTS idx_edges_source   ON edges(index_id, source);
CREATE INDEX IF NOT EXISTS idx_edges_target   ON edges(index_id, target);
CREATE INDEX IF NOT EXISTS idx_edges_relation ON edges(index_id, relation);
CREATE TABLE IF NOT EXISTS api_specs (
  id        INTEGER PRIMARY KEY,
  index_id  INTEGER NOT NULL REFERENCES indexes(id),
  kind      TEXT NOT NULL,
  name      TEXT,
  version   TEXT,
  path      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS http_operations (
  id              INTEGER PRIMARY KEY,
  api_spec_id     INTEGER NOT NULL REFERENCES api_specs(id),
  method          TEXT NOT NULL,
  path            TEXT NOT NULL,
  operation_id    TEXT,
  summary         TEXT,
  request_schema  TEXT,
  response_schema TEXT,
  security        TEXT,
  tags            TEXT
);
CREATE TABLE IF NOT EXISTS api_schemas (
  id          INTEGER PRIMARY KEY,
  api_spec_id INTEGER NOT NULL REFERENCES api_specs(id),
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL,
  raw_ref     TEXT
);
CREATE INDEX IF NOT EXISTS idx_http_ops_spec    ON http_operations(api_spec_id);
CREATE INDEX IF NOT EXISTS idx_http_ops_opid    ON http_operations(operation_id);
CREATE INDEX IF NOT EXISTS idx_api_schemas_spec ON api_schemas(api_spec_id);
CREATE INDEX IF NOT EXISTS idx_api_specs_index  ON api_specs(index_id);
CREATE TABLE IF NOT EXISTS proto_files (
  id         INTEGER PRIMARY KEY,
  index_id   INTEGER NOT NULL REFERENCES indexes(id),
  path       TEXT NOT NULL,
  package    TEXT,
  go_package TEXT,
  imports    TEXT
);
CREATE TABLE IF NOT EXISTS proto_services (
  id            INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name          TEXT NOT NULL,
  full_name     TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS proto_rpcs (
  id               INTEGER PRIMARY KEY,
  proto_service_id INTEGER NOT NULL REFERENCES proto_services(id),
  name             TEXT NOT NULL,
  full_name        TEXT NOT NULL,
  request_message  TEXT,
  response_message TEXT,
  stream_kind      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS proto_messages (
  id            INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name          TEXT NOT NULL,
  full_name     TEXT NOT NULL,
  fields        TEXT
);
CREATE TABLE IF NOT EXISTS proto_enums (
  id            INTEGER PRIMARY KEY,
  proto_file_id INTEGER NOT NULL REFERENCES proto_files(id),
  name          TEXT NOT NULL,
  full_name     TEXT NOT NULL,
  "values"      TEXT
);
CREATE TABLE IF NOT EXISTS proto_rpc_impls (
  id           INTEGER PRIMARY KEY,
  proto_rpc_id INTEGER NOT NULL REFERENCES proto_rpcs(id),
  node_id      TEXT NOT NULL,
  confidence   REAL NOT NULL,
  match_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_proto_files_index ON proto_files(index_id);
CREATE INDEX IF NOT EXISTS idx_proto_services_file ON proto_services(proto_file_id);
CREATE INDEX IF NOT EXISTS idx_proto_rpcs_service ON proto_rpcs(proto_service_id);
CREATE INDEX IF NOT EXISTS idx_proto_messages_file ON proto_messages(proto_file_id);
CREATE INDEX IF NOT EXISTS idx_proto_enums_file ON proto_enums(proto_file_id);
CREATE INDEX IF NOT EXISTS idx_proto_rpc_impls_rpc ON proto_rpc_impls(proto_rpc_id);
CREATE TABLE IF NOT EXISTS dependencies (
  id          INTEGER PRIMARY KEY,
  index_id    INTEGER NOT NULL REFERENCES indexes(id),
  module_path TEXT NOT NULL,
  version     TEXT,
  ecosystem   TEXT NOT NULL,
  is_private  INTEGER NOT NULL,
  direct      INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS private_libraries (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  module_path  TEXT NOT NULL,
  readme       TEXT,
  doc_synopsis TEXT
);
CREATE TABLE IF NOT EXISTS private_library_exports (
  id                 INTEGER PRIMARY KEY,
  private_library_id INTEGER NOT NULL REFERENCES private_libraries(id),
  package_path       TEXT NOT NULL,
  symbol             TEXT NOT NULL,
  kind               TEXT NOT NULL,
  doc                TEXT,
  node_id            TEXT
);
CREATE TABLE IF NOT EXISTS private_library_usages (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  module_path  TEXT NOT NULL,
  version      TEXT,
  package_path TEXT NOT NULL,
  symbol       TEXT,
  file         TEXT,
  line         INTEGER
);
CREATE INDEX IF NOT EXISTS idx_dependencies_index ON dependencies(index_id);
CREATE INDEX IF NOT EXISTS idx_private_libs_index ON private_libraries(index_id);
CREATE INDEX IF NOT EXISTS idx_private_libs_module ON private_libraries(module_path);
CREATE INDEX IF NOT EXISTS idx_private_exports_lib ON private_library_exports(private_library_id);
CREATE INDEX IF NOT EXISTS idx_private_usages_index ON private_library_usages(index_id);
CREATE INDEX IF NOT EXISTS idx_private_usages_module ON private_library_usages(index_id, module_path);
`
