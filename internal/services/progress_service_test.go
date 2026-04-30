package services_test

import (
	"errors"
	"path/filepath"
	"testing"

	"mangahub/internal/database"
	"mangahub/internal/models"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

func TestProgressServiceRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	service := newTestProgressService(t)

	testCases := []struct {
		name    string
		userID  string
		manga   string
		chapter int
		status  string
		want    error
	}{
		{name: "missing user id", userID: " ", manga: "one-piece", chapter: 1, status: models.ProgressStatusReading, want: services.ErrInvalidUserID},
		{name: "missing manga id", userID: "user-1", manga: " ", chapter: 1, status: models.ProgressStatusReading, want: services.ErrInvalidMangaID},
		{name: "invalid chapter", userID: "user-1", manga: "one-piece", chapter: 0, status: models.ProgressStatusReading, want: services.ErrInvalidChapter},
		{name: "invalid status", userID: "user-1", manga: "one-piece", chapter: 1, status: "watching", want: services.ErrInvalidProgressStatus},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := service.Upsert(tc.userID, tc.manga, tc.chapter, tc.status, 0)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestProgressServiceRejectsUnknownManga(t *testing.T) {
	t.Parallel()

	service := newTestProgressService(t)

	_, err := service.Upsert("user-1", "missing", 1, models.ProgressStatusReading, 0)
	if !errors.Is(err, services.ErrMangaNotFound) {
		t.Fatalf("error = %v, want ErrMangaNotFound", err)
	}
}

func newTestProgressService(t *testing.T) *services.ProgressService {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)
	return services.NewProgressService(mangaRepo, progressRepo)
}
