CREATE TABLE host_offline_operations (
 id TEXT PRIMARY KEY,
 host_id TEXT NOT NULL,
 snapshot_json TEXT NOT NULL CHECK(json_valid(snapshot_json)),
 created_at TEXT NOT NULL,
 restored_at TEXT
);
