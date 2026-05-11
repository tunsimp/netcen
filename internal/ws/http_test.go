package ws

import (
	"fmt"
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

func TestChatBroadcastTwentyClients(t *testing.T) {
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
	const clientCount = 20

	conns := make([]*websocket.Conn, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		header := http.Header{}
		header.Set("X-UserID", fmt.Sprintf("u%d", i))
		header.Set("X-User", fmt.Sprintf("user%d", i))

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		require.NoError(t, err)
		conns = append(conns, conn)
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	sender := conns[0]
	require.NoError(t, sender.WriteJSON(map[string]string{"message": "hello-20"}))

	for i := 1; i < clientCount; i++ {
		msg, err := waitForMessage(conns[i], "user0", "hello-20", 5*time.Second)
		require.NoError(t, err)
		require.Equal(t, "u0", msg.UserID)
	}
}

func waitForMessage(conn *websocket.Conn, username, message string, timeout time.Duration) (ChatMessage, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	for {
		var msg ChatMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			return ChatMessage{}, err
		}
		if msg.Username == username && msg.Message == message {
			return msg, nil
		}
	}
}
