package grpc_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"mangahub/internal/config"
	"mangahub/internal/database"
	internalgrpc "mangahub/internal/grpc"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	cancel, errCh, conn := startTestGRPCServer(t)
	defer conn.Close()

	var res internalgrpc.HealthCheckResponse
	if err := conn.Invoke(context.Background(), internalgrpc.MethodHealthCheck, &internalgrpc.HealthCheckRequest{}, &res); err != nil {
		t.Fatalf("Invoke(HealthCheck) error = %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("Status = %s, want ok", res.Status)
	}

	stopGRPCServer(t, cancel, errCh)
}

func TestGetManga(t *testing.T) {
	t.Parallel()

	cancel, errCh, conn := startTestGRPCServer(t)
	defer conn.Close()

	var res internalgrpc.MangaResponse
	if err := conn.Invoke(context.Background(), internalgrpc.MethodGetManga, &internalgrpc.GetMangaRequest{
		MangaID: "one-piece",
	}, &res); err != nil {
		t.Fatalf("Invoke(GetManga) error = %v", err)
	}
	if res.ID != "one-piece" {
		t.Fatalf("ID = %s, want one-piece", res.ID)
	}
	if res.Title == "" {
		t.Fatal("Title is empty")
	}

	stopGRPCServer(t, cancel, errCh)
}

func TestSearchManga(t *testing.T) {
	t.Parallel()

	cancel, errCh, conn := startTestGRPCServer(t)
	defer conn.Close()

	var res internalgrpc.SearchResponse
	if err := conn.Invoke(context.Background(), internalgrpc.MethodSearchManga, &internalgrpc.SearchRequest{
		Query: "piece",
	}, &res); err != nil {
		t.Fatalf("Invoke(SearchManga) error = %v", err)
	}
	if len(res.Manga) == 0 {
		t.Fatal("SearchManga returned empty result")
	}

	stopGRPCServer(t, cancel, errCh)
}

func TestUpdateProgress(t *testing.T) {
	t.Parallel()

	cancel, errCh, conn := startTestGRPCServer(t)
	defer conn.Close()

	var res internalgrpc.ProgressResponse
	if err := conn.Invoke(context.Background(), internalgrpc.MethodUpdateProgress, &internalgrpc.ProgressRequest{
		UserID:    "user-1",
		MangaID:   "one-piece",
		Chapter:   123,
		Status:    "reading",
		Timestamp: 1710000000,
	}, &res); err != nil {
		t.Fatalf("Invoke(UpdateProgress) error = %v", err)
	}
	if res.UserID != "user-1" || res.MangaID != "one-piece" || res.Chapter != 123 {
		t.Fatalf("ProgressResponse = %#v", res)
	}

	stopGRPCServer(t, cancel, errCh)
}

func TestUpdateProgressRejectsUnknownManga(t *testing.T) {
	t.Parallel()

	cancel, errCh, conn := startTestGRPCServer(t)
	defer conn.Close()

	var res internalgrpc.ProgressResponse
	err := conn.Invoke(context.Background(), internalgrpc.MethodUpdateProgress, &internalgrpc.ProgressRequest{
		UserID:    "user-1",
		MangaID:   "missing",
		Chapter:   1,
		Status:    "reading",
		Timestamp: 1710000000,
	}, &res)
	if err == nil {
		t.Fatal("Invoke(UpdateProgress) expected error for unknown manga")
	}
	stopGRPCServer(t, cancel, errCh)
}

func TestPublishNotification(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		GRPCPort: freeTCPPort(t),
	}
	db := newTestDB(t)
	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)
	progressService := services.NewProgressService(mangaRepo, progressRepo)
	notificationService := services.NewNotificationService(mangaRepo)
	server := internalgrpc.NewServer(cfg, mangaRepo, progressService, notificationService)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	conn := mustDialGRPC(t, cfg.GRPCPort)
	defer conn.Close()

	ch, unsubscribe := notificationService.Subscribe(1)
	defer unsubscribe()

	var res internalgrpc.PublishNotificationResponse
	if err := conn.Invoke(context.Background(), internalgrpc.MethodPublishNotification, &internalgrpc.PublishNotificationRequest{
		MangaID:   "one-piece",
		Message:   "new chapter",
		Timestamp: 1710000000,
	}, &res); err != nil {
		t.Fatalf("Invoke(PublishNotification) error = %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("Status = %s, want ok", res.Status)
	}

	select {
	case notification := <-ch:
		if notification.MangaID != "one-piece" {
			t.Fatalf("MangaID = %s, want one-piece", notification.MangaID)
		}
		if notification.Message != "new chapter" {
			t.Fatalf("Message = %s, want new chapter", notification.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive notification from service")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("grpc server did not stop in time")
	}
}

func startTestGRPCServer(t *testing.T) (context.CancelFunc, chan error, *gogrpc.ClientConn) {
	t.Helper()

	cfg := config.Config{
		GRPCPort: freeTCPPort(t),
	}

	db := newTestDB(t)
	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)
	progressService := services.NewProgressService(mangaRepo, progressRepo)
	notificationService := services.NewNotificationService(mangaRepo)
	server := internalgrpc.NewServer(cfg, mangaRepo, progressService, notificationService)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	conn := mustDialGRPC(t, cfg.GRPCPort)
	return cancel, errCh, conn
}

func stopGRPCServer(t *testing.T, cancel context.CancelFunc, errCh chan error) {
	t.Helper()

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("server.Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("grpc server did not stop in time")
	}
}

func mustDialGRPC(t *testing.T, port string) *gogrpc.ClientConn {
	t.Helper()

	target := "127.0.0.1:" + port
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		conn, err := gogrpc.DialContext(
			ctx,
			target,
			gogrpc.WithTransportCredentials(insecure.NewCredentials()),
			gogrpc.WithBlock(),
			gogrpc.WithDefaultCallOptions(gogrpc.CallContentSubtype("json")),
		)
		cancel()
		if err == nil {
			return conn
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("grpc server %s not ready", target)
	return nil
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
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
