CREATE TABLE IF NOT EXISTS split_group_direct_invites (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES split_groups(id) ON DELETE CASCADE,
    invite_id BIGINT NOT NULL REFERENCES split_group_invites(id) ON DELETE CASCADE,
    target_email VARCHAR(254) NOT NULL DEFAULT '',
    target_phone VARCHAR(32) NOT NULL DEFAULT '',
    invited_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT split_group_direct_invites_target_check CHECK (target_email <> '' OR target_phone <> ''),
    CONSTRAINT split_group_direct_invites_status_check CHECK (status IN ('pending', 'accepted', 'revoked'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_split_group_direct_invites_email_unique
    ON split_group_direct_invites (group_id, LOWER(target_email))
    WHERE target_email <> '' AND status = 'pending';

CREATE UNIQUE INDEX IF NOT EXISTS idx_split_group_direct_invites_phone_unique
    ON split_group_direct_invites (group_id, target_phone)
    WHERE target_phone <> '' AND status = 'pending';

CREATE INDEX IF NOT EXISTS idx_split_group_direct_invites_invited_user
    ON split_group_direct_invites (invited_user_id, status, created_at DESC);
