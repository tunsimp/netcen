package manga

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

var ErrMangaExists = errors.New("manga already exists")

func SeedFromJSON(db *sql.DB, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var mangas []Manga
	if err := json.Unmarshal(data, &mangas); err != nil {
		return err
	}

	for _, manga := range mangas {
		if err := validateManga(manga); err != nil {
			return err
		}

		_, err := db.Exec(
			`INSERT INTO manga(id, title, author, genres, status, total_chapters, description)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			 title = excluded.title,
			 author = excluded.author,
			 genres = excluded.genres,
			 status = excluded.status,
			 total_chapters = excluded.total_chapters,
			 description = excluded.description`,
			manga.ID,
			manga.Title,
			manga.Author,
			strings.Join(manga.Genres, ", "),
			manga.Status,
			manga.TotalChapters,
			manga.Description,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func InsertManga(db *sql.DB, manga Manga) error {
	manga.ID = strings.TrimSpace(manga.ID)
	manga.Title = strings.TrimSpace(manga.Title)
	manga.Author = strings.TrimSpace(manga.Author)
	manga.Status = strings.TrimSpace(manga.Status)
	manga.Description = strings.TrimSpace(manga.Description)

	if manga.Source == "" {
		manga.Source = "manual"
	}
	if err := validateManga(manga); err != nil {
		return err
	}

	_, err := db.Exec(
		`INSERT INTO manga(id, title, author, genres, status, total_chapters, description)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		manga.ID,
		manga.Title,
		manga.Author,
		strings.Join(manga.Genres, ", "),
		manga.Status,
		manga.TotalChapters,
		manga.Description,
	)
	if err != nil && strings.Contains(err.Error(), "constraint failed") {
		return ErrMangaExists
	}

	return err
}

func validateManga(manga Manga) error {
	if strings.TrimSpace(manga.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(manga.Title) == "" {
		return errors.New("title is required")
	}
	if manga.TotalChapters < 0 {
		return errors.New("total_chapters must be >= 0")
	}

	status := strings.ToLower(strings.TrimSpace(manga.Status))
	if status != "" &&
		status != "ongoing" &&
		status != "completed" &&
		status != "hiatus" &&
		status != "cancelled" &&
		status != "unknown" {
		return errors.New("invalid status")
	}

	return nil
}
