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

func (r *MangaRepository) List(filters MangaFilters) ([]models.Manga, error) {
	query := `
SELECT id, title, author, genres, status, total_chapters, description, cover_url
FROM manga
WHERE 1 = 1`
	args := make([]any, 0, 3)

	if value := strings.TrimSpace(filters.Query); value != "" {
		query += ` AND (LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(description) LIKE ?)`
		like := "%" + strings.ToLower(value) + "%"
		args = append(args, like, like, like)
	}
	if value := strings.TrimSpace(filters.Genre); value != "" {
		query += ` AND LOWER(genres) LIKE ?`
		args = append(args, "%"+strings.ToLower(value)+"%")
	}
	if value := strings.TrimSpace(filters.Status); value != "" {
		query += ` AND LOWER(status) = ?`
		args = append(args, strings.ToLower(value))
	}

	query += ` ORDER BY title ASC;`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query manga list: %w", err)
	}
	defer rows.Close()

	var mangaList []models.Manga
	for rows.Next() {
		manga, err := scanManga(rows.Scan)
		if err != nil {
			return nil, err
		}
		mangaList = append(mangaList, *manga)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate manga list: %w", err)
	}

	return mangaList, nil
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
