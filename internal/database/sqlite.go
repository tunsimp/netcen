package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"mangahub/internal/models"

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

	if err := seedManga(db); err != nil {
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
);
CREATE TABLE IF NOT EXISTS manga (
    id             TEXT PRIMARY KEY,
    title          TEXT NOT NULL,
    author         TEXT NOT NULL,
    genres         TEXT NOT NULL,
    status         TEXT NOT NULL,
    total_chapters INTEGER NOT NULL,
    description    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS user_progress (
    user_id         TEXT NOT NULL,
    manga_id        TEXT NOT NULL,
    current_chapter INTEGER NOT NULL,
    status          TEXT NOT NULL,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, manga_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (manga_id) REFERENCES manga(id)
);`

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func seedManga(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM manga;`).Scan(&count); err != nil {
		return fmt.Errorf("failed to count manga: %w", err)
	}
	if count > 0 {
		return nil
	}

	seeds := []models.Manga{
		{ID: "one-piece", Title: "One Piece", Author: "Eiichiro Oda", Genres: []string{"Action", "Adventure", "Fantasy"}, Status: "ongoing", TotalChapters: 1112, Description: "Pirates chase the greatest treasure."},
		{ID: "kingdom", Title: "Kingdom", Author: "Yasuhisa Hara", Genres: []string{"Action", "Historical"}, Status: "ongoing", TotalChapters: 830, Description: "War and strategy in ancient China."},
		{ID: "frieren", Title: "Frieren: Beyond Journey's End", Author: "Kanehito Yamada", Genres: []string{"Fantasy", "Drama"}, Status: "ongoing", TotalChapters: 140, Description: "An elf mage reflects after the hero's journey."},
	}

	for _, manga := range seeds {
		rawGenres, err := json.Marshal(manga.Genres)
		if err != nil {
			return fmt.Errorf("failed to encode genres: %w", err)
		}

		if _, err := db.Exec(`
INSERT INTO manga (id, title, author, genres, status, total_chapters, description)
VALUES (?, ?, ?, ?, ?, ?, ?);`,
			manga.ID,
			manga.Title,
			manga.Author,
			string(rawGenres),
			manga.Status,
			manga.TotalChapters,
			manga.Description,
		); err != nil {
			return fmt.Errorf("failed to seed manga %s: %w", manga.ID, err)
		}
	}

	log.Printf("seeded %d manga records", len(seeds))
	return nil
}
