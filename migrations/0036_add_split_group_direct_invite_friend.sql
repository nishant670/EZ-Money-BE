-- An invite raised by adding a specific person to a group knows which friend
-- row it stands for. Without it, acceptance has to guess which of the owner's
-- friends the arriving user is, and a name-only row ("Wife") never matches an
-- account identified by email — leaving the expenses already recorded against
-- that row stranded on a duplicate.
ALTER TABLE split_group_direct_invites
    ADD COLUMN IF NOT EXISTS friend_id BIGINT REFERENCES split_friends(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_split_group_direct_invites_friend
    ON split_group_direct_invites (friend_id)
    WHERE friend_id IS NOT NULL;
