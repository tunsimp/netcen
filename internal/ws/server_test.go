package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"mangahub/internal/auth"
	"mangahub/internal/config"
	"mangahub/internal/models"
	"mangahub/internal/ws"
)

func TestWebSocketChatBroadcast(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		WSPort: freeTCPPort(t),
	}
	jwtManager := auth.NewManager("test-secret")
	server := ws.NewServer(cfg, jwtManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	waitForHTTPServer(t, cfg.WSPort)

	tokenA, err := jwtManager.CreateToken("user-a", "alice")
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	tokenB, err := jwtManager.CreateToken("user-b", "bob")
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	connA := mustDialWS(t, cfg.WSPort, tokenA)
	defer connA.Close()

	connB := mustDialWS(t, cfg.WSPort, tokenB)
	defer connB.Close()

	if err := connA.WriteJSON(map[string]any{
		"type":    "chat",
		"message": "hello from alice",
	}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var got models.ChatMessage
	readWSJSON(t, connB, &got)

	if got.Type != "chat" {
		t.Fatalf("Type = %s, want chat", got.Type)
	}
	if got.UserID != "user-a" {
		t.Fatalf("UserID = %s, want user-a", got.UserID)
	}
	if got.Username != "alice" {
		t.Fatalf("Username = %s, want alice", got.Username)
	}
	if got.Message != "hello from alice" {
		t.Fatalf("Message = %s, want hello from alice", got.Message)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("websocket server did not stop in time")
	}
}

func TestWebSocketRejectsMissingToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		WSPort: freeTCPPort(t),
	}
	server := ws.NewServer(cfg, auth.NewManager("test-secret"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	waitForHTTPServer(t, cfg.WSPort)

	_, resp, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:"+cfg.WSPort+"/ws", nil)
	if err == nil {
		t.Fatal("Dial() expected error for missing token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("websocket server did not stop in time")
	}
}

func mustDialWS(t *testing.T, port, token string) *websocket.Conn {
	t.Helper()

	url := "ws://127.0.0.1:" + port + "/ws?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Dial(%s) error = %v", url, err)
	}
	return conn
}

func readWSJSON(t *testing.T, conn *websocket.Conn, target any) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func waitForHTTPServer(t *testing.T, port string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	url := "http://127.0.0.1:" + port + "/ws"
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err == nil {
			q := req.URL.Query()
			q.Set("token", "invalid")
			req.URL.RawQuery = q.Encode()
			resp, callErr := http.DefaultClient.Do(req)
			if callErr == nil {
				_ = resp.Body.Close()
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("websocket server on port %s not ready", port)
}

func freeTCPPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}
