package tcp

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSendProgressUpdateWritesJSONLine(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	received := make(chan ProgressUpdate, 1)
	acceptErr := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				acceptErr <- err
				return
			}
			acceptErr <- nil
			return
		}

		var update ProgressUpdate
		if err := json.Unmarshal(scanner.Bytes(), &update); err != nil {
			acceptErr <- err
			return
		}

		received <- update
		acceptErr <- nil
	}()

	update := ProgressUpdate{
		UserID:    "u1",
		MangaID:   "one-piece",
		Chapter:   10,
		Status:    "reading",
		Timestamp: 123,
	}
	require.NoError(t, SendProgressUpdate(listener.Addr().String(), update))

	select {
	case got := <-received:
		require.Equal(t, update, got)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for progress update")
	}

	require.NoError(t, <-acceptErr)
}

func TestProgressHubBroadcastSendsToOtherClientsOnly(t *testing.T) {
	hub := NewProgressHub(":0")

	senderServer, senderClient := net.Pipe()
	defer senderServer.Close()
	defer senderClient.Close()

	receiverServer, receiverClient := net.Pipe()
	defer receiverServer.Close()
	defer receiverClient.Close()

	hub.clients[senderServer] = true
	hub.clients[receiverServer] = true

	message := `{"user_id":"u1","manga_id":"one-piece","chapter":10,"status":"reading","timestamp":123}`
	received := make(chan string, 1)
	receiverErr := make(chan error, 1)

	go func() {
		_ = receiverClient.SetReadDeadline(time.Now().Add(2 * time.Second))
		line, err := bufio.NewReader(receiverClient).ReadString('\n')
		if err != nil {
			receiverErr <- err
			return
		}
		received <- line
		receiverErr <- nil
	}()

	hub.broadcast(message, senderServer)

	select {
	case got := <-received:
		require.Equal(t, message+"\n", got)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
	require.NoError(t, <-receiverErr)

	_ = senderClient.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err := bufio.NewReader(senderClient).ReadString('\n')
	require.Error(t, err)
}

func TestSendProgressUpdateReturnsDialError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	err = SendProgressUpdate(address, ProgressUpdate{})
	require.Error(t, err)
}
