
# Finance Parser API (Go 1.21 + Gin)

Accepts **audio** (or `hint_text`), transcribes with Whisper, parses with an LLM using a confirm-first policy, validates against JSON Schema, and returns structured JSON.

## Quick Start

```bash
go version
go mod tidy
cp .env.example .env
# edit .env and add: OPENAI_API_KEY=<your project key>
go run ./cmd/server
```

If `cp .env.example .env` doesn't work, create it manually (contents below).

## Rotate the OpenAI key

The key is read only by the backend from `EZ-Money-BE/.env`.

1. Open `.env` and replace only the value after `OPENAI_API_KEY=`.
2. Do not add quotes or spaces, and never put the key in the mobile or web app.
3. If `OPENAI_API_KEY` was previously exported in your shell, run
   `unset OPENAI_API_KEY`; exported variables take precedence over `.env`.
4. Restart the backend; environment variables are loaded only at startup.
5. Confirm that the key's OpenAI project has billing/credits and access to
   `gpt-4o-mini` and `gpt-4o-mini-transcribe`.

`.env` is ignored by Git. Commit `.env.example`, which contains configuration
names but no secrets.

## Development cost controls

- Speech-to-text defaults to `gpt-4o-mini-transcribe`.
- Transaction parsing uses `gpt-4o-mini`; it is a small, low-cost model suited
  to focused JSON extraction.
- AI output is capped at 600 tokens with `OPENAI_MAX_OUTPUT_TOKENS`.
- Input is capped at 1,000 characters with `MAX_TRANSCRIPT_CHARS`.
- Auth JSON bodies are capped with `MAX_JSON_KB`; parse/upload request bodies
  are capped with `MAX_UPLOAD_MB`.
- Each parse logs token counts, not prompt content or the API key.
- The API does not retry failed OpenAI calls, preventing duplicate charges.

## Database migration

Apply checked-in migrations in filename order before deploying the updated
server. Migrations `0002_lock_transaction_contract.sql` and
`0005_require_entry_account_id.sql` create a default Cash account where needed,
backfill legacy transactions, and make `account_id` mandatory. Migration `0002`
also converts amounts to `numeric(19,2)`.

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0001_add_entry_account_id.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0002_lock_transaction_contract.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0003_make_entry_account_optional.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0004_add_entry_idempotency.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0005_require_entry_account_id.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0006_create_auth_sessions.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/0007_create_auth_verifications.sql
```

## Auth verification

OTP codes are randomly generated and stored only as hashes. Verified OTPs issue
opaque, expiring, one-time `claim_token` values for registration or contact
updates; the token does not contain the email or phone number. For local manual
testing only, set `OTP_DEBUG_RESPONSE=true` to include `dev_otp` in the
`POST /v1/auth/otp/send` response.

## Test
```bash
curl -X POST http://localhost:8080/v1/parse   -H "Authorization: Bearer test"   -F "hint_text=I spent 500 rupees today with my Amex card for my wife's birthday gift"
```

## FAQ
- **What does `cp .env.example .env` do?** Copies the template env file so you can edit secrets.
- **Which models are used?** `gpt-4o-mini` parses transactions and
  `gpt-4o-mini-transcribe` transcribes voice. Both are configurable in `.env`.
- **How do I replace the OpenAI key?** Replace the `OPENAI_API_KEY` value in
  `.env`, then restart the backend.
