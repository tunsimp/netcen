package ws

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type ChatMessage struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type ClientConnection struct {
	Conn     *websocket.Conn
	UserID   string
	Username string
}

type ChatHub struct {
	Clients    map[*websocket.Conn]string
	Broadcast  chan ChatMessage
	Register   chan ClientConnection
	Unregister chan *websocket.Conn
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		Clients:    make(map[*websocket.Conn]string),
		Broadcast:  make(chan ChatMessage),
		Register:   make(chan ClientConnection),
		Unregister: make(chan *websocket.Conn),
	}
}

func (h *ChatHub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client.Conn] = client.Username
			h.sendToAll(ChatMessage{
				UserID:    client.UserID,
				Username:  "system",
				Message:   client.Username + " joined the chat",
				Timestamp: time.Now().Unix(),
			})
		case conn := <-h.Unregister:
			username, exists := h.Clients[conn]
			if exists {
				delete(h.Clients, conn)
				_ = conn.Close()
				h.sendToAll(ChatMessage{
					UserID:    "",
					Username:  "system",
					Message:   username + " left the chat",
					Timestamp: time.Now().Unix(),
				})
			}
		case message := <-h.Broadcast:
			h.sendToAll(message)
		}
	}
}

func (h *ChatHub) sendToAll(message ChatMessage) {
	for conn := range h.Clients {
		err := conn.WriteJSON(message)
		if err != nil {
			_ = conn.Close()
			delete(h.Clients, conn)
		}
	}
}

func RegisterHTTPRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc, hub *ChatHub) {
	wsGroup := router.Group("/ws")
	wsGroup.Use(authMiddleware)
	wsGroup.GET("/chat", chatHandler(hub))
}

func chatHandler(hub *ChatHub) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userIDValue, userIDExists := ctx.Get("userID")
		usernameValue, usernameExists := ctx.Get("username")
		if !userIDExists || !usernameExists {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}

		userID := userIDValue.(string)
		username := usernameValue.(string)

		hub.Register <- ClientConnection{
			Conn:     conn,
			UserID:   userID,
			Username: username,
		}
		defer func() {
			hub.Unregister <- conn
		}()

		for {
			var incoming struct {
				Message string `json:"message"`
			}

			err = conn.ReadJSON(&incoming)
			if err != nil {
				break
			}
			if incoming.Message == "" {
				continue
			}

			hub.Broadcast <- ChatMessage{
				UserID:    userID,
				Username:  username,
				Message:   incoming.Message,
				Timestamp: time.Now().Unix(),
			}
		}
	}
}
