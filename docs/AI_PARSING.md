# FINNRI AI Parsing

## Behaviour Rules
- AI proposes, user confirms.
- Never save without explicit user action.
- Do not hallucinate merchant, account, or category.
- Make uncertainty visible.
- Store source text/transcript for audit and future reprocessing.
- Avoid permanent voice recording storage by default.

## Provider Abstraction
Implement an internal parser interface so OpenAI, Claude, local models, or rule engines can be swapped without changing transaction persistence.

## Output Contract
amount, currency, type, merchant, category, account_hint, date, note, tags, recurring_candidate, split_candidate, confidence, missing_fields.
