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
`
