CREATE TABLE IF NOT EXISTS split_group_user_members (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES split_groups(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL DEFAULT 'member',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_group_user_members_role_check CHECK (role IN ('member')),
    CONSTRAINT split_group_user_members_status_check CHECK (status IN ('active', 'removed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_split_group_user_members_unique
    ON split_group_user_members (group_id, user_id);

CREATE INDEX IF NOT EXISTS idx_split_group_user_members_user_active
    ON split_group_user_members (user_id, status, group_id);
