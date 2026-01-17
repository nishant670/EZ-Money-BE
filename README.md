
# Finance Parser API (Go 1.21 + Gin)

Accepts **audio** (or `hint_text`), transcribes with Whisper, parses with an LLM using a confirm-first policy, validates against JSON Schema, and returns structured JSON.

## Quick Start

```bash
go version
go mod tidy
cp .env.example .env
# edit .env and add: OPENAI_API_KEY=sk-...
go run ./cmd/server
```

If `cp .env.example .env` doesn't work, create it manually (contents below).

## Test
```bash
curl -X POST http://localhost:8080/v1/parse   -H "Authorization: Bearer test"   -F "hint_text=I spent 500 rupees today with my Amex card for my wife's birthday gift"
```

## FAQ
- **What does `cp .env.example .env` do?** Copies the template env file so you can edit secrets.
- **Which LLM model is used?** Defaults to `gpt-4o-mini`; override via `OPENAI_LLM_MODEL` in `.env` if you have access to a different model.
- **How to put the OpenAI key?** Open `.env` and set `OPENAI_API_KEY=sk-...` (no quotes).
- **See current key in shell?** `echo $OPENAI_API_KEY` (only works if previously exported in your shell).
