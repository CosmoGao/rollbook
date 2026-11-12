ALTER TABLE versions ADD COLUMN raw_tag TEXT;
CREATE TABLE version_alert_settings (id INTEGER PRIMARY KEY CHECK(id=1), enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN (0,1)), webhook_url TEXT NOT NULL DEFAULT '', format TEXT NOT NULL DEFAULT 'feishu' CHECK(format IN ('feishu','wecom')));
INSERT INTO version_alert_settings(id) VALUES(1);
CREATE TABLE version_alert_delivery (day TEXT PRIMARY KEY, attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at TEXT, sent_at TEXT, last_error TEXT);
