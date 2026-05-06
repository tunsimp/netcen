package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mangahub/internal/models"
)

type MangaRepository struct {
	db *sql.DB
}

type MangaFilters struct {
	Query  string
	Genre  string
	Status string
}

func NewMangaRepository(db *sql.DB) *MangaRepository {
	return &MangaRepository{db: db}
}

func (r *MangaRepository) FindByID(id string) (*models.Manga, error) {
	row := r.db.QueryRow(`
SELECT id, title, author, genres, status, total_chapters, description, cover_url
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

func (r *MangaRepository) List(filters MangaFilters) ([]models.Manga, error) {
	baseSQL := `
SELECT id, title, author, genres, status, total_chapters, description, cover_url
FROM manga`

	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)

	query := strings.TrimSpace(filters.Query)
	if query != "" {
		conditions = append(conditions, "(lower(title) LIKE lower(?) OR lower(author) LIKE lower(?))")
		searchValue := "%" + query + "%"
		args = append(args, searchValue, searchValue)
	}

	genre := strings.TrimSpace(filters.Genre)
	if genre != "" {
		conditions = append(conditions, "lower(genres) LIKE lower(?)")
		args = append(args, "%"+genre+"%")
	}

	status := strings.TrimSpace(filters.Status)
	if status != "" {
		conditions = append(conditions, "lower(status) = lower(?)")
		args = append(args, status)
	}

	if len(conditions) > 0 {
		baseSQL += "\nWHERE " + strings.Join(conditions, " AND ")
	}
	baseSQL += "\nORDER BY title ASC;"

	rows, err := r.db.Query(baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list manga: %w", err)
	}
	defer rows.Close()

	results := make([]models.Manga, 0)
	for rows.Next() {
		manga, scanErr := scanManga(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, *manga)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate manga list rows: %w", err)
	}

	return results, nil
}

func (r *MangaRepository) Search(query string) ([]models.Manga, error) {
	return r.List(MangaFilters{Query: query})
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
		&manga.CoverURL,
	); err != nil {
		return nil, fmt.Errorf("failed to scan manga: %w", err)
	}

	if err := json.Unmarshal([]byte(rawGenres), &manga.Genres); err != nil {
		return nil, fmt.Errorf("failed to decode genres: %w", err)
	}

	return &manga, nil
}
