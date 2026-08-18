-- A group's type and its default split are properties of the group, not of the
-- member reading it: only a trip has dates, and a couple splitting 60/40 splits
-- 60/40 for both of them. Both move onto split_groups so every member sees the
-- same answer.
ALTER TABLE split_groups
    ADD COLUMN IF NOT EXISTS kind VARCHAR(16) NOT NULL DEFAULT 'other',
    ADD COLUMN IF NOT EXISTS default_split TEXT;

ALTER TABLE split_groups
    DROP CONSTRAINT IF EXISTS split_groups_kind_check;

ALTER TABLE split_groups
    ADD CONSTRAINT split_groups_kind_check CHECK (kind IN ('trip', 'home', 'couple', 'other'));

-- A default split names people by the owner's friend ids, so a member reading
-- it needs to know which of those rows is themselves. Invite acceptance already
-- finds that row by email or phone; this records the answer.
ALTER TABLE split_friends
    ADD COLUMN IF NOT EXISTS linked_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_split_friends_linked_user
    ON split_friends (linked_user_id)
    WHERE linked_user_id IS NOT NULL;

UPDATE split_friends
SET linked_user_id = users.id
FROM users
WHERE split_friends.linked_user_id IS NULL
    AND (
        (split_friends.email <> '' AND LOWER(split_friends.email) = LOWER(users.email))
        OR (split_friends.phone <> '' AND split_friends.phone = users.phone)
    );
