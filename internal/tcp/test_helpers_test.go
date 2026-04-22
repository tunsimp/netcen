package tcp_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"mangahub/internal/config"
)

func socketConfig(t *testing.T) config.Config {
	t.Helper()

	return config.Config{
		HTTPPort:  "18080",
		TCPPort:   freeTCPPort(t),
		DBPath:    "./test.db",
		JWTSecret: "test-secret",
	}
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
