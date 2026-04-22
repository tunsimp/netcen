package tcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"mangahub/internal/config"
)

type Server struct {
	cfg config.Config

	listener net.Listener

	mu      sync.RWMutex
	clients map[string]net.Conn
	updates chan ProgressUpdate
}

type ClientMessage struct {
	Type      string `json:"type"`
	ClientID  string `json:"client_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	MangaID   string `json:"manga_id,omitempty"`
	Chapter   int    `json:"chapter,omitempty"`
	Status    string `json:"status,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

type ProgressUpdate struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

func NewServer(cfg config.Config) *Server {
	return &Server{
		cfg:     cfg,
		clients: make(map[string]net.Conn),
		updates: make(chan ProgressUpdate, 32),
	}
}

func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", ":"+s.cfg.TCPPort)
	if err != nil {
		return fmt.Errorf("failed to start tcp listener: %w", err)
	}
	s.listener = listener

	log.Printf("tcp server listening on :%s", s.cfg.TCPPort)

	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
		s.closeAllClients()
		close(s.updates)
	}()

	go s.broadcastLoop()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("tcp accept error: %v", err)
			continue
		}

		clientAddr := conn.RemoteAddr().String()
		log.Printf("tcp client connected: %s", clientAddr)

		s.mu.Lock()
		s.clients[clientAddr] = conn
		s.mu.Unlock()

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		clientAddr := conn.RemoteAddr().String()
		s.mu.Lock()
		delete(s.clients, clientAddr)
		s.mu.Unlock()
		_ = conn.Close()
		log.Printf("tcp client disconnected: %s", clientAddr)
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg ClientMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			s.writeJSON(conn, map[string]any{"type": "error", "error": "invalid json"})
			continue
		}

		switch msg.Type {
		case "hello":
			s.writeJSON(conn, map[string]any{"type": "hello", "status": "connected"})
		case "progress_update":
			if msg.UserID == "" || msg.MangaID == "" || msg.Chapter < 1 || strings.TrimSpace(msg.Status) == "" {
				s.writeJSON(conn, map[string]any{"type": "error", "error": "invalid progress_update payload"})
				continue
			}

			update := ProgressUpdate{
				Type:      "progress_broadcast",
				UserID:    msg.UserID,
				MangaID:   msg.MangaID,
				Chapter:   msg.Chapter,
				Status:    msg.Status,
				Timestamp: msg.Timestamp,
			}
			if update.Timestamp == 0 {
				update.Timestamp = time.Now().Unix()
			}

			select {
			case s.updates <- update:
			default:
				log.Printf("tcp broadcast queue full, dropping update from %s", conn.RemoteAddr().String())
			}

			s.writeJSON(conn, map[string]any{"type": "ack", "status": "ok"})
		default:
			s.writeJSON(conn, map[string]any{"type": "error", "error": "unsupported message type"})
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("tcp read error from %s: %v", conn.RemoteAddr().String(), err)
	}
}

func (s *Server) broadcastLoop() {
	for update := range s.updates {
		s.broadcast(update)
	}
}

func (s *Server) broadcast(update ProgressUpdate) {
	payload, err := json.Marshal(update)
	if err != nil {
		log.Printf("tcp broadcast marshal error: %v", err)
		return
	}
	payload = append(payload, '\n')

	s.mu.RLock()
	clients := make(map[string]net.Conn, len(s.clients))
	for addr, conn := range s.clients {
		clients[addr] = conn
	}
	s.mu.RUnlock()

	for addr, conn := range clients {
		if _, err := conn.Write(payload); err != nil {
			log.Printf("tcp broadcast error to %s: %v", addr, err)
			s.mu.Lock()
			delete(s.clients, addr)
			s.mu.Unlock()
			_ = conn.Close()
		}
	}
}

func (s *Server) closeAllClients() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for addr, conn := range s.clients {
		_ = conn.Close()
		delete(s.clients, addr)
	}
}

func (s *Server) writeJSON(conn net.Conn, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	raw = append(raw, '\n')
	if _, err := conn.Write(raw); err != nil {
		log.Printf("tcp write error to %s: %v", conn.RemoteAddr().String(), err)
	}
}
