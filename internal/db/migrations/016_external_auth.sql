CREATE TABLE users_new (
 id TEXT PRIMARY KEY, login TEXT NOT NULL UNIQUE, password_hash TEXT,
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)), created_at TEXT NOT NULL,
 password_reset_at TEXT
);
INSERT INTO users_new SELECT id,login,password_hash,enabled,created_at,password_reset_at FROM users;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
CREATE UNIQUE INDEX users_login_folded ON users(lower(login));

CREATE TABLE identity_bindings (
 provider TEXT NOT NULL CHECK(provider IN ('ldap','oidc','feishu')),
 instance TEXT NOT NULL, subject TEXT NOT NULL,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 allowed INTEGER NOT NULL DEFAULT 1 CHECK(allowed IN (0,1)),
 verified_at TEXT,
 PRIMARY KEY(provider,instance,subject)
);
CREATE INDEX identity_bindings_user ON identity_bindings(user_id);
ALTER TABLE sessions ADD COLUMN provider TEXT NOT NULL DEFAULT 'local';
ALTER TABLE sessions ADD COLUMN instance TEXT;
ALTER TABLE sessions ADD COLUMN subject TEXT;
CREATE TABLE auth_settings (name TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO auth_settings(name,value) VALUES('local_login_enabled','1');
CREATE TABLE auth_audit (
 id INTEGER PRIMARY KEY AUTOINCREMENT, occurred_at TEXT NOT NULL,
 action TEXT NOT NULL, target TEXT NOT NULL
);
CREATE TABLE auth_pending (
 state_hash BLOB PRIMARY KEY, provider TEXT NOT NULL,
 nonce TEXT, verifier TEXT, expires_at TEXT NOT NULL
);
