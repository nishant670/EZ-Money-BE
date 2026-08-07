package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	Port                            string
	AllowOrigins                    string
	AuthBearer                      string
	TZDefault                       string
	OpenAIKey                       string
	OpenAIBaseURL                   string
	OpenAILlmModel                  string
	OpenAIWhisper                   string
	OpenAIMaxTokens                 int
	ReqTimeoutSec                   int
	RateLimitRPS                    float64
	RateLimitBurst                  int
	MaxJSONKB                       int64
	MaxUploadMB                     int64
	MaxTranscriptChars              int
	AIParseDisabled                 bool
	AIUnpaidMaxVoiceBytes           int64
	AIFailedParseCooldownThreshold  int
	AIFailedParseCooldownWindowMin  int
	AIFailedParseCooldownMinutes    int
	AIProviderFailureThreshold      int
	AIProviderCircuitBreakerSeconds int
	OTPDebugResponse                bool
	OTPDevCode                      string
	OTPExpiresMinutes               int
	ClaimTokenMinutes               int
	GoogleClientIDs                 []string
	AIDailyCostAlertUSDMicros       int64
	AIAbuseDailyCreditsThreshold    int
	AIFreeCostPerUserAlertUSDMicros int64
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

func atoi64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
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

func csv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func Load() *Config {
	return &Config{
		Port:                            getenv("PORT", "8080"),
		AllowOrigins:                    getenv("ALLOW_ORIGINS", ""),
		AuthBearer:                      getenv("AUTH_BEARER", ""),
		TZDefault:                       getenv("TZ_DEFAULT", "Asia/Kolkata"),
		OpenAIKey:                       getenv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:                   getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAILlmModel:                  getenv("OPENAI_LLM_MODEL", "gpt-4o-mini"),
		OpenAIWhisper:                   getenv("OPENAI_WHISPER_MODEL", "gpt-4o-mini-transcribe"),
		OpenAIMaxTokens:                 atoi("OPENAI_MAX_OUTPUT_TOKENS", 600),
		ReqTimeoutSec:                   atoi("REQUEST_TIMEOUT_SECONDS", 30),
		RateLimitRPS:                    atof("RATE_LIMIT_RPS", 5),
		RateLimitBurst:                  atoi("RATE_LIMIT_BURST", 10),
		MaxJSONKB:                       int64(atoi("MAX_JSON_KB", 64)),
		MaxUploadMB:                     int64(atoi("MAX_UPLOAD_MB", 15)),
		MaxTranscriptChars:              atoi("MAX_TRANSCRIPT_CHARS", 1000),
		AIParseDisabled:                 atob("AI_PARSE_DISABLED", false),
		AIUnpaidMaxVoiceBytes:           int64(atoi("AI_UNPAID_MAX_VOICE_BYTES", 512*1024)),
		AIFailedParseCooldownThreshold:  atoi("AI_FAILED_PARSE_COOLDOWN_THRESHOLD", 5),
		AIFailedParseCooldownWindowMin:  atoi("AI_FAILED_PARSE_COOLDOWN_WINDOW_MINUTES", 15),
		AIFailedParseCooldownMinutes:    atoi("AI_FAILED_PARSE_COOLDOWN_MINUTES", 10),
		AIProviderFailureThreshold:      atoi("AI_PROVIDER_FAILURE_THRESHOLD", 5),
		AIProviderCircuitBreakerSeconds: atoi("AI_PROVIDER_CIRCUIT_BREAKER_SECONDS", 120),
		OTPDebugResponse:                atob("OTP_DEBUG_RESPONSE", false),
		OTPDevCode:                      getenv("OTP_DEV_CODE", ""),
		OTPExpiresMinutes:               atoi("OTP_EXPIRES_MINUTES", 10),
		ClaimTokenMinutes:               atoi("CLAIM_TOKEN_EXPIRES_MINUTES", 15),
		GoogleClientIDs:                 csv("GOOGLE_CLIENT_IDS"),
		AIDailyCostAlertUSDMicros:       atoi64("AI_DAILY_COST_ALERT_USD_MICROS", 0),
		AIAbuseDailyCreditsThreshold:    atoi("AI_ABUSE_DAILY_CREDITS_THRESHOLD", 500),
		AIFreeCostPerUserAlertUSDMicros: atoi64("AI_FREE_COST_PER_USER_ALERT_USD_MICROS", 100000),
	}
}

func ResolveBackendPath(rel string) string {
	candidates := []string{
		rel,
		filepath.Join("EZ-Money-BE", rel),
	}

	if _, file, _, ok := runtime.Caller(0); ok && filepath.IsAbs(file) {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", rel))
	}

	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			if filepath.Base(dir) == "EZ-Money-BE" {
				candidates = append(candidates, filepath.Join(dir, rel))
			}
			candidates = append(candidates, filepath.Join(dir, "EZ-Money-BE", rel))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return rel
}
