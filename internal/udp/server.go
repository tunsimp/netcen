package udp

import (
	"encoding/json"
	"fmt"
	"net"
)

type NotificationServer struct {
	Port    string
	clients map[string]*net.UDPAddr
}

func NewNotificationServer(port string) *NotificationServer {
	return &NotificationServer{
		Port:    port,
		clients: make(map[string]*net.UDPAddr),
	}
}

func (s *NotificationServer) Run() error {
	addr, err := net.ResolveUDPAddr("udp", s.Port)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Println("UDP server is listening on port", s.Port)

	buf := make([]byte, 2048)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}

		msg := string(buf[:n])
		fmt.Printf("Received from %s: %s\n", clientAddr.String(), msg)

		if msg == "register" {
			s.clients[clientAddr.String()] = clientAddr
			fmt.Println("Registered client:", clientAddr.String())

			_, err = conn.WriteToUDP([]byte("registered"), clientAddr)
			if err != nil {
				fmt.Println("Register reply error:", err)
			}

			continue
		}

		var notification Notification
		err = json.Unmarshal([]byte(msg), &notification)
		if err != nil {
			fmt.Println("Invalid notification format:", err)
			continue
		}

		fmt.Println("Received notification:", notification)
		s.broadcast(conn, msg, clientAddr)
	}
}

func (s *NotificationServer) broadcast(conn *net.UDPConn, message string, sender *net.UDPAddr) {
	for _, client := range s.clients {
		if client.String() == sender.String() {
			continue
		}

		_, err := conn.WriteToUDP([]byte(message), client)
		if err != nil {
			fmt.Println("Broadcast error:", err)
		}
	}
}

func RunServer(port string) error {
	server := NewNotificationServer(port)
	return server.Run()
}
