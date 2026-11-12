CREATE TABLE login_limits (
 kind TEXT NOT NULL, subject BLOB NOT NULL, failures INTEGER NOT NULL,
 blocked_until TEXT NOT NULL, updated_at TEXT NOT NULL,
 PRIMARY KEY(kind, subject)
);
CREATE INDEX login_limits_updated ON login_limits(updated_at);
