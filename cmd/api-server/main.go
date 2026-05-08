package main

import (
	"database/sql"
	"os"

	"github.com/gin-gonic/gin"
	"project/internal/auth"
	"project/internal/manga"
	"project/internal/user"
	"project/internal/ws"
	"project/pkg/database"
)

type APIServer struct {
	Router      *gin.Engine
	Database    *sql.DB
	JWTSecret   string
	AuthService *auth.Service
	ChatHub     *ws.ChatHub
}

func NewAPIServer(databaseConn *sql.DB, jwtSecret string) *APIServer {
	authService := auth.NewService(databaseConn, []byte(jwtSecret))

	server := &APIServer{
		Router:      gin.Default(),
		Database:    databaseConn,
		JWTSecret:   jwtSecret,
		AuthService: authService,
		ChatHub:     ws.NewChatHub(),
	}
	go server.ChatHub.Run()
	server.registerRoutes()
	return server
}

func (s *APIServer) registerRoutes() {
	auth.RegisterHTTPRoutes(s.Router, s.AuthService)
	manga.RegisterHTTPRoutes(s.Router, s.Database)

	tcpServerAddress := os.Getenv("TCP_SERVER_ADDR")
	if tcpServerAddress == "" {
		tcpServerAddress = "localhost:9000"
	}

	users := s.Router.Group("/users")
	authMiddleware := auth.AuthMiddleware(s.AuthService)
	users.Use(authMiddleware)
	user.RegisterHTTPRoutes(users, s.Database, tcpServerAddress)
	ws.RegisterHTTPRoutes(s.Router, authMiddleware, s.ChatHub)
}

func main() {
	var err error
	var db *sql.DB

	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "data/mangahub.db"
	}

	db, err = database.OpenSQLite(dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	server := NewAPIServer(db, "secret-key")

	err = server.AuthService.EnsureSchema()
	if err != nil {
		panic(err)
	}

	err = manga.SeedFromJSON(db, "data/manga_seed.json")
	if err != nil {
		panic(err)
	}

	err = server.Router.Run(":8080")
	if err != nil {
		panic(err)
	}
}
