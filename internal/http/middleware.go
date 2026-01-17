package http

import (
	"strings"

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

		token := parts[1]

		// Parse Mock Token: mock_token_{UUID}_{Random}
		if !strings.HasPrefix(token, "mock_token_") {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid_token_format"})
			return
		}

		tokenParts := strings.Split(token, "_")
		if len(tokenParts) < 4 {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid_token_structure"})
			return
		}

		uuid := tokenParts[2] // mock, token, UUID, Random...

		var user models.User
		if err := database.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid_token_user_not_found"})
			return
		}

		// Store user in context
		c.Set("user", &user)
		c.Set("userID", user.ID)

		c.Next()
	}
}
