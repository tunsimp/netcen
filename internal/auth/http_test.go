package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"project/pkg/database"
)

func setupAuthTest(t *testing.T) (*gin.Engine, *Service, *sql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)

	service := NewService(db, []byte("test-secret"))
	require.NoError(t, database.EnsureSchema(db))

	router := gin.New()
	RegisterHTTPRoutes(router, service)
	return router, service, db
}

func TestRegisterAndLogin(t *testing.T) {
	router, _, db := setupAuthTest(t)
	defer db.Close()

	registerBody := []byte(`{"username":"alice","password":"123456"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRes := httptest.NewRecorder()
	router.ServeHTTP(registerRes, registerReq)
	require.Equal(t, http.StatusCreated, registerRes.Code)

	loginBody := []byte(`{"username":"alice","password":"123456"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	router.ServeHTTP(loginRes, loginReq)
	require.Equal(t, http.StatusOK, loginRes.Code)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(loginRes.Body.Bytes(), &payload))
	require.NotEmpty(t, payload["token"])
}

func TestAuthMiddlewareUnauthorizedWithoutToken(t *testing.T) {
	router, service, db := setupAuthTest(t)
	defer db.Close()

	protected := router.Group("/protected")
	protected.Use(AuthMiddleware(service))
	protected.GET("", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusUnauthorized, res.Code)
}
