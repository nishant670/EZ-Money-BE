package config

import "testing"

func TestLoadDoesNotDefaultCORSWildcard(t *testing.T) {
	t.Setenv("ALLOW_ORIGINS", "")

	cfg := Load()
	if cfg.AllowOrigins == "*" {
		t.Fatal("ALLOW_ORIGINS must not default to wildcard")
	}
	if cfg.AllowOrigins != "" {
		t.Fatalf("expected empty default ALLOW_ORIGINS, got %q", cfg.AllowOrigins)
	}
}
