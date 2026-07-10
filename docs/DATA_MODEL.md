# FINNRI Data Model

## Entities
### User
id, guest_id, phone/email optional, display_name, locale, currency, created_at

### Account
id, user_id, name, type, institution_name, last_four_optional, is_default, timestamps

### Category
id, user_id nullable, name, type, icon, color, is_system

### Transaction
id, user_id, account_id required for new writes, amount, type, category_id, merchant, date, note, tags, source, source_text, ai_confidence, timestamps

### ParseAttempt
id, user_id, input_type, source_text, provider, raw_response, normalized_result, confidence, created_at

### Insight
id, user_id, type, title, body, severity, related_entity_id, dismissed_at, created_at

### RecurringCandidate
id, user_id, merchant, amount_pattern, interval_guess, confidence, status, created_at

### SplitMetadata
id, transaction_id, participant_name, total_amount, user_share, receivable_amount, status, note
