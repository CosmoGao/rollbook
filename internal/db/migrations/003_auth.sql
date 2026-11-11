CREATE TABLE users (
 id TEXT PRIMARY KEY, login TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)), created_at TEXT NOT NULL
);
CREATE TABLE api_keys (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash BLOB NOT NULL UNIQUE,
 level TEXT NOT NULL CHECK(level IN ('read','propose','write')),
 issuer_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 created_at TEXT NOT NULL, expires_at TEXT, note TEXT,
 last_used_at TEXT, revoked_at TEXT
);
CREATE INDEX api_keys_issuer ON api_keys(issuer_user_id);
CREATE TABLE sessions (
 id_hash BLOB PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at TEXT NOT NULL, csrf_hash BLOB NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
