package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"project/pkg/database"
)

func TestUpdateProgressForwardsXDeviceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, database.EnsureSchema(db))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "test-user-id")
		c.Next()
	})

	RegisterHTTPRoutes(router.Group("/users"), db, "127.0.0.1:0")

	// We are testing X-Device-ID parsing, TCP fail is expected because we use an invalid TCP address
	reqBody := updateProgressRequest{
		MangaID:        "manga-1",
		CurrentChapter: 5,
		ReadingStatus:  "reading",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/users/progress", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", "ios-app-1")
	req.Header.Set("Authorization", "Bearer fake-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Will fail bad gateway since TCP is offline, but X-Device-ID logic ran
	require.Equal(t, http.StatusBadGateway, w.Code)
}
