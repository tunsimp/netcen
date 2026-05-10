package tcp

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func generateTestToken(userID string, secret []byte) string {
	claims := jwt.MapClaims{
		"uid": userID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, _ := token.SignedString(secret)
	return str
}

func TestTCPMultiDeviceSync(t *testing.T) {
	os.Setenv("JWT_SECRET", "test_secret")
	hub := NewProgressHub("127.0.0.1:0")
	
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	hub.Port = listener.Addr().String()
	
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go hub.handleClient(conn)
		}
	}()
	defer listener.Close()

	tokenU1 := generateTestToken("u1", []byte("test_secret"))
	tokenU2 := generateTestToken("u2", []byte("test_secret"))

	// Connect Client 1 (User 1, Device 1)
	c1, err := net.Dial("tcp", hub.Port)
	require.NoError(t, err)
	defer c1.Close()
	
	reg1, _ := json.Marshal(DeviceRegistration{Token: tokenU1, DeviceID: "dev1"})
	c1.Write(append(reg1, '\n'))

	// Connect Client 2 (User 1, Device 2)
	c2, err := net.Dial("tcp", hub.Port)
	require.NoError(t, err)
	defer c2.Close()
	
	reg2, _ := json.Marshal(DeviceRegistration{Token: tokenU1, DeviceID: "dev2"})
	c2.Write(append(reg2, '\n'))

	// Connect Client 3 (User 2, Device 3)
	c3, err := net.Dial("tcp", hub.Port)
	require.NoError(t, err)
	defer c3.Close()
	
	reg3, _ := json.Marshal(DeviceRegistration{Token: tokenU2, DeviceID: "dev3"})
	c3.Write(append(reg3, '\n'))

	time.Sleep(100 * time.Millisecond) // Wait for registration to complete

	// Test case 1: Send update from Client 1, Client 2 should receive it, Client 3 should not.
	msg1 := ProgressSyncMessage{
		ProgressUpdate: ProgressUpdate{
			UserID:    "u1",
			MangaID:   "m1",
			Chapter:   1,
			Timestamp: 100,
		},
		DeviceID: "dev1",
	}
	msg1Bytes, _ := json.Marshal(msg1)
	c1.Write(append(msg1Bytes, '\n'))

	c2Scanner := bufio.NewScanner(c2)
	require.True(t, c2Scanner.Scan())
	var receivedMsg ProgressSyncMessage
	require.NoError(t, json.Unmarshal(c2Scanner.Bytes(), &receivedMsg))
	require.Equal(t, AcceptedLastWriteWins, receivedMsg.ConflictResolution)
	require.Equal(t, "dev1", string(receivedMsg.DeviceID))

	// Check Client 1 also receives the confirmation
	c1Scanner := bufio.NewScanner(c1)
	require.True(t, c1Scanner.Scan())
	var confirmMsg ProgressSyncMessage
	require.NoError(t, json.Unmarshal(c1Scanner.Bytes(), &confirmMsg))
	require.Equal(t, AcceptedLastWriteWins, confirmMsg.ConflictResolution)

	// Test case 2: Spoofed update from Client 1 (trying to act as User 2)
	msgSpoof := ProgressSyncMessage{
		ProgressUpdate: ProgressUpdate{
			UserID:    "u2",
			MangaID:   "m1",
			Chapter:   2,
			Timestamp: 200,
		},
		DeviceID: "dev1",
	}
	msgSpoofBytes, _ := json.Marshal(msgSpoof)
	c1.Write(append(msgSpoofBytes, '\n'))

	// Valid message right after spoofed message to ensure server processes it
	msg2 := ProgressSyncMessage{
		ProgressUpdate: ProgressUpdate{
			UserID:    "u1",
			MangaID:   "m1",
			Chapter:   2,
			Timestamp: 200,
		},
		DeviceID: "dev1",
	}
	msg2Bytes, _ := json.Marshal(msg2)
	c1.Write(append(msg2Bytes, '\n'))

	require.True(t, c1Scanner.Scan())
	var confirmMsg2 ProgressSyncMessage
	require.NoError(t, json.Unmarshal(c1Scanner.Bytes(), &confirmMsg2))
	require.Equal(t, 2, confirmMsg2.Chapter) // The valid one was processed

	// Test case 3: Stale update
	msgStale := ProgressSyncMessage{
		ProgressUpdate: ProgressUpdate{
			UserID:    "u1",
			MangaID:   "m1",
			Chapter:   0,
			Timestamp: 50, // older than 200
		},
		DeviceID: "dev1",
	}
	msgStaleBytes, _ := json.Marshal(msgStale)
	c1.Write(append(msgStaleBytes, '\n'))

	require.True(t, c1Scanner.Scan())
	var staleMsg ProgressSyncMessage
	require.NoError(t, json.Unmarshal(c1Scanner.Bytes(), &staleMsg))
	require.Equal(t, IgnoredStaleUpdate, staleMsg.ConflictResolution)
}
