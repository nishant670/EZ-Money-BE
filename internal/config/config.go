package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	AllowOrigins       string
	AuthBearer         string
	TZDefault          string
	OpenAIKey          string
	OpenAIBaseURL      string
	OpenAILlmModel     string
	OpenAIWhisper      string
	OpenAIMaxTokens    int
	ReqTimeoutSec      int
	RateLimitRPS       float64
	RateLimitBurst     int
	MaxJSONKB          int64
	MaxUploadMB        int64
	MaxTranscriptChars int
	OTPDebugResponse   bool
	OTPExpiresMinutes  int
	ClaimTokenMinutes  int
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoi(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func atof(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func atob(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return def
}

func Load() *Config {
	return &Config{
		Port:               getenv("PORT", "8080"),
		AllowOrigins:       getenv("ALLOW_ORIGINS", ""),
		AuthBearer:         getenv("AUTH_BEARER", ""),
		TZDefault:          getenv("TZ_DEFAULT", "Asia/Kolkata"),
		OpenAIKey:          getenv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:      getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAILlmModel:     getenv("OPENAI_LLM_MODEL", "gpt-4o-mini"),
		OpenAIWhisper:      getenv("OPENAI_WHISPER_MODEL", "gpt-4o-mini-transcribe"),
		OpenAIMaxTokens:    atoi("OPENAI_MAX_OUTPUT_TOKENS", 600),
		ReqTimeoutSec:      atoi("REQUEST_TIMEOUT_SECONDS", 30),
		RateLimitRPS:       atof("RATE_LIMIT_RPS", 5),
		RateLimitBurst:     atoi("RATE_LIMIT_BURST", 10),
		MaxJSONKB:          int64(atoi("MAX_JSON_KB", 64)),
		MaxUploadMB:        int64(atoi("MAX_UPLOAD_MB", 15)),
		MaxTranscriptChars: atoi("MAX_TRANSCRIPT_CHARS", 1000),
		OTPDebugResponse:   atob("OTP_DEBUG_RESPONSE", false),
		OTPExpiresMinutes:  atoi("OTP_EXPIRES_MINUTES", 10),
		ClaimTokenMinutes:  atoi("CLAIM_TOKEN_EXPIRES_MINUTES", 15),
	}
}
