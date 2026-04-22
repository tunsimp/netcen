package router

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"mangahub/internal/auth"
	"mangahub/internal/config"
	"mangahub/internal/http/handlers"
	"mangahub/internal/http/middleware"
	"mangahub/internal/repository"
)

type Server struct {
	cfg    config.Config
	engine *gin.Engine
}

func NewServer(cfg config.Config, db *sql.DB) *Server {
	engine := gin.Default()

	userRepo := repository.NewUserRepository(db)
	jwtManager := auth.NewManager(cfg.JWTSecret)
	authHandler := handlers.NewAuthHandler(userRepo, jwtManager)

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	engine.POST("/auth/register", authHandler.Register)
	engine.POST("/auth/login", authHandler.Login)

	protected := engine.Group("/")
	protected.Use(middleware.RequireAuth(jwtManager))
	protected.GET("/auth/me", authHandler.Me)

	return &Server{
		cfg:    cfg,
		engine: engine,
	}
}

func (s *Server) Run() error {
	return s.engine.Run(fmt.Sprintf(":%s", s.cfg.HTTPPort))
}
