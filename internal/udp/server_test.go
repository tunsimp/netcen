package udp

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNotificationServerBroadcastSendsToOtherClientsOnly(t *testing.T) {
	serverConn, err := net.ListenUDP("udp", udpAddr(t, "127.0.0.1:0"))
	require.NoError(t, err)
	defer serverConn.Close()

	senderConn, err := net.ListenUDP("udp", udpAddr(t, "127.0.0.1:0"))
	require.NoError(t, err)
	defer senderConn.Close()

	receiverConn, err := net.ListenUDP("udp", udpAddr(t, "127.0.0.1:0"))
	require.NoError(t, err)
	defer receiverConn.Close()

	notificationServer := NewNotificationServer(":0")
	notificationServer.clients[senderConn.LocalAddr().String()] = senderConn.LocalAddr().(*net.UDPAddr)
	notificationServer.clients[receiverConn.LocalAddr().String()] = receiverConn.LocalAddr().(*net.UDPAddr)

	notification := Notification{
		Type:      "progress",
		MangaID:   "one-piece",
		Message:   "chapter updated",
		Timestamp: 123,
	}
	payload, err := json.Marshal(notification)
	require.NoError(t, err)

	notificationServer.broadcast(serverConn, string(payload), senderConn.LocalAddr().(*net.UDPAddr))

	_ = receiverConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := receiverConn.ReadFromUDP(buf)
	require.NoError(t, err)
	require.JSONEq(t, string(payload), string(buf[:n]))

	_ = senderConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = senderConn.ReadFromUDP(buf)
	require.Error(t, err)
}

func udpAddr(t *testing.T, address string) *net.UDPAddr {
	t.Helper()

	addr, err := net.ResolveUDPAddr("udp", address)
	require.NoError(t, err)
	return addr
}
