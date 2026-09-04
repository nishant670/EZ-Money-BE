-- Make the split ledger derivable from what the app can actually show, and
-- clean up the rows that were not.
--
-- Three things were true at once:
--
--   1. `split_participants` had no foreign key to `split_bills`, so a bill
--      deleted through any path that missed its participants left rows that
--      still counted toward `/v1/split/balances`.
--   2. `split_settlements` named a friend but never a group, so deleting a
--      group removed its bills and left the settlements that closed them —
--      each one still applying its full amount, now against nothing.
--   3. Nothing on the Splits screen draws either kind of leftover, so the
--      overall figure had no line item anywhere to explain it. An account with
--      every group deleted still read "Overall, you are owed ₹4,000".
--
-- The FK and the group column stop it recurring; the guarded block clears what
-- has already accumulated. `internal/database/schema.go` applies the same
-- statements at boot, keyed on the same `schema_repairs` row, so applying this
-- file by hand and letting the server do it are the same operation done once.

ALTER TABLE split_groups
    ADD COLUMN IF NOT EXISTS photo_url TEXT NOT NULL DEFAULT '';

ALTER TABLE split_settlements
    ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES split_groups(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_split_settlements_group
    ON split_settlements (group_id)
    WHERE group_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS split_friend_merges (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    from_friend_id BIGINT NOT NULL REFERENCES split_friends(id) ON DELETE CASCADE,
    to_friend_id BIGINT NOT NULL REFERENCES split_friends(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_split_friend_merges_target
    ON split_friend_merges (user_id, to_friend_id);

-- A row can only be merged away once. Stated as its own index rather than an
-- inline UNIQUE on the column: an inline one becomes a constraint named by
-- Postgres, GORM's AutoMigrate looks for a GORM-named one, and finding neither
-- it tries to drop the name it expected and fails the server's boot outright.
CREATE UNIQUE INDEX IF NOT EXISTS idx_split_friend_merges_source
    ON split_friend_merges (from_friend_id);

CREATE TABLE IF NOT EXISTS schema_repairs (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Participants whose bill is gone. Pure phantom: every screen builds its rows
-- from bills, so these were only ever visible inside the totals. Unguarded,
-- because it is a no-op the moment the constraint below exists.
DELETE FROM split_participants
WHERE NOT EXISTS (
    SELECT 1 FROM split_bills WHERE split_bills.id = split_participants.bill_id
);

ALTER TABLE split_participants
    DROP CONSTRAINT IF EXISTS fk_split_participants_bill;
ALTER TABLE split_participants
    ADD CONSTRAINT fk_split_participants_bill
    FOREIGN KEY (bill_id) REFERENCES split_bills(id) ON DELETE CASCADE;

-- Everything below rewrites or deletes rows that predate the fixes above, and
-- none of it should touch data written afterwards — in particular, deleting a
-- settlement for a friend with no expense history is right for a leftover and
-- wrong for a settlement somebody records before their first expense. Hence the
-- guard rather than a plain script.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM schema_repairs WHERE name = '0043_repair_split_ledger') THEN
        RETURN;
    END IF;

    -- Bills stranded on an archived or deleted group. Archiving deletes the
    -- owner's own bills, but a group shared with another member kept theirs,
    -- and an archived group is on no list they could reach it through.
    DELETE FROM split_participants
    WHERE bill_id IN (
        SELECT b.id FROM split_bills b
        LEFT JOIN split_groups g ON g.id = b.group_id
        WHERE b.group_id IS NOT NULL AND (g.id IS NULL OR g.archived)
    );

    DELETE FROM split_bills b
    USING split_groups g
    WHERE b.group_id = g.id AND g.archived;

    DELETE FROM split_bills
    WHERE group_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM split_groups g WHERE g.id = split_bills.group_id);

    -- Settlements for a friend who has no expense history left at all. A
    -- settlement only ever means "this much of what we owed each other is now
    -- paid", so with nothing on either side of the ledger it cannot be
    -- describing a debt — it can only be inventing one. Settlements for friends
    -- who still have participants are left exactly as they are.
    DELETE FROM split_settlements s
    WHERE s.group_id IS NULL
      AND NOT EXISTS (
          SELECT 1 FROM split_participants p
          WHERE p.user_id = s.user_id AND p.friend_id = s.friend_id
      );

    -- A legacy settlement whose friend shares exactly one group with the user
    -- belongs to that group: there is no other ledger it could have settled.
    -- Ambiguous cases keep a NULL group and stay friend-level, which is the
    -- behaviour they already had.
    UPDATE split_settlements s
    SET group_id = single.group_id
    FROM (
        SELECT m.user_id, m.friend_id, MIN(m.group_id) AS group_id
        FROM split_group_members m
        JOIN split_groups g ON g.id = m.group_id AND NOT g.archived
        GROUP BY m.user_id, m.friend_id
        HAVING COUNT(DISTINCT m.group_id) = 1
    ) AS single
    WHERE s.group_id IS NULL
      AND s.user_id = single.user_id
      AND s.friend_id = single.friend_id;

    INSERT INTO schema_repairs (name) VALUES ('0043_repair_split_ledger');
END
$$;
