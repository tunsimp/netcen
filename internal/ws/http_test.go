package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestChatBroadcastBetweenClients(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewChatHub()
	go hub.Run()

	router := gin.New()
	testAuth := func(ctx *gin.Context) {
		userID := ctx.GetHeader("X-UserID")
		username := ctx.GetHeader("X-User")
		if userID == "" || username == "" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Set("userID", userID)
		ctx.Set("username", username)
		ctx.Next()
	}
	RegisterHTTPRoutes(router, testAuth, hub)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat"

	aliceHeader := http.Header{}
	aliceHeader.Set("X-UserID", "u1")
	aliceHeader.Set("X-User", "alice")
	aliceConn, _, err := websocket.DefaultDialer.Dial(wsURL, aliceHeader)
	require.NoError(t, err)
	defer aliceConn.Close()

	bobHeader := http.Header{}
	bobHeader.Set("X-UserID", "u2")
	bobHeader.Set("X-User", "bob")
	bobConn, _, err := websocket.DefaultDialer.Dial(wsURL, bobHeader)
	require.NoError(t, err)
	defer bobConn.Close()

	_ = aliceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_ = bobConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Consume possible join system messages before the user message assertion.
	var scratch ChatMessage
	_ = aliceConn.ReadJSON(&scratch)
	_ = bobConn.ReadJSON(&scratch)

	require.NoError(t, aliceConn.WriteJSON(map[string]string{"message": "hello"}))

	var received ChatMessage
	require.NoError(t, bobConn.ReadJSON(&received))
	require.Equal(t, "alice", received.Username)
	require.Equal(t, "hello", received.Message)
	require.Equal(t, "u1", received.UserID)
}
