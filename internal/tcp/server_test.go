package tcp_test

import (
	"bufio"
	"context"
	"database/sql"
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
	"mangahub/internal/tcp"
)

func TestTCPServerBroadcastsProgressUpdates(t *testing.T) {
	t.Parallel()

	server, cfg, userID, db := newTestTCPServer(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	addr := waitForTCPListener(t, cfg.TCPPort)
	conn1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial conn1 error = %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial conn2 error = %v", err)
	}
	defer conn2.Close()

	fmt.Fprintf(conn1, "{\"type\":\"hello\",\"client_id\":\"cli-1\"}\n")
	readJSONLine(t, conn1, &map[string]any{})

	fmt.Fprintf(conn2, "{\"type\":\"progress_update\",\"user_id\":\"%s\",\"manga_id\":\"one-piece\",\"chapter\":1095,\"status\":\"reading\"}\n", userID)

	var ack map[string]any
	readJSONLine(t, conn2, &ack)
	if ack["type"] != "ack" {
		t.Fatalf("ack type = %v, want ack", ack["type"])
	}

	var broadcast models.ProgressUpdate
	readJSONLine(t, conn1, &broadcast)
	if broadcast.Type != "progress_broadcast" || broadcast.MangaID != "one-piece" || broadcast.Chapter != 1095 {
		t.Fatalf("broadcast = %#v", broadcast)
	}

	var storedChapter int
	if err := db.QueryRow(`SELECT current_chapter FROM user_progress WHERE user_id = ? AND manga_id = ?`, userID, "one-piece").Scan(&storedChapter); err != nil {
		t.Fatalf("query stored progress error = %v", err)
	}
	if storedChapter != 1095 {
		t.Fatalf("storedChapter = %d, want 1095", storedChapter)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tcp server did not stop in time")
	}
}

func TestTCPServerRejectsUnknownManga(t *testing.T) {
	t.Parallel()

	server, cfg, userID, db := newTestTCPServer(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	addr := waitForTCPListener(t, cfg.TCPPort)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial error = %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "{\"type\":\"progress_update\",\"user_id\":\"%s\",\"manga_id\":\"missing\",\"chapter\":10,\"status\":\"reading\"}\n", userID)

	var payload map[string]any
	readJSONLine(t, conn, &payload)
	if payload["type"] != "error" {
		t.Fatalf("type = %v, want error", payload["type"])
	}
	if payload["error"] != services.ErrMangaNotFound.Error() {
		t.Fatalf("error = %v, want %q", payload["error"], services.ErrMangaNotFound.Error())
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_progress WHERE user_id = ?`, userID).Scan(&count); err != nil {
		t.Fatalf("count progress error = %v", err)
	}
	if count != 0 {
		t.Fatalf("progress count = %d, want 0", count)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tcp server did not stop in time")
	}
}

func TestTCPServerRejectsInvalidStatusWithoutBroadcast(t *testing.T) {
	t.Parallel()

	server, cfg, userID, db := newTestTCPServer(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	addr := waitForTCPListener(t, cfg.TCPPort)
	conn1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial conn1 error = %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial conn2 error = %v", err)
	}
	defer conn2.Close()

	fmt.Fprintf(conn1, "{\"type\":\"hello\",\"client_id\":\"cli-1\"}\n")
	readJSONLine(t, conn1, &map[string]any{})

	fmt.Fprintf(conn2, "{\"type\":\"progress_update\",\"user_id\":\"%s\",\"manga_id\":\"one-piece\",\"chapter\":10,\"status\":\"watching\"}\n", userID)

	var payload map[string]any
	readJSONLine(t, conn2, &payload)
	if payload["type"] != "error" {
		t.Fatalf("type = %v, want error", payload["type"])
	}
	if payload["error"] != services.ErrInvalidProgressStatus.Error() {
		t.Fatalf("error = %v, want %q", payload["error"], services.ErrInvalidProgressStatus.Error())
	}

	assertNoJSONLine(t, conn1)

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tcp server did not stop in time")
	}
}

func readJSONLine(t *testing.T, conn net.Conn, target any) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("ReadBytes() error = %v", err)
	}

	if err := json.Unmarshal(line, target); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, line=%s", err, string(line))
	}
}

func newTestTCPServer(t *testing.T) (*tcp.Server, config.Config, string, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.Create("reader", "hash")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	cfg := config.Config{
		HTTPPort:  "18080",
		TCPPort:   freeTCPPort(t),
		UDPPort:   "19091",
		DBPath:    dbPath,
		JWTSecret: "test-secret",
	}

	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)
	progressService := services.NewProgressService(mangaRepo, progressRepo)

	return tcp.NewServer(cfg, progressService), cfg, user.ID, db
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

func waitForTCPListener(t *testing.T, port string) string {
	t.Helper()

	addr := "127.0.0.1:" + port
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return addr
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("tcp listener %s not ready", addr)
	return ""
}

func TestSeedMangaRunsOnlyOnce(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	defer db.Close()

	var firstCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM manga`).Scan(&firstCount); err != nil {
		t.Fatalf("count manga error = %v", err)
	}

	db2, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() second error = %v", err)
	}
	defer db2.Close()

	var secondCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM manga`).Scan(&secondCount); err != nil {
		t.Fatalf("count manga second error = %v", err)
	}

	if firstCount != secondCount {
		t.Fatalf("manga count changed unexpectedly: first=%d second=%d", firstCount, secondCount)
	}
}

func TestProgressServiceRejectsUnknownManga(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	defer db.Close()

	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)
	progressService := services.NewProgressService(mangaRepo, progressRepo)

	_, err = progressService.Upsert("user-1", "missing", 1, "reading", 0)
	if !errors.Is(err, services.ErrMangaNotFound) {
		t.Fatalf("error = %v, want ErrMangaNotFound", err)
	}
}

func assertNoJSONLine(t *testing.T, conn net.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	reader := bufio.NewReader(conn)
	_, err := reader.ReadBytes('\n')
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("ReadBytes() error = %v, want timeout", err)
	}
}
