package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadDoesNotDefaultCORSWildcard(t *testing.T) {
	t.Setenv("ALLOW_ORIGINS", "")

	cfg := Load()
	if cfg.AllowOrigins == "*" {
		t.Fatal("ALLOW_ORIGINS must not default to wildcard")
	}
	if cfg.AllowOrigins != "" {
		t.Fatalf("expected empty default ALLOW_ORIGINS, got %q", cfg.AllowOrigins)
	}
	if cfg.MaxJSONKB != 64 {
		t.Fatalf("expected default MAX_JSON_KB 64, got %d", cfg.MaxJSONKB)
	}
}

func TestEnvExampleContainsRequiredRedactedVariables(t *testing.T) {
	contents, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	values := parseEnvExample(string(contents))

	required := []string{
		"PORT",
		"TZ_DEFAULT",
		"ALLOW_ORIGINS",
		"OPENAI_API_KEY",
		"OPENAI_BASE_URL",
		"OPENAI_LLM_MODEL",
		"OPENAI_WHISPER_MODEL",
		"OPENAI_MAX_OUTPUT_TOKENS",
		"REQUEST_TIMEOUT_SECONDS",
		"RATE_LIMIT_RPS",
		"RATE_LIMIT_BURST",
		"MAX_JSON_KB",
		"MAX_UPLOAD_MB",
		"MAX_TRANSCRIPT_CHARS",
		"OTP_DEBUG_RESPONSE",
		"OTP_DEV_CODE",
		"OTP_EXPIRES_MINUTES",
		"CLAIM_TOKEN_EXPIRES_MINUTES",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSLMODE",
		"DATABASE_URL",
	}

	for _, key := range required {
		if _, ok := values[key]; !ok {
			t.Fatalf(".env.example missing %s", key)
		}
	}
	for _, secretKey := range []string{"OPENAI_API_KEY", "DB_PASSWORD", "DATABASE_URL"} {
		if strings.TrimSpace(values[secretKey]) != "" {
			t.Fatalf("%s must be redacted in .env.example", secretKey)
		}
	}
}

func parseEnvExample(contents string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}
