package udp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"mangahub/internal/config"
	"mangahub/internal/database"
	"mangahub/internal/models"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

func TestUDPServerRegistersAndBroadcasts(t *testing.T) {
	t.Parallel()

	server, cfg, notificationService := newTestUDPServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	addr := waitForUDPServer(t, cfg.UDPPort)

	conn1, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("Dial conn1 error = %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("Dial conn2 error = %v", err)
	}
	defer conn2.Close()

	if _, err := conn1.Write([]byte(`{"type":"register"}`)); err != nil {
		t.Fatalf("conn1 write error = %v", err)
	}
	assertRegisteredResponse(t, conn1)

	if _, err := conn2.Write([]byte(`{"type":"register"}`)); err != nil {
		t.Fatalf("conn2 write error = %v", err)
	}
	assertRegisteredResponse(t, conn2)

	if err := notificationService.Publish("one-piece", "One Piece chapter 1096 released", 1710000000); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	assertNotification(t, conn1, "one-piece", "One Piece chapter 1096 released", 1710000000)
	assertNotification(t, conn2, "one-piece", "One Piece chapter 1096 released", 1710000000)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("udp server did not stop in time")
	}
}

func TestUDPServerIgnoresInvalidJSONAndUnsupportedMessage(t *testing.T) {
	t.Parallel()

	server, cfg, _ := newTestUDPServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	addr := waitForUDPServer(t, cfg.UDPPort)

	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("Dial error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(`{invalid`)); err != nil {
		t.Fatalf("write invalid json error = %v", err)
	}
	assertNoUDPResponse(t, conn)

	if _, err := conn.Write([]byte(`{"type":"ping"}`)); err != nil {
		t.Fatalf("write unsupported packet error = %v", err)
	}
	assertNoUDPResponse(t, conn)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("udp server did not stop in time")
	}
}

func TestUDPServerCleansUpClientsAfterRepeatedSendFailures(t *testing.T) {
	t.Parallel()

	server := &Server{
		clients: map[string]*client{
			"bad-client": {addr: fakeAddr("bad-client")},
		},
		conn: &fakePacketConn{
			writeErrByAddr: map[string]error{
				"bad-client": errors.New("write failed"),
			},
		},
	}

	notification := models.Notification{
		Type:      "chapter_release",
		MangaID:   "one-piece",
		Message:   "release",
		Timestamp: 1710000000,
	}

	server.broadcast(notification)
	server.broadcast(notification)
	server.broadcast(notification)

	server.mu.RLock()
	defer server.mu.RUnlock()

	if len(server.clients) != 0 {
		t.Fatalf("len(server.clients) = %d, want 0", len(server.clients))
	}
}

func assertRegisteredResponse(t *testing.T, conn net.Conn) {
	t.Helper()

	var payload map[string]any
	readUDPJSON(t, conn, &payload)

	if payload["type"] != "registered" {
		t.Fatalf("type = %v, want registered", payload["type"])
	}
	if payload["status"] != "ok" {
		t.Fatalf("status = %v, want ok", payload["status"])
	}
}

func assertNotification(t *testing.T, conn net.Conn, mangaID, message string, timestamp int64) {
	t.Helper()

	var payload struct {
		Type      string `json:"type"`
		MangaID   string `json:"manga_id"`
		Message   string `json:"message"`
		Timestamp int64  `json:"timestamp"`
	}
	readUDPJSON(t, conn, &payload)

	if payload.Type != "chapter_release" {
		t.Fatalf("Type = %s, want chapter_release", payload.Type)
	}
	if payload.MangaID != mangaID {
		t.Fatalf("MangaID = %s, want %s", payload.MangaID, mangaID)
	}
	if payload.Message != message {
		t.Fatalf("Message = %s, want %s", payload.Message, message)
	}
	if payload.Timestamp != timestamp {
		t.Fatalf("Timestamp = %d, want %d", payload.Timestamp, timestamp)
	}
}

func assertNoUDPResponse(t *testing.T, conn net.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	buffer := make([]byte, 512)
	_, err := conn.Read(buffer)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("Read() error = %v, want timeout", err)
	}
}

func readUDPJSON(t *testing.T, conn net.Conn, target any) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	buffer := make([]byte, 2048)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if err := json.Unmarshal(buffer[:n], target); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func newTestUDPServer(t *testing.T) (*Server, config.Config, *services.NotificationService) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := config.Config{
		HTTPPort:  "18080",
		TCPPort:   "19090",
		UDPPort:   freeUDPPort(t),
		DBPath:    dbPath,
		JWTSecret: "test-secret",
	}

	notificationService := services.NewNotificationService(repository.NewMangaRepository(db))
	return NewServer(cfg, notificationService), cfg, notificationService
}

func freeUDPPort(t *testing.T) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer conn.Close()

	return fmt.Sprintf("%d", conn.LocalAddr().(*net.UDPAddr).Port)
}

func waitForUDPServer(t *testing.T, port string) string {
	t.Helper()

	addr := "127.0.0.1:" + port
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("udp", addr)
		if err == nil {
			if _, writeErr := conn.Write([]byte(`{"type":"register"}`)); writeErr == nil {
				if readErr := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); readErr == nil {
					buffer := make([]byte, 512)
					if n, readPacketErr := conn.Read(buffer); readPacketErr == nil {
						var payload map[string]any
						if json.Unmarshal(buffer[:n], &payload) == nil && payload["type"] == "registered" {
							_ = conn.Close()
							return addr
						}
					}
				}
			}
			_ = conn.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("udp server %s not ready", addr)
	return ""
}

type fakeAddr string

func (a fakeAddr) Network() string { return "udp" }
func (a fakeAddr) String() string  { return string(a) }

type fakePacketConn struct {
	writeErrByAddr map[string]error
}

func (c *fakePacketConn) ReadFrom(_ []byte) (n int, addr net.Addr, err error) {
	return 0, nil, net.ErrClosed
}

func (c *fakePacketConn) WriteTo(_ []byte, addr net.Addr) (n int, err error) {
	if err, ok := c.writeErrByAddr[addr.String()]; ok {
		return 0, err
	}
	return 1, nil
}

func (c *fakePacketConn) Close() error { return nil }
