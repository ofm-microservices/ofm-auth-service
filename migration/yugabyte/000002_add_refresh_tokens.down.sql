ALTER TABLE auth_credentials
DROP COLUMN IF EXISTS status;

ALTER TABLE auth_credentials
DROP COLUMN IF EXISTS username;

DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP TABLE IF EXISTS refresh_tokens;
