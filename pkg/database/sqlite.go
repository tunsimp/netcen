package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func OpenSQLite(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func EnsureSchema(db *sql.DB) error {
	const createUsers = `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE,
		password_hash TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	const createManga = `
	CREATE TABLE IF NOT EXISTS manga (
		id TEXT PRIMARY KEY,
		title TEXT,
		author TEXT,
		genres TEXT,
		status TEXT,
		total_chapters INTEGER,
		description TEXT
	);`

	const createUserProgress = `
	CREATE TABLE IF NOT EXISTS user_progress (
		user_id TEXT,
		manga_id TEXT,
		current_chapter INTEGER,
		status TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, manga_id)
	);`

	if _, err := db.Exec(createUsers); err != nil {
		return err
	}
	if _, err := db.Exec(createManga); err != nil {
		return err
	}
	if _, err := db.Exec(createUserProgress); err != nil {
		return err
	}
	return nil
}
