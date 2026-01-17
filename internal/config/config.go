
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	AllowOrigins   string
	AuthBearer     string
	TZDefault      string
	OpenAIKey      string
	OpenAIBaseURL  string
	OpenAILlmModel string
	OpenAIWhisper  string
	ReqTimeoutSec  int
	RateLimitRPS   float64
	RateLimitBurst int
	MaxUploadMB    int64
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

func atoi(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil { return i }
	}
	return def
}

func atof(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil { return f }
	}
	return def
}

func Load() *Config {
	return &Config{
		Port:           getenv("PORT", "8080"),
		AllowOrigins:   getenv("ALLOW_ORIGINS", "*"),
		AuthBearer:     getenv("AUTH_BEARER", ""),
		TZDefault:      getenv("TZ_DEFAULT", "Asia/Kolkata"),
		OpenAIKey:      getenv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:  getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAILlmModel: getenv("OPENAI_LLM_MODEL", "gpt-4o-mini"),
		OpenAIWhisper:  getenv("OPENAI_WHISPER_MODEL", "whisper-1"),
		ReqTimeoutSec:  atoi("REQUEST_TIMEOUT_SECONDS", 30),
		RateLimitRPS:   atof("RATE_LIMIT_RPS", 5),
		RateLimitBurst: atoi("RATE_LIMIT_BURST", 10),
		MaxUploadMB:    int64(atoi("MAX_UPLOAD_MB", 15)),
	}
}
