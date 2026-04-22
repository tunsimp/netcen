package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"mangahub/internal/auth"
	"mangahub/internal/http/middleware"
	"mangahub/internal/repository"
)

type AuthHandler struct {
	users *repository.UserRepository
	jwt   *auth.Manager
}

type authRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

func NewAuthHandler(users *repository.UserRepository, jwt *auth.Manager) *AuthHandler {
	return &AuthHandler{
		users: users,
		jwt:   jwt,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	existingUser, err := h.users.FindByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check user"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process password"})
		return
	}

	user, err := h.users.Create(req.Username, string(hash))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"created_at": user.CreatedAt,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	user, err := h.users.FindByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.jwt.CreateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get(middleware.ContextUserIDKey)
	username, _ := c.Get(middleware.ContextUsernameKey)

	c.JSON(http.StatusOK, gin.H{
		"id":       userID,
		"username": username,
	})
}
