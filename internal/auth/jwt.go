package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) CreateToken(userID, username string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) ParseToken(rawToken string) (string, string, error) {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.secret, nil
	})
	if err != nil {
		return "", "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", fmt.Errorf("invalid token claims")
	}

	userID, _ := claims["sub"].(string)
	username, _ := claims["username"].(string)
	if userID == "" || username == "" {
		return "", "", fmt.Errorf("missing token payload")
	}

	return userID, username, nil
}
