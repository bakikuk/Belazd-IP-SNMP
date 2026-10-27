package db

const schemaV1 = `
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);
INSERT OR IGNORE INTO schema_version (version) VALUES (1);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  permissions_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  username TEXT PRIMARY KEY,
  group_id TEXT NOT NULL REFERENCES user_groups(id),
  password_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS queues (
  idx INTEGER PRIMARY KEY,
  target TEXT NOT NULL,
  label TEXT NOT NULL DEFAULT ''
);
`
