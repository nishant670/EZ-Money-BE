CREATE TABLE IF NOT EXISTS split_group_invites (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES split_groups(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_group_invites_status_check CHECK (status IN ('active', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_split_group_invites_group_active
    ON split_group_invites (user_id, group_id, status, created_at DESC);
