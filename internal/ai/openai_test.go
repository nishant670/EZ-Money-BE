package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"finance-parser-go/internal/config"
)

func TestParseTextUsesConfiguredCostControls(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"{\"stage\":\"draft\"}"}}],
			"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120}
		}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(&config.Config{
		OpenAIKey:       "test-key",
		OpenAIBaseURL:   server.URL,
		OpenAILlmModel:  "gpt-4o-mini",
		OpenAIMaxTokens: 600,
	})

	if _, err := client.ParseText(context.Background(), "coffee 200 via upi", "Asia/Kolkata"); err != nil {
		t.Fatal(err)
	}
	if got := requestBody["model"]; got != "gpt-4o-mini" {
		t.Fatalf("model = %v", got)
	}
	if got := requestBody["max_completion_tokens"]; got != float64(600) {
		t.Fatalf("max_completion_tokens = %v", got)
	}
}

func TestTranscribeUsesConfiguredModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("model"); got != "gpt-4o-mini-transcribe" {
			t.Fatalf("model = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"coffee 200 via upi"}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(&config.Config{
		OpenAIKey:     "test-key",
		OpenAIBaseURL: server.URL,
		OpenAIWhisper: "gpt-4o-mini-transcribe",
	})

	got, err := client.Transcribe(context.Background(), "recording.m4a", []byte("audio"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "coffee 200 via upi" {
		t.Fatalf("transcript = %q", got)
	}
}
