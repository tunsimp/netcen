package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"project/pkg/models"
)

var (
	ErrUserExists         = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type Service struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewService(db *sql.DB, jwtSecret []byte) *Service {
	return &Service{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

func (s *Service) Register(username, password string) (*models.User, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	userID, err := newID()
	if err != nil {
		return nil, err
	}

	_, err = s.db.Exec(
		`INSERT INTO users(id, username, password_hash) VALUES (?, ?, ?)`,
		userID,
		username,
		string(passwordHash),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrUserExists
		}
		return nil, err
	}

	return s.GetUserByID(userID)
}

func (s *Service) Login(username, password string) (string, error) {
	var user models.User
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`,
		username,
	)
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", ErrInvalidCredentials
	}

	claims := jwt.MapClaims{
		"sub": user.Username,
		"uid": user.ID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *Service) ParseToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", "", err
	}
	if !token.Valid {
		return "", "", ErrInvalidCredentials
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", ErrInvalidCredentials
	}

	userID, _ := claims["uid"].(string)
	username, _ := claims["sub"].(string)
	if userID == "" || username == "" {
		return "", "", ErrInvalidCredentials
	}

	return userID, username, nil
}

func (s *Service) GetUserByID(id string) (*models.User, error) {
	var user models.User
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`,
		id,
	)
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt); err != nil {
		return nil, err
	}
	return &user, nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed"))
}

func WrapValidationError(field string) error {
	return fmt.Errorf("%s is required", field)
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
