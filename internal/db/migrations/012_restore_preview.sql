ALTER TABLE pending_imports ADD COLUMN preview_json TEXT NOT NULL DEFAULT '';
ALTER TABLE pending_imports ADD COLUMN preview_hash TEXT NOT NULL DEFAULT '';
