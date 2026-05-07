package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type ProgressHub struct {
	Port    string
	clients map[net.Conn]bool
	mu      sync.Mutex
}

func NewProgressHub(port string) *ProgressHub {
	return &ProgressHub{
		Port:    port,
		clients: make(map[net.Conn]bool),
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
	h.mu.Lock()
	h.clients[conn] = true
	clientCount := len(h.clients)
	h.mu.Unlock()

	fmt.Println("TCP client connected:", conn.RemoteAddr())
	fmt.Println("Online TCP clients:", clientCount)

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		clientCount := len(h.clients)
		h.mu.Unlock()

		conn.Close()
		fmt.Println("TCP client disconnected:", conn.RemoteAddr())
		fmt.Println("Online TCP clients:", clientCount)
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()

		var update ProgressUpdate
		err := json.Unmarshal([]byte(msg), &update)
		if err != nil {
			fmt.Println("Invalid TCP progress JSON:", err)
			continue
		}

		fmt.Printf("Received TCP progress: %+v\n", update)
		h.broadcast(msg, conn)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("TCP read error:", err)
	}
}

func (h *ProgressHub) broadcast(message string, sender net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients {
		if conn == sender {
			continue
		}

		_, err := fmt.Fprintln(conn, message)
		if err != nil {
			fmt.Println("TCP broadcast error:", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

func RunServer(port string) error {
	hub := NewProgressHub(port)
	return hub.Run()
}
