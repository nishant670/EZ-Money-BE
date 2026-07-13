CREATE TABLE IF NOT EXISTS split_groups (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS split_group_members (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES split_groups(id) ON DELETE CASCADE,
    friend_id BIGINT NOT NULL REFERENCES split_friends(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE split_bills
    ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES split_groups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_split_groups_user_archived
    ON split_groups (user_id, archived, name);

CREATE UNIQUE INDEX IF NOT EXISTS idx_split_group_members_unique
    ON split_group_members (user_id, group_id, friend_id);
