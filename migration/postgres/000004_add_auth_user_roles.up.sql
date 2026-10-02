CREATE TABLE IF NOT EXISTS auth_user_roles (
    user_id UUID NOT NULL REFERENCES auth_credentials(user_id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role)
);

CREATE INDEX IF NOT EXISTS idx_auth_user_roles_role ON auth_user_roles(role);
