# FINNRI Code Review Checklist

## Product
- Does this support the MVP habit loop?
- Is the feature in MVP scope?
- Does the user remain in control of financial data?

## AI Safety
- Is AI output treated as draft only?
- Are uncertain fields visible?
- Are hallucinations avoided?

## Backend
- Server-side validation exists.
- Ownership checks exist.
- Secrets are not committed.
- Migrations are explicit.

## Frontend
- Confirmation is mandatory before save.
- Loading/error states are handled.
- Forms are editable and accessible.

## Testing
- Parser normalization tests.
- Transaction validation tests.
- Account ownership tests.
- Dashboard calculation tests.
