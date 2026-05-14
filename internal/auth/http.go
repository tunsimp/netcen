package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterHTTPRoutes(router *gin.Engine, authService *Service) {
	router.POST("/auth/register", registerHandler(authService))
	router.POST("/auth/login", loginHandler(authService))
}

func AuthMiddleware(authService *Service) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if authHeader == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			ctx.Abort()
			return
		}

		tokenString := authHeader
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			tokenString = strings.TrimSpace(authHeader[7:])
		}
		if tokenString == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			ctx.Abort()
			return
		}

		userID, username, err := authService.ParseToken(tokenString)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			ctx.Abort()
			return
		}

		ctx.Set("userID", userID)
		ctx.Set("username", username)
		ctx.Next()
	}
}

func registerHandler(authService *Service) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req credentialsRequest
		err := ctx.ShouldBindJSON(&req)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Username == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": WrapValidationError("username").Error()})
			return
		}
		if req.Password == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": WrapValidationError("password").Error()})
			return
		}

		user, err := authService.Register(req.Username, req.Password)
		if err != nil {
			if err == ErrUserExists {
				ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
			return
		}

		ctx.JSON(http.StatusCreated, gin.H{
			"message": "registered successfully",
			"user": gin.H{
				"id":         user.ID,
				"username":   user.Username,
				"created_at": user.CreatedAt,
			},
		})
	}
}

func loginHandler(authService *Service) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req credentialsRequest
		err := ctx.ShouldBindJSON(&req)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.Username == "" || req.Password == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
			return
		}

		token, err := authService.Login(req.Username, req.Password)
		if err != nil {
			if err == ErrInvalidCredentials {
				ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"token": token})
	}
}
