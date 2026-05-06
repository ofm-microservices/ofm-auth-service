CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_credentials(user_id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

ALTER TABLE auth_credentials
ADD COLUMN IF NOT EXISTS username TEXT;

UPDATE auth_credentials
SET username = ''
WHERE username IS NULL;

ALTER TABLE auth_credentials
ALTER COLUMN username SET NOT NULL;

ALTER TABLE auth_credentials
ADD COLUMN IF NOT EXISTS status TEXT;

UPDATE auth_credentials
SET status = CASE
    WHEN email_verified THEN 'email_verified'
    ELSE 'pending_registration'
END
WHERE status IS NULL;

ALTER TABLE auth_credentials
ALTER COLUMN status SET NOT NULL;

ALTER TABLE auth_credentials
ALTER COLUMN status SET DEFAULT 'pending_registration';
