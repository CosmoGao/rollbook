CREATE TABLE pending_imports_retained (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL CHECK(kind IN ('catalog','full','merge')),
 yaml BLOB NOT NULL, proposer TEXT NOT NULL, created_at TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','approved','rejected','expired')),
 actor_json TEXT NOT NULL DEFAULT '{}', reviewed_by TEXT, reviewed_at TEXT
);
INSERT INTO pending_imports_retained(id,kind,yaml,proposer,created_at,state,actor_json)
 SELECT id,kind,CASE WHEN state='pending' THEN yaml ELSE x'' END,proposer,created_at,state,actor_json FROM pending_imports;
DROP TABLE pending_imports;
ALTER TABLE pending_imports_retained RENAME TO pending_imports;
CREATE INDEX pending_imports_state_created ON pending_imports(state,created_at);
