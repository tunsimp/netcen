package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"mangahub/internal/auth"
	"mangahub/internal/config"
	"mangahub/internal/models"
)

type Server struct {
	cfg config.Config
	jwt *auth.Manager

	mu      sync.RWMutex
	clients map[*websocket.Conn]clientIdentity

	httpServer *http.Server
}

type clientIdentity struct {
	userID   string
	username string
}

type incomingMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func NewServer(cfg config.Config, jwt *auth.Manager) *Server {
	return &Server{
		cfg:     cfg,
		jwt:     jwt,
		clients: make(map[*websocket.Conn]clientIdentity),
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:              ":" + s.cfg.WSPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
	}()

	log.Printf("websocket server listening on :%s", s.cfg.WSPort)
	err := s.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for conn := range s.clients {
		_ = conn.Close()
		delete(s.clients, conn)
	}
	s.mu.Unlock()

	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	userID, username, err := s.jwt.ParseToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	s.mu.Lock()
	s.clients[conn] = clientIdentity{
		userID:   userID,
		username: username,
	}
	s.mu.Unlock()

	log.Printf("websocket client connected: %s (%s)", username, userID)
	defer s.disconnect(conn)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			var closeErr *websocket.CloseError
			if errors.As(err, &closeErr) {
				return
			}
			log.Printf("websocket read error: %v", err)
			return
		}

		var incoming incomingMessage
		if err := json.Unmarshal(payload, &incoming); err != nil {
			s.writeMessage(conn, map[string]any{"type": "error", "error": "invalid json"})
			continue
		}

		if incoming.Type != "chat" {
			s.writeMessage(conn, map[string]any{"type": "error", "error": "unsupported message type"})
			continue
		}

		message := strings.TrimSpace(incoming.Message)
		if message == "" {
			s.writeMessage(conn, map[string]any{"type": "error", "error": "message is required"})
			continue
		}

		s.broadcast(models.ChatMessage{
			Type:      "chat",
			UserID:    userID,
			Username:  username,
			Message:   message,
			Timestamp: time.Now().Unix(),
		})
	}
}

func (s *Server) broadcast(message models.ChatMessage) {
	s.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for conn := range s.clients {
		clients = append(clients, conn)
	}
	s.mu.RUnlock()

	for _, conn := range clients {
		if err := s.writeMessage(conn, message); err != nil {
			log.Printf("websocket broadcast error: %v", err)
			s.disconnect(conn)
		}
	}
}

func (s *Server) writeMessage(conn *websocket.Conn, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal websocket payload: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		return fmt.Errorf("failed to write websocket message: %w", err)
	}
	return nil
}

func (s *Server) disconnect(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	_ = conn.Close()
}
