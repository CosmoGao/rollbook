CREATE TABLE pending_imports (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL CHECK(kind IN ('catalog','full')),
 yaml BLOB NOT NULL, proposer TEXT NOT NULL, created_at TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','rejected'))
);
