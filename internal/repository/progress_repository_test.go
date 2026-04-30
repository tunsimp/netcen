package repository_test

import (
	"path/filepath"
	"testing"

	"mangahub/internal/database"
	"mangahub/internal/models"
	"mangahub/internal/repository"
)

func TestProgressRepositoryUpsertAndLoad(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	progressRepo := repository.NewProgressRepository(db)

	user, err := userRepo.Create("reader", "hashed-password")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	progress, err := progressRepo.Upsert(models.UserProgress{
		UserID:         user.ID,
		MangaID:        "one-piece",
		CurrentChapter: 100,
		Status:         "reading",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if progress.CurrentChapter != 100 {
		t.Fatalf("CurrentChapter = %d, want 100", progress.CurrentChapter)
	}

	progress, err = progressRepo.Upsert(models.UserProgress{
		UserID:         user.ID,
		MangaID:        "one-piece",
		CurrentChapter: 101,
		Status:         "reading",
	})
	if err != nil {
		t.Fatalf("Upsert() second error = %v", err)
	}
	if progress.CurrentChapter != 101 {
		t.Fatalf("CurrentChapter = %d, want 101", progress.CurrentChapter)
	}

	progressList, err := progressRepo.ListByUser(user.ID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(progressList) != 1 {
		t.Fatalf("len(progressList) = %d, want 1", len(progressList))
	}
	if progressList[0].MangaID != "one-piece" {
		t.Fatalf("progressList[0].MangaID = %s, want one-piece", progressList[0].MangaID)
	}
}
