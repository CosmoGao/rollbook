ALTER TABLE users ADD COLUMN email TEXT;
CREATE UNIQUE INDEX users_email_unique ON users(lower(trim(email))) WHERE email IS NOT NULL AND trim(email) <> '';
ALTER TABLE auth_pending ADD COLUMN purpose TEXT NOT NULL DEFAULT 'login';
ALTER TABLE auth_pending ADD COLUMN user_id TEXT;
ALTER TABLE auth_pending ADD COLUMN session_hash BLOB;
