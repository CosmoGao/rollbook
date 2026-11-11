CREATE TABLE version_checks (
 service_id TEXT PRIMARY KEY REFERENCES services(id) ON DELETE CASCADE,
 last_attempt_at TEXT NOT NULL,
 last_error TEXT
);
CREATE TABLE version_check_lease (
 id INTEGER PRIMARY KEY CHECK(id=1),
 expires_at TEXT NOT NULL
);
