-- An entry may have no account.
--
-- Reverses 0002/0005/0012, which each asserted NOT NULL, and restores the rule
-- 0003 first set. Capture is never blocked on account setup: an unlinked entry
-- saves, shows an "Unlinked" affordance in the feed, and is returned as a
-- review item so the account can be attached later.
--
-- No backfill. Assigning the default account to unlinked rows is what the old
-- runtime schema did, and it silently destroys the state this change exists to
-- create.
ALTER TABLE entries
    ALTER COLUMN account_id DROP NOT NULL;
