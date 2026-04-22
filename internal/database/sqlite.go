package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func NewSQLite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	if err := createSchema(db); err != nil {
		return nil, err
	}

	return db, nil
}

func createSchema(db *sql.DB) error {
	query := `
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	return nil
}
