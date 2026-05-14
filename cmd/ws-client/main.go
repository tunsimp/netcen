package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

type outgoingMessage struct {
	Message string `json:"message"`
}

type incomingMessage struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	wsURL := flag.String("url", "ws://localhost:8080/ws/chat", "websocket URL")
	token := flag.String("token", "", "JWT token (or set WS_TOKEN env var)")
	flag.Parse()

	if strings.TrimSpace(*token) == "" {
		*token = strings.TrimSpace(os.Getenv("WS_TOKEN"))
	}
	if strings.TrimSpace(*token) == "" {
		log.Fatal("missing token: use -token <jwt> or set WS_TOKEN")
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+strings.TrimSpace(*token))

	conn, _, err := websocket.DefaultDialer.Dial(*wsURL, headers)
	if err != nil {
		log.Fatalf("websocket connect failed: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to", *wsURL)
	fmt.Println("Type a message and press Enter. Ctrl+C to quit.")

	go func() {
		for {
			var msg incomingMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Printf("read error: %v", err)
				os.Exit(0)
			}
			fmt.Printf("[%s] %s\n", msg.Username, msg.Message)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		err = conn.WriteJSON(outgoingMessage{Message: text})
		if err != nil {
			log.Fatalf("send error: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin error: %v", err)
	}
}
