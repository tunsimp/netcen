package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"mangahub/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(username, passwordHash string) (*models.User, error) {
	user := &models.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: passwordHash,
	}

	query := `
INSERT INTO users (id, username, password_hash)
VALUES (?, ?, ?);`

	if _, err := r.db.Exec(query, user.ID, user.Username, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return r.FindByUsername(username)
}

func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	query := `
SELECT id, username, password_hash, created_at
FROM users
WHERE username = ?;`

	user := &models.User{}
	if err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query user by username: %w", err)
	}

	return user, nil
}

