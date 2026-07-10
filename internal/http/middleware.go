package http

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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
