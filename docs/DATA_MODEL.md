# FINNRI Data Model

## Entities
### User
id, guest_id, phone/email optional, display_name, locale, currency, created_at

### Account
id, user_id, name, type, institution_name, last_four_optional,
credit_limit fixed-point money, balance fixed-point money, is_default,
timestamps

Account `type` uses lowercase singular API values: `cash`, `upi`, `bank`,
`credit_card`, `debit_card`, `wallet`, or `other`. Legacy aliases `credit`,
`debit`, and `wallets` are normalized to the canonical values before storage.

### Category
Deferred as a separate table for MVP. Entries keep a required `category` string
so AI capture, manual entry, filters, dashboard rollups, and mobile forms remain
simple while category names are still changing.

Future normalized shape: id, user_id nullable, name, type, icon, color,
is_system, timestamps.

Migration plan:
- Create `categories` with seeded system rows and optional user rows.
- Backfill distinct `(user_id, lower(category), type)` values from entries.
- Add nullable `entries.category_id` while continuing to expose `category`.
- Dual-read by preferring `category_id` when present and falling back to the
  string column.
- Dual-write both fields for one client release, then make `category_id`
  required once old clients are no longer supported.

### Transaction
id, user_id, account_id required for new writes, amount, type, category string,
merchant, date, note, tags, source, optional source_text retained only after
user confirmation, ai_confidence, timestamps

Database constraints enforce positive transaction amounts, supported transaction
types (`expense`, `income`), supported sources (`manual`, `text`, `voice`), and
owned account references through a composite foreign key from
`entries(user_id, account_id)` to `accounts(user_id, id)`.

### ParseAttempt
Deferred for MVP to minimize sensitive raw financial text storage. Do not persist parse attempts, transcripts, provider prompts, or raw provider responses until a short retention window, access controls, and deletion job are defined.

### Insight
id, user_id, type, title, body, severity, related_entity_id, dismissed_at, created_at

### RecurringCandidate
Computed dashboard payload for now, not persisted: label, merchant, category,
average_amount, interval_guess, confidence, occurrences, last_seen_date,
next_expected_date, review_due. Persist only if dismissal/history or a fuller
subscription review workflow is added later.

### SplitFriend
id, user_id, name, email, phone, archived, timestamps

### SplitBill
id, user_id, entry_id optional, title, total_amount fixed-point money,
currency, date, notes, timestamps

### SplitParticipant
id, user_id, bill_id, friend_id, share_amount fixed-point money, direction,
timestamps. Direction is `friend_owes_user` or `user_owes_friend`.

### SplitSettlement
id, user_id, friend_id, amount fixed-point money, direction, date, notes,
timestamps. Direction is `friend_paid_user` or `user_paid_friend`.

Friend balances are computed from participant shares minus settlements. Positive
net balance means the friend owes the user; negative net balance means the user
owes the friend.
