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

func newTestServer(t *testing.T) *httprouter.Server {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	userRepo := repository.NewUserRepository(db)
	jwtManager := auth.NewManager("test-secret")

	return httprouter.NewServer(
		cfgForTest(),
		handlers.NewAuthHandler(userRepo, jwtManager),
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
