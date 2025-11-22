PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS blobs (
  id TEXT PRIMARY KEY,
  cipher BLOB NOT NULL,
  nonce BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  version INTEGER NOT NULL,
  deleted INTEGER NOT NULL DEFAULT 0,
  file_name TEXT,
  blob_id TEXT REFERENCES blobs(id),
  login_cipher BLOB,
  login_nonce BLOB,
  password_cipher BLOB,
  password_nonce BLOB,
  text_cipher BLOB,
  text_nonce BLOB,
  card_cipher BLOB,
  card_nonce BLOB
);

CREATE INDEX IF NOT EXISTS idx_items_deleted_updated_at ON items(deleted, updated_at);
CREATE INDEX IF NOT EXISTS idx_items_name ON items(name);
