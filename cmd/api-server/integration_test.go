package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"mangahub/internal/auth"
	"mangahub/internal/config"
	"mangahub/internal/database"
	internalgrpc "mangahub/internal/grpc"
	"mangahub/internal/http/handlers"
	"mangahub/internal/http/middleware"
	httprouter "mangahub/internal/http/router"
	"mangahub/internal/repository"
	"mangahub/internal/services"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	"mangahub/internal/ws"
)

func TestProtocolIntegrationWeek8(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		HTTPPort:  freeTCPPort(t),
		TCPPort:   freeTCPPort(t),
		UDPPort:   freeUDPPort(t),
		WSPort:    freeTCPPort(t),
		GRPCPort:  freeTCPPort(t),
		DBPath:    filepath.Join(t.TempDir(), "integration.db"),
		JWTSecret: "integration-secret",
	}

	db := mustInitDB(t, cfg.DBPath)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	mangaRepo := repository.NewMangaRepository(db)
	progressRepo := repository.NewProgressRepository(db)

	jwtManager := auth.NewManager(cfg.JWTSecret)
	progressService := services.NewProgressService(mangaRepo, progressRepo)
	notificationService := services.NewNotificationService(mangaRepo)

	httpServer := httprouter.NewServer(cfg, handlers.NewAuthHandler(userRepo, jwtManager), middleware.RequireAuth(jwtManager))
	tcpServer := tcp.NewServer(cfg, progressService)
	udpServer := udp.NewServer(cfg, notificationService)
	wsServer := ws.NewServer(cfg, jwtManager)
	grpcServer := internalgrpc.NewServer(cfg, notificationService)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 5)
	go func() { errCh <- httpServer.Start() }()
	go func() { errCh <- tcpServer.Start(ctx) }()
	go func() { errCh <- udpServer.Start(ctx) }()
	go func() { errCh <- wsServer.Start(ctx) }()
	go func() { errCh <- grpcServer.Start(ctx) }()

	waitForHTTPHealth(t, cfg.HTTPPort)
	waitForTCPListener(t, cfg.TCPPort)
	waitForUDPListener(t, cfg.UDPPort)
	waitForWebSocket(t, cfg.WSPort)
	grpcConn := mustDialGRPC(t, cfg.GRPCPort)
	defer grpcConn.Close()

	token := mustRegisterAndLogin(t, cfg.HTTPPort)

	tcpConn := mustDialTCP(t, cfg.TCPPort)
	defer tcpConn.Close()
	fmt.Fprintf(tcpConn, "{\"type\":\"hello\",\"client_id\":\"integration\"}\n")
	var hello map[string]any
	readTCPJSON(t, tcpConn, &hello)
	if hello["type"] != "hello" {
		t.Fatalf("tcp hello response = %v", hello)
	}

	wsConn := mustDialWS(t, cfg.WSPort, token)
	defer wsConn.Close()
	if err := wsConn.WriteJSON(map[string]any{"type": "chat", "message": "integration check"}); err != nil {
		t.Fatalf("websocket WriteJSON error = %v", err)
	}
	var chatPayload map[string]any
	readWSJSON(t, wsConn, &chatPayload)
	if chatPayload["type"] != "chat" {
		t.Fatalf("websocket payload type = %v, want chat", chatPayload["type"])
	}

	udpConn := mustDialUDP(t, cfg.UDPPort)
	defer udpConn.Close()
	if _, err := udpConn.Write([]byte(`{"type":"register"}`)); err != nil {
		t.Fatalf("udp register write error = %v", err)
	}
	var registered map[string]any
	readUDPJSON(t, udpConn, &registered)
	if registered["type"] != "registered" {
		t.Fatalf("udp register payload = %v", registered)
	}

	var publishRes internalgrpc.PublishNotificationResponse
	if err := grpcConn.Invoke(context.Background(), internalgrpc.MethodPublishNotification, &internalgrpc.PublishNotificationRequest{
		MangaID:   "one-piece",
		Message:   "integration notification",
		Timestamp: 1710000000,
	}, &publishRes); err != nil {
		t.Fatalf("grpc PublishNotification error = %v", err)
	}
	if publishRes.Status != "ok" {
		t.Fatalf("grpc publish status = %s, want ok", publishRes.Status)
	}

	var notification map[string]any
	readUDPJSON(t, udpConn, &notification)
	if notification["type"] != "chapter_release" {
		t.Fatalf("udp notification type = %v, want chapter_release", notification["type"])
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = wsServer.Shutdown(shutdownCtx)

	for i := 0; i < 5; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("server start/stop error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("servers did not stop in time")
		}
	}
}

