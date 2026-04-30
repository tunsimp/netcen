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

	"mangahub/internal/config"
	"mangahub/internal/models"
	"mangahub/internal/services"
)

type Server struct {
	cfg      config.Config
	progress *services.ProgressService

	listener net.Listener

	mu      sync.RWMutex
	clients map[string]net.Conn
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

func NewServer(cfg config.Config, progress *services.ProgressService) *Server {
	return &Server{
		cfg:      cfg,
		progress: progress,
		clients:  make(map[string]net.Conn),
	}
}

func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", ":"+s.cfg.TCPPort)
	if err != nil {
		return fmt.Errorf("failed to start tcp listener: %w", err)
	}
	s.listener = listener

	log.Printf("tcp server listening on :%s", s.cfg.TCPPort)

	progressCh, unsubscribe := s.progress.Subscribe(32)
	defer unsubscribe()

	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
		s.closeAllClients()
	}()

	go s.broadcastLoop(progressCh)

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
			if strings.TrimSpace(msg.UserID) == "" || strings.TrimSpace(msg.MangaID) == "" || msg.Chapter < 1 || strings.TrimSpace(msg.Status) == "" {
				s.writeJSON(conn, map[string]any{"type": "error", "error": "invalid progress_update payload"})
				continue
			}

			if _, err := s.progress.Upsert(msg.UserID, msg.MangaID, msg.Chapter, msg.Status, msg.Timestamp); err != nil {
				log.Printf("tcp progress update rejected from %s: %v", conn.RemoteAddr().String(), err)
				s.writeJSON(conn, map[string]any{"type": "error", "error": err.Error()})
				continue
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

func (s *Server) broadcastLoop(progressCh <-chan models.ProgressUpdate) {
	for update := range progressCh {
		s.broadcast(update)
	}
}

func (s *Server) broadcast(update models.ProgressUpdate) {
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
