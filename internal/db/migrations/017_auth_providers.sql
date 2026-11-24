CREATE TABLE auth_provider_settings (
 kind TEXT PRIMARY KEY CHECK(kind IN ('ldap','oidc','feishu')),
 enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN (0,1)),
 config TEXT NOT NULL,
 secret TEXT NOT NULL DEFAULT ''
);
ALTER TABLE auth_pending ADD COLUMN config_version TEXT NOT NULL DEFAULT '';
