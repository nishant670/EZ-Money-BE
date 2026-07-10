package http

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/config"
	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "authorization_header_missing"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(401, gin.H{"error": "authorization_header_invalid"})
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid_token"})
			return
		}

		var session models.AuthSession
		if err := database.DB.Preload("User").
			Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hashSessionToken(token), time.Now().UTC()).
			First(&session).Error; err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid_or_expired_session"})
			return
		}
		user := session.User
		if user.ID == 0 {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid_session_user"})
			return
		}
		// Store user in context
		c.Set("user", &user)
		c.Set("userID", user.ID)

		c.Next()
	}
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type memoryRateLimiter struct {
	mu      sync.Mutex
	rps     float64
	burst   int
	buckets map[string]*tokenBucket
	now     func() time.Time
}

func newMemoryRateLimiter(rps float64, burst int) *memoryRateLimiter {
	return &memoryRateLimiter{
		rps:     rps,
		burst:   burst,
		buckets: make(map[string]*tokenBucket),
		now:     time.Now,
	}
}

func (l *memoryRateLimiter) allow(key string) (bool, int) {
	if l.rps <= 0 || l.burst <= 0 {
		return true, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	bucket, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &tokenBucket{tokens: float64(l.burst - 1), last: now}
		return true, 0
	}

	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = math.Min(float64(l.burst), bucket.tokens+elapsed*l.rps)
		bucket.last = now
	}

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}

	retryAfter := int(math.Ceil((1 - bucket.tokens) / l.rps))
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, retryAfter
}

func rateLimit(cfg *config.Config, scope string) gin.HandlerFunc {
	limiter := newMemoryRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	return func(c *gin.Context) {
		key := scope + ":" + c.ClientIP()
		ok, retryAfter := limiter.allow(key)
		if !ok {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(429, gin.H{"error": "rate_limited"})
			return
		}
		c.Next()
	}
}

func requestLimits(maxBytes int64, timeoutSec int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_body_too_large"})
			return
		}
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}

		if timeoutSec > 0 {
			ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}

func jsonRequestLimits(cfg *config.Config) gin.HandlerFunc {
	return requestLimits(cfg.MaxJSONKB*1024, cfg.ReqTimeoutSec)
}

func uploadRequestLimits(cfg *config.Config) gin.HandlerFunc {
	return requestLimits(cfg.MaxUploadMB*1024*1024, cfg.ReqTimeoutSec)
}

func requestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
