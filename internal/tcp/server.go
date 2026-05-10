package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

type ProgressHub struct {
	Port        string
	clients     map[string]map[DeviceID]net.Conn
	lastUpdates map[string]map[string]int64 // UserID -> MangaID -> Timestamp
	mu          sync.Mutex
	jwtSecret   []byte
}

func NewProgressHub(port string) *ProgressHub {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		fmt.Println("WARNING: JWT_SECRET environment variable not set. JWT validation will fail.")
	}

	return &ProgressHub{
		Port:        port,
		clients:     make(map[string]map[DeviceID]net.Conn),
		lastUpdates: make(map[string]map[string]int64),
		jwtSecret:   []byte(secret),
	}
}

func (h *ProgressHub) Run() error {
	listener, err := net.Listen("tcp", h.Port)
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Println("TCP Progress Hub listening on", h.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("TCP accept error:", err)
			continue
		}

		go h.handleClient(conn)
	}
}

func (h *ProgressHub) handleClient(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	
	// Read registration message first
	if !scanner.Scan() {
		conn.Close()
		return
	}
	
	var reg DeviceRegistration
	if err := json.Unmarshal([]byte(scanner.Text()), &reg); err != nil {
		fmt.Println("Invalid registration message:", err)
		conn.Close()
		return
	}
	
	userID, err := h.validateToken(reg.Token)
	if err != nil {
		fmt.Println("Invalid token:", err)
		conn.Close()
		return
	}
	
	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[DeviceID]net.Conn)
	}
	h.clients[userID][reg.DeviceID] = conn
	clientCount := 0
	for _, devs := range h.clients {
		clientCount += len(devs)
	}
	h.mu.Unlock()

	fmt.Printf("TCP client registered: User=%s, Device=%s, Addr=%s\n", userID, reg.DeviceID, conn.RemoteAddr())
	fmt.Println("Online TCP clients:", clientCount)

	defer func() {
		h.mu.Lock()
		if devices, ok := h.clients[userID]; ok {
			delete(devices, reg.DeviceID)
			if len(devices) == 0 {
				delete(h.clients, userID)
			}
		}
		clientCount := 0
		for _, devs := range h.clients {
			clientCount += len(devs)
		}
		h.mu.Unlock()

		conn.Close()
		fmt.Printf("TCP client disconnected: User=%s, Device=%s\n", userID, reg.DeviceID)
		fmt.Println("Online TCP clients:", clientCount)
	}()

	for scanner.Scan() {
		msgText := scanner.Text()

		var msg ProgressSyncMessage
		if err := json.Unmarshal([]byte(msgText), &msg); err != nil {
			fmt.Println("Invalid TCP progress JSON:", err)
			continue
		}

		// Prevent spoofing
		if msg.UserID != userID || msg.DeviceID != reg.DeviceID {
			fmt.Printf("Spoofing detected: expected User=%s Device=%s, got User=%s Device=%s\n", 
				userID, reg.DeviceID, msg.UserID, msg.DeviceID)
			continue
		}

		// Conflict resolution: last write wins
		h.mu.Lock()
		if h.lastUpdates[userID] == nil {
			h.lastUpdates[userID] = make(map[string]int64)
		}
		lastTimestamp := h.lastUpdates[userID][msg.MangaID]
		
		if msg.Timestamp < lastTimestamp {
			// Stale update
			msg.ConflictResolution = IgnoredStaleUpdate
			response, _ := json.Marshal(msg)
			fmt.Fprintln(conn, string(response))
			h.mu.Unlock()
			continue
		}
		
		h.lastUpdates[userID][msg.MangaID] = msg.Timestamp
		msg.ConflictResolution = AcceptedLastWriteWins
		h.mu.Unlock()

		fmt.Printf("Received TCP progress: %+v\n", msg)
		
		// Broadcast to other devices of the SAME user
		responseStr, _ := json.Marshal(msg)
		h.broadcastToUser(userID, reg.DeviceID, string(responseStr))
		
		// Send confirmation to sender
		fmt.Fprintln(conn, string(responseStr))
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("TCP read error:", err)
	}
}

func (h *ProgressHub) broadcastToUser(userID string, senderDevice DeviceID, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	devices, ok := h.clients[userID]
	if !ok {
		return
	}

	for deviceID, conn := range devices {
		if deviceID == senderDevice {
			continue
		}

		_, err := fmt.Fprintln(conn, message)
		if err != nil {
			fmt.Println("TCP broadcast error:", err)
			conn.Close()
			delete(devices, deviceID)
		}
	}
}

func (h *ProgressHub) validateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return h.jwtSecret, nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}

	userID, _ := claims["uid"].(string)
	if userID == "" {
		return "", fmt.Errorf("uid claim missing")
	}

	return userID, nil
}

func RunServer(port string) error {
	hub := NewProgressHub(port)
	return hub.Run()
}
