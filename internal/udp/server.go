package udp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"mangahub/internal/config"
	"mangahub/internal/models"
	"mangahub/internal/services"
)

const maxSendFailures = 3

type packetConn interface {
	ReadFrom(p []byte) (n int, addr net.Addr, err error)
	WriteTo(p []byte, addr net.Addr) (n int, err error)
	Close() error
}

type client struct {
	addr         net.Addr
	sendFailures int
}

type Server struct {
	cfg           config.Config
	notifications *services.NotificationService

	conn packetConn

	mu      sync.RWMutex
	clients map[string]*client
}

type clientMessage struct {
	Type string `json:"type"`
}

func NewServer(cfg config.Config, notifications *services.NotificationService) *Server {
	return &Server{
		cfg:           cfg,
		notifications: notifications,
		clients:       make(map[string]*client),
	}
}

func (s *Server) Start(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", ":"+s.cfg.UDPPort)
	if err != nil {
		return fmt.Errorf("failed to start udp listener: %w", err)
	}
	s.conn = conn

	log.Printf("udp server listening on :%s", s.cfg.UDPPort)

	notificationCh, unsubscribe := s.notifications.Subscribe(32)
	defer unsubscribe()

	go func() {
		<-ctx.Done()
		if s.conn != nil {
			_ = s.conn.Close()
		}
	}()

	go s.broadcastLoop(notificationCh)

	buffer := make([]byte, 2048)
	for {
		n, addr, err := s.conn.ReadFrom(buffer)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("udp read error: %v", err)
			continue
		}

		s.handlePacket(buffer[:n], addr)
	}
}

func (s *Server) handlePacket(packet []byte, addr net.Addr) {
	var msg clientMessage
	if err := json.Unmarshal(packet, &msg); err != nil {
		log.Printf("udp invalid json from %s: %v", addr.String(), err)
		return
	}

	switch msg.Type {
	case "register":
		s.registerClient(addr)
		s.writeJSON(addr, map[string]any{
			"type":      "registered",
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	default:
		log.Printf("udp unsupported message type from %s: %s", addr.String(), msg.Type)
	}
}

func (s *Server) broadcastLoop(notificationCh <-chan models.Notification) {
	for notification := range notificationCh {
		s.broadcast(notification)
	}
}

func (s *Server) registerClient(addr net.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[addr.String()] = &client{addr: addr}
	log.Printf("udp client registered: %s", addr.String())
}

func (s *Server) broadcast(notification models.Notification) {
	payload, err := json.Marshal(notification)
	if err != nil {
		log.Printf("udp broadcast marshal error: %v", err)
		return
	}

	s.mu.RLock()
	clients := make([]*client, 0, len(s.clients))
	for _, registered := range s.clients {
		clients = append(clients, registered)
	}
	s.mu.RUnlock()

	for _, registered := range clients {
		if err := s.writeJSON(registered.addr, payload); err != nil {
			log.Printf("udp broadcast error to %s: %v", registered.addr.String(), err)
			s.recordSendFailure(registered.addr.String())
			continue
		}

		s.resetSendFailure(registered.addr.String())
	}
}

func (s *Server) writeJSON(addr net.Addr, payload any) error {
	var raw []byte
	switch value := payload.(type) {
	case []byte:
		raw = value
	default:
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = encoded
	}

	if _, err := s.conn.WriteTo(raw, addr); err != nil {
		return err
	}

	return nil
}

func (s *Server) recordSendFailure(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	registered, ok := s.clients[addr]
	if !ok {
		return
	}

	registered.sendFailures++
	if registered.sendFailures >= maxSendFailures {
		delete(s.clients, addr)
	}
}

func (s *Server) resetSendFailure(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	registered, ok := s.clients[addr]
	if !ok {
		return
	}

	registered.sendFailures = 0
}