func mustInitDB(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := database.NewSQLite(path)
	if err != nil {
		t.Fatalf("database.NewSQLite() error = %v", err)
	}
	return db
}

func mustRegisterAndLogin(t *testing.T, httpPort string) string {
	t.Helper()

	baseURL := "http://127.0.0.1:" + httpPort
	body := map[string]string{
		"username": "integration-user",
		"password": "secret123",
	}

	registerBody, _ := json.Marshal(body)
	registerResp, err := http.Post(baseURL+"/auth/register", "application/json", bytes.NewReader(registerBody))
	if err != nil {
		t.Fatalf("register request error = %v", err)
	}
	_ = registerResp.Body.Close()
	if registerResp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", registerResp.StatusCode, http.StatusCreated)
	}

	loginBody, _ := json.Marshal(body)
	loginResp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login request error = %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResp.StatusCode, http.StatusOK)
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode login response error = %v", err)
	}
	if payload.Token == "" {
		t.Fatal("login token is empty")
	}
	return payload.Token
}

func waitForHTTPHealth(t *testing.T, port string) {
	t.Helper()

	url := "http://127.0.0.1:" + port + "/healthz"
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("http server %s not ready", url)
}

func waitForWebSocket(t *testing.T, port string) {
	t.Helper()

	url := "http://127.0.0.1:" + port + "/ws"
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("websocket server on %s not ready", port)
}

func waitForTCPListener(t *testing.T, port string) {
	t.Helper()

	addr := "127.0.0.1:" + port
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("tcp listener %s not ready", addr)
}

func waitForUDPListener(t *testing.T, port string) {
	t.Helper()

	conn := mustDialUDP(t, port)
	defer conn.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := conn.Write([]byte(`{"type":"register"}`)); err == nil {
			var payload map[string]any
			if readUDPJSONNonFatal(conn, &payload) && payload["type"] == "registered" {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("udp listener on %s not ready", port)
}

func mustDialGRPC(t *testing.T, port string) *gogrpc.ClientConn {
	t.Helper()

	target := "127.0.0.1:" + port
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
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

func mustDialTCP(t *testing.T, port string) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("net.Dial(tcp) error = %v", err)
	}
	return conn
}

func mustDialUDP(t *testing.T, port string) net.Conn {
	t.Helper()

	conn, err := net.Dial("udp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("net.Dial(udp) error = %v", err)
	}
	return conn
}

func mustDialWS(t *testing.T, port, token string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:"+port+"/ws?token="+token, nil)
	if err != nil {
		t.Fatalf("websocket dial error = %v", err)
	}
	return conn
}

func readTCPJSON(t *testing.T, conn net.Conn, target any) {
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
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
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

func readUDPJSON(t *testing.T, conn net.Conn, target any) {
	t.Helper()
	if !readUDPJSONNonFatal(conn, target) {
		t.Fatal("failed to read UDP json payload")
	}
}

func readUDPJSONNonFatal(conn net.Conn, target any) bool {
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buffer := make([]byte, 2048)
	n, err := conn.Read(buffer)
	if err != nil {
		return false
	}
	return json.Unmarshal(buffer[:n], target) == nil
}

func freeTCPPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(tcp) error = %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func freeUDPPort(t *testing.T) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.ListenPacket(udp) error = %v", err)
	}
	defer conn.Close()
	return fmt.Sprintf("%d", conn.LocalAddr().(*net.UDPAddr).Port)
}
