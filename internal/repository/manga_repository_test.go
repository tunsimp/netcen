package repository_test

import (
	"path/filepath"
	"testing"

	"mangahub/internal/database"
	"mangahub/internal/repository"
)

func TestMangaRepositoryListAndFilters(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	defer db.Close()

	mangaRepo := repository.NewMangaRepository(db)

	all, err := mangaRepo.List(repository.MangaFilters{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) < 30 {
		t.Fatalf("len(all) = %d, want at least 30", len(all))
	}

	queryMatches, err := mangaRepo.List(repository.MangaFilters{Query: "one"})
	if err != nil {
		t.Fatalf("List(query) error = %v", err)
	}
	if len(queryMatches) == 0 {
		t.Fatal("query filter returned no results")
	}

	genreMatches, err := mangaRepo.List(repository.MangaFilters{Genre: "josei"})
	if err != nil {
		t.Fatalf("List(genre) error = %v", err)
	}
	if len(genreMatches) == 0 {
		t.Fatal("genre filter returned no results")
	}

	statusMatches, err := mangaRepo.List(repository.MangaFilters{Status: "completed"})
	if err != nil {
		t.Fatalf("List(status) error = %v", err)
	}
	if len(statusMatches) == 0 {
		t.Fatal("status filter returned no results")
	}

	onePiece, err := mangaRepo.FindByID("one-piece")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if onePiece == nil {
		t.Fatal("FindByID(one-piece) returned nil")
	}
	if onePiece.CoverURL == "" {
		t.Fatal("one-piece cover_url is empty")
	}
}
