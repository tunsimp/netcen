package router

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mangahub/internal/config"
	"mangahub/internal/http/handlers"
)

type Server struct {
	cfg        config.Config
	engine     *gin.Engine
	httpServer *http.Server
}

func NewServer(
	cfg config.Config,
	authHandler *handlers.AuthHandler,
	authMiddleware gin.HandlerFunc,
) *Server {
	engine := gin.Default()
	engine.Use(allowCORS())

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	engine.POST("/auth/register", authHandler.Register)
	engine.POST("/auth/login", authHandler.Login)

	protected := engine.Group("/")
	protected.Use(authMiddleware)
	protected.GET("/auth/me", authHandler.Me)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Server{
		cfg:        cfg,
		engine:     engine,
		httpServer: httpServer,
	}
}

func (s *Server) Start() error {
	log.Printf("http server listening on :%s", s.cfg.HTTPPort)
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}
