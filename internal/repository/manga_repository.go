package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"mangahub/internal/models"
)

type MangaRepository struct {
	db *sql.DB
}

func NewMangaRepository(db *sql.DB) *MangaRepository {
	return &MangaRepository{db: db}
}

func (r *MangaRepository) FindByID(id string) (*models.Manga, error) {
	row := r.db.QueryRow(`
SELECT id, title, author, genres, status, total_chapters, description
FROM manga
WHERE id = ?;`, id)

	manga, err := scanManga(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return manga, nil
}

func scanManga(scan func(dest ...any) error) (*models.Manga, error) {
	var manga models.Manga
	var rawGenres string

	if err := scan(
		&manga.ID,
		&manga.Title,
		&manga.Author,
		&rawGenres,
		&manga.Status,
		&manga.TotalChapters,
		&manga.Description,
	); err != nil {
		return nil, fmt.Errorf("failed to scan manga: %w", err)
	}

	if err := json.Unmarshal([]byte(rawGenres), &manga.Genres); err != nil {
		return nil, fmt.Errorf("failed to decode genres: %w", err)
	}

	return &manga, nil
}
