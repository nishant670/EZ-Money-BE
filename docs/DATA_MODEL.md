# FINNRI Data Model

## Entities
### User
id, guest_id, phone/email optional, display_name, locale, currency, created_at

### Account
id, user_id, name, type, institution_name, last_four_optional, is_default, timestamps

Account `type` uses lowercase singular API values: `cash`, `upi`, `bank`,
`credit_card`, `debit_card`, `wallet`, or `other`. Legacy aliases `credit`,
`debit`, and `wallets` are normalized to the canonical values before storage.

### Category
id, user_id nullable, name, type, icon, color, is_system

### Transaction
id, user_id, account_id required for new writes, amount, type, category_id, merchant, date, note, tags, source, optional source_text retained only after user confirmation, ai_confidence, timestamps

### ParseAttempt
Deferred for MVP to minimize sensitive raw financial text storage. Do not persist parse attempts, transcripts, provider prompts, or raw provider responses until a short retention window, access controls, and deletion job are defined.

### Insight
id, user_id, type, title, body, severity, related_entity_id, dismissed_at, created_at

### RecurringCandidate
id, user_id, merchant, amount_pattern, interval_guess, confidence, status, created_at

### SplitMetadata
id, transaction_id, participant_name, total_amount, user_share, receivable_amount, status, note
