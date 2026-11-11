CREATE TABLE pending_imports_new (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL CHECK(kind IN ('catalog','full','merge')),
 yaml BLOB NOT NULL, proposer TEXT NOT NULL, created_at TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','rejected')),
 actor_json TEXT NOT NULL DEFAULT '{}'
);
INSERT INTO pending_imports_new(id,kind,yaml,proposer,created_at,state) SELECT * FROM pending_imports;
DROP TABLE pending_imports;
ALTER TABLE pending_imports_new RENAME TO pending_imports;
