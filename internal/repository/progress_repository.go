package repository

import (
	"database/sql"
	"fmt"

	"mangahub/internal/models"
)

type ProgressRepository struct {
	db *sql.DB
}

func NewProgressRepository(db *sql.DB) *ProgressRepository {
	return &ProgressRepository{db: db}
}

func (r *ProgressRepository) Upsert(progress models.UserProgress) (*models.UserProgress, error) {
	query := `
INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, manga_id) DO UPDATE SET
	current_chapter = excluded.current_chapter,
	status = excluded.status,
	updated_at = CURRENT_TIMESTAMP;`

	if _, err := r.db.Exec(query, progress.UserID, progress.MangaID, progress.CurrentChapter, progress.Status); err != nil {
		return nil, fmt.Errorf("failed to upsert progress: %w", err)
	}

	return r.FindByUserAndManga(progress.UserID, progress.MangaID)
}

func (r *ProgressRepository) FindByUserAndManga(userID, mangaID string) (*models.UserProgress, error) {
	row := r.db.QueryRow(`
SELECT user_id, manga_id, current_chapter, status, updated_at
FROM user_progress
WHERE user_id = ? AND manga_id = ?;`, userID, mangaID)

	progress, err := scanUserProgress(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("failed to query progress: %w", err)
	}

	return progress, nil
}

func (r *ProgressRepository) ListByUser(userID string) ([]models.UserProgress, error) {
	rows, err := r.db.Query(`
SELECT user_id, manga_id, current_chapter, status, updated_at
FROM user_progress
WHERE user_id = ?
ORDER BY updated_at DESC, manga_id ASC;`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user progress list: %w", err)
	}
	defer rows.Close()

	var progressList []models.UserProgress
	for rows.Next() {
		progress, err := scanUserProgress(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user progress: %w", err)
		}
		progressList = append(progressList, *progress)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user progress list: %w", err)
	}

	return progressList, nil
}

func scanUserProgress(scan func(dest ...any) error) (*models.UserProgress, error) {
	progress := &models.UserProgress{}
	if err := scan(
		&progress.UserID,
		&progress.MangaID,
		&progress.CurrentChapter,
		&progress.Status,
		&progress.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return progress, nil
}
