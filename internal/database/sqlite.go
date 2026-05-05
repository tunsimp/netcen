package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
    description    TEXT NOT NULL,
    cover_url      TEXT NOT NULL DEFAULT ''
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

	if err := ensureMangaCoverURLColumn(db); err != nil {
		return err
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

	seeds, err := loadMangaSeeds()
	if err != nil {
		return err
	}

	for _, manga := range seeds {
		rawGenres, err := json.Marshal(manga.Genres)
		if err != nil {
			return fmt.Errorf("failed to encode genres: %w", err)
		}

		if _, err := db.Exec(`
INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
			manga.ID,
			manga.Title,
			manga.Author,
			string(rawGenres),
			manga.Status,
			manga.TotalChapters,
			manga.Description,
			manga.CoverURL,
		); err != nil {
			return fmt.Errorf("failed to seed manga %s: %w", manga.ID, err)
		}
	}

	log.Printf("seeded %d manga records", len(seeds))
	return nil
}

func ensureMangaCoverURLColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(manga);`)
	if err != nil {
		return fmt.Errorf("failed to inspect manga schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("failed to scan manga schema: %w", err)
		}
		if name == "cover_url" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate manga schema: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE manga ADD COLUMN cover_url TEXT NOT NULL DEFAULT '';`); err != nil {
		return fmt.Errorf("failed to add manga cover_url column: %w", err)
	}
	return nil
}

func loadMangaSeeds() ([]models.Manga, error) {
	seedPath, err := findSeedPath()
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(seedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manga seed file %s: %w", seedPath, err)
	}

	var seeds []models.Manga
	if err := json.Unmarshal(raw, &seeds); err != nil {
		return nil, fmt.Errorf("failed to parse manga seed file %s: %w", seedPath, err)
	}
	if len(seeds) < 30 {
		return nil, fmt.Errorf("manga seed file must contain at least 30 records, found %d", len(seeds))
	}

	seen := make(map[string]struct{}, len(seeds))
	for _, manga := range seeds {
		if err := validateSeedManga(manga); err != nil {
			return nil, err
		}
		if _, ok := seen[manga.ID]; ok {
			return nil, fmt.Errorf("duplicate manga seed id: %s", manga.ID)
		}
		seen[manga.ID] = struct{}{}
	}

	return seeds, nil
}

func validateSeedManga(manga models.Manga) error {
	switch {
	case strings.TrimSpace(manga.ID) == "":
		return errors.New("manga seed id is required")
	case strings.TrimSpace(manga.Title) == "":
		return fmt.Errorf("manga seed %s title is required", manga.ID)
	case strings.TrimSpace(manga.Author) == "":
		return fmt.Errorf("manga seed %s author is required", manga.ID)
	case len(manga.Genres) == 0:
		return fmt.Errorf("manga seed %s must have at least one genre", manga.ID)
	case strings.TrimSpace(manga.Status) == "":
		return fmt.Errorf("manga seed %s status is required", manga.ID)
	case manga.TotalChapters < 1:
		return fmt.Errorf("manga seed %s total_chapters must be greater than 0", manga.ID)
	case strings.TrimSpace(manga.Description) == "":
		return fmt.Errorf("manga seed %s description is required", manga.ID)
	case strings.TrimSpace(manga.CoverURL) == "":
		return fmt.Errorf("manga seed %s cover_url is required", manga.ID)
	default:
		return nil
	}
}

func findSeedPath() (string, error) {
	candidates := []string{
		filepath.Join("data", "manga_seed.json"),
		filepath.Join("..", "data", "manga_seed.json"),
		filepath.Join("..", "..", "data", "manga_seed.json"),
		filepath.Join("..", "..", "..", "data", "manga_seed.json"),
	}

	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
		candidates = append(candidates, filepath.Join(repoRoot, "data", "manga_seed.json"))
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", errors.New("failed to find data/manga_seed.json")
}
