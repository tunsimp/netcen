package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"mangahub/internal/auth"
	"mangahub/internal/config"
	"mangahub/internal/database"
	"mangahub/internal/http/handlers"
	"mangahub/internal/http/middleware"
	httprouter "mangahub/internal/http/router"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

func TestAuthFlow(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := newTestServer(t)

	registerBody := map[string]string{
		"username": "reader",
		"password": "secret12",
	}
	rec := performJSON(server.Handler(), http.MethodPost, "/auth/register", registerBody, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	loginRec := performJSON(server.Handler(), http.MethodPost, "/auth/login", registerBody, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var loginPayload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("json.Unmarshal(login) error = %v", err)
	}

	meRec := performJSON(server.Handler(), http.MethodGet, "/auth/me", nil, loginPayload.Token)
	if meRec.Code != http.StatusOK {
		t.Fatalf("/auth/me status = %d, want %d, body=%s", meRec.Code, http.StatusOK, meRec.Body.String())
	}
}

func TestMangaEndpoints(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := newTestServer(t)

	listRec := performJSON(server.Handler(), http.MethodGet, "/manga", nil, "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /manga status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}

	var mangaList []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &mangaList); err != nil {
		t.Fatalf("json.Unmarshal(/manga) error = %v", err)
	}
	if len(mangaList) < 30 {
		t.Fatalf("len(mangaList) = %d, want at least 30", len(mangaList))
	}

	searchRec := performJSON(server.Handler(), http.MethodGet, "/manga?q=one", nil, "")
	if searchRec.Code != http.StatusOK {
		t.Fatalf("GET /manga?q=one status = %d, want %d, body=%s", searchRec.Code, http.StatusOK, searchRec.Body.String())
	}
	var searchList []map[string]any
	if err := json.Unmarshal(searchRec.Body.Bytes(), &searchList); err != nil {
		t.Fatalf("json.Unmarshal(search) error = %v", err)
	}
	if len(searchList) == 0 {
		t.Fatal("search result is empty, want at least one manga")
	}

	detailRec := performJSON(server.Handler(), http.MethodGet, "/manga/one-piece", nil, "")
	if detailRec.Code != http.StatusOK {
		t.Fatalf("GET /manga/one-piece status = %d, want %d, body=%s", detailRec.Code, http.StatusOK, detailRec.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("json.Unmarshal(detail) error = %v", err)
	}
	if detail["id"] != "one-piece" {
		t.Fatalf("detail id = %v, want one-piece", detail["id"])
	}
	if detail["cover_url"] == "" {
		t.Fatal("detail cover_url is empty")
	}

	missingRec := performJSON(server.Handler(), http.MethodGet, "/manga/unknown", nil, "")
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("GET /manga/unknown status = %d, want %d", missingRec.Code, http.StatusNotFound)
	}
}

func TestLibraryEndpoints(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	server := newTestServer(t)

	unauthorizedRec := performJSON(server.Handler(), http.MethodGet, "/users/library", nil, "")
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /users/library without token status = %d, want %d", unauthorizedRec.Code, http.StatusUnauthorized)
	}

	token := registerAndLogin(t, server.Handler())

	addBody := map[string]any{
		"manga_id":        "one-piece",
		"current_chapter": 1,
		"status":          "reading",
	}
	addRec := performJSON(server.Handler(), http.MethodPost, "/users/library", addBody, token)
	if addRec.Code != http.StatusOK {
		t.Fatalf("POST /users/library status = %d, want %d, body=%s", addRec.Code, http.StatusOK, addRec.Body.String())
	}

	listRec := performJSON(server.Handler(), http.MethodGet, "/users/library", nil, token)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /users/library status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var progressList []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &progressList); err != nil {
		t.Fatalf("json.Unmarshal(library) error = %v", err)
	}
	if len(progressList) != 1 {
		t.Fatalf("len(progressList) = %d, want 1", len(progressList))
	}

	updateBody := map[string]any{
		"manga_id":        "one-piece",
		"current_chapter": 2,
		"status":          "reading",
	}
	updateRec := performJSON(server.Handler(), http.MethodPut, "/users/progress", updateBody, token)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("PUT /users/progress status = %d, want %d, body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}
	var progress map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &progress); err != nil {
		t.Fatalf("json.Unmarshal(progress) error = %v", err)
	}
	if progress["current_chapter"] != float64(2) {
		t.Fatalf("current_chapter = %v, want 2", progress["current_chapter"])
	}
}

func registerAndLogin(t *testing.T, handler http.Handler) string {
	t.Helper()

	body := map[string]string{
		"username": "reader",
		"password": "secret12",
	}
	registerRec := performJSON(handler, http.MethodPost, "/auth/register", body, "")
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d, body=%s", registerRec.Code, http.StatusCreated, registerRec.Body.String())
	}

	loginRec := performJSON(handler, http.MethodPost, "/auth/login", body, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(login) error = %v", err)
	}
	if payload.Token == "" {
		t.Fatal("login token is empty")
	}
	return payload.Token
}

func newTestServer(t *testing.T) *httprouter.Server {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	userRepo := repository.NewUserRepository(db)
	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)
	jwtManager := auth.NewManager("test-secret")
	progressService := services.NewProgressService(mangaRepo, progressRepo)

	return httprouter.NewServer(
		cfgForTest(),
		handlers.NewAuthHandler(userRepo, jwtManager),
		handlers.NewMangaHandler(mangaRepo),
		handlers.NewLibraryHandler(progressRepo, progressService),
		middleware.RequireAuth(jwtManager),
	)
}

func cfgForTest() config.Config {
	return config.Config{
		HTTPPort:  "18080",
		TCPPort:   "19090",
		UDPPort:   "19091",
		DBPath:    "./test.db",
		JWTSecret: "test-secret",
	}
}

func performJSON(handler http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
