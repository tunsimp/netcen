package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mangahub/internal/auth"
)

const (
	ContextUserIDKey   = "user_id"
	ContextUsernameKey = "username"
)

func RequireAuth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			c.Abort()
			return
		}

		userID, username, err := manager.ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, userID)
		c.Set(ContextUsernameKey, username)
		c.Next()
	}
}
