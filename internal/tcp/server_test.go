package tcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"mangahub/internal/tcp"
)

func TestTCPServerBroadcastsProgressUpdates(t *testing.T) {
	t.Parallel()

	cfg := socketConfig(t)
	server := tcp.NewServer(cfg)

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

	fmt.Fprintf(conn2, "{\"type\":\"progress_update\",\"user_id\":\"user-1\",\"manga_id\":\"one-piece\",\"chapter\":1095,\"status\":\"reading\"}\n")

	var ack map[string]any
	readJSONLine(t, conn2, &ack)
	if ack["type"] != "ack" {
		t.Fatalf("ack type = %v, want ack", ack["type"])
	}

	var broadcast tcp.ProgressUpdate
	readJSONLine(t, conn1, &broadcast)
	if broadcast.Type != "progress_broadcast" || broadcast.MangaID != "one-piece" || broadcast.Chapter != 1095 {
		t.Fatalf("broadcast = %#v", broadcast)
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
