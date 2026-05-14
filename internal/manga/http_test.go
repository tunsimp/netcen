package manga

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"project/pkg/database"
)

func setupMangaTest(t *testing.T) (*gin.Engine, *sql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.EnsureSchema(db))

	router := gin.New()
	RegisterHTTPRoutes(router, db)
	return router, db
}

func sampleManga(id string) Manga {
	return Manga{
		ID:            id,
		Title:         "One Piece",
		Author:        "Eiichiro Oda",
		Genres:        []string{"action", "adventure"},
		Status:        "ongoing",
		TotalChapters: 1100,
		Description:   "Pirate journey",
	}
}

func TestInsertMangaCreatesRecord(t *testing.T) {
	_, db := setupMangaTest(t)
	defer db.Close()

	require.NoError(t, InsertManga(db, sampleManga("one-piece")))

	var title string
	err := db.QueryRow(`SELECT title FROM manga WHERE id = ?`, "one-piece").Scan(&title)
	require.NoError(t, err)
	require.Equal(t, "One Piece", title)
}

func TestInsertMangaValidatesInput(t *testing.T) {
	_, db := setupMangaTest(t)
	defer db.Close()

	tests := []struct {
		name  string
		manga Manga
	}{
		{name: "missing id", manga: Manga{Title: "One Piece", TotalChapters: 1}},
		{name: "missing title", manga: Manga{ID: "one-piece", TotalChapters: 1}},
		{name: "negative chapters", manga: Manga{ID: "one-piece", Title: "One Piece", TotalChapters: -1}},
		{name: "invalid status", manga: Manga{ID: "one-piece", Title: "One Piece", Status: "paused"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, InsertManga(db, tt.manga))
		})
	}
}

func TestInsertMangaReturnsErrMangaExistsForDuplicateID(t *testing.T) {
	_, db := setupMangaTest(t)
	defer db.Close()

	manga := sampleManga("one-piece")
	require.NoError(t, InsertManga(db, manga))

	err := InsertManga(db, manga)
	require.ErrorIs(t, err, ErrMangaExists)
}

func TestSeedFromJSONCreatesAndUpdatesManga(t *testing.T) {
	_, db := setupMangaTest(t)
	defer db.Close()

	require.NoError(t, InsertManga(db, sampleManga("one-piece")))

	seed := []Manga{
		{
			ID:            "one-piece",
			Title:         "One Piece Updated",
			Author:        "Eiichiro Oda",
			Genres:        []string{"action"},
			Status:        "ongoing",
			TotalChapters: 1101,
			Description:   "Updated",
		},
		{
			ID:            "chainsaw-man",
			Title:         "Chainsaw Man",
			Author:        "Tatsuki Fujimoto",
			Genres:        []string{"action", "horror"},
			Status:        "ongoing",
			TotalChapters: 200,
			Description:   "Devils",
		},
	}
	path := filepath.Join(t.TempDir(), "manga_seed.json")
	data, err := json.Marshal(seed)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	require.NoError(t, SeedFromJSON(db, path))

	var title string
	var chapters int
	err = db.QueryRow(`SELECT title, total_chapters FROM manga WHERE id = ?`, "one-piece").Scan(&title, &chapters)
	require.NoError(t, err)
	require.Equal(t, "One Piece Updated", title)
	require.Equal(t, 1101, chapters)

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM manga`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestSearchMangaHandlerReturnsAndFiltersManga(t *testing.T) {
	router, db := setupMangaTest(t)
	defer db.Close()

	require.NoError(t, InsertManga(db, sampleManga("one-piece")))
	require.NoError(t, InsertManga(db, Manga{
		ID:            "monster",
		Title:         "Monster",
		Author:        "Naoki Urasawa",
		Genres:        []string{"mystery", "thriller"},
		Status:        "completed",
		TotalChapters: 162,
		Description:   "Doctor thriller",
	}))

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/manga?title=piece&status=ongoing&genre=action", nil)
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code)
	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	require.Equal(t, "one-piece", body.Data[0]["id"])
}

func TestGetMangaByIDHandler(t *testing.T) {
	router, db := setupMangaTest(t)
	defer db.Close()

	require.NoError(t, InsertManga(db, sampleManga("one-piece")))

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/manga/one-piece", nil)
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "one-piece", body["id"])
	require.Equal(t, "One Piece", body["title"])
}

func TestGetMangaByIDHandlerReturnsNotFound(t *testing.T) {
	router, db := setupMangaTest(t)
	defer db.Close()

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/manga/missing", nil)
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNotFound, res.Code)
}

func TestCreateMangaHandler(t *testing.T) {
	router, db := setupMangaTest(t)
	defer db.Close()

	body, err := json.Marshal(sampleManga("one-piece"))
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/manga", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusCreated, res.Code)
}

func TestCreateMangaHandlerRejectsInvalidBodyAndDuplicate(t *testing.T) {
	router, db := setupMangaTest(t)
	defer db.Close()

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/manga", bytes.NewBufferString(`{"id":`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)

	body, err := json.Marshal(sampleManga("one-piece"))
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodPost, "/manga", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), req)

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/manga", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusConflict, res.Code)
}

func TestImportMangaDexHandlerRequiresTitle(t *testing.T) {
	router, db := setupMangaTest(t)
	defer db.Close()

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/manga/import/mangadex", nil)
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code)
}

func TestDuplicateErrorStillMatchesErrMangaExists(t *testing.T) {
	_, db := setupMangaTest(t)
	defer db.Close()

	manga := sampleManga("one-piece")
	require.NoError(t, InsertManga(db, manga))
	err := InsertManga(db, manga)
	require.True(t, errors.Is(err, ErrMangaExists))
}
