-- Remove self-action transaction notifications.
--
-- The API used to write a notification every time a user created, updated, or
-- deleted an entry — describing an action the user had just performed and
-- watched succeed. A two-day-old account with 97 entries carried 113 unread
-- notifications, all of them receipts, which buried the alerts that actually
-- matter (budget thresholds, subscription renewals, autopay drafts).
--
-- The handlers no longer emit these, so this clears the historical rows.
-- Safe to re-run: after the code change nothing recreates them.
--
-- Deliberately NOT deleted:
--   subscription.autopay  -- the app added an entry on the user's behalf
--   budget.*              -- threshold crossings the user could not otherwise know
--   split.*               -- actions taken by other people

DELETE FROM notifications
WHERE type IN (
  'transaction.created',
  'transaction.updated',
  'transaction.deleted'
);
