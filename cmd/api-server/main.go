package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"mangahub/internal/auth"
	"mangahub/internal/config"
	"mangahub/internal/database"
	"mangahub/internal/http/handlers"
	"mangahub/internal/http/middleware"
	httprouter "mangahub/internal/http/router"
	"mangahub/internal/repository"
	"mangahub/internal/tcp"
)

func main() {
	cfg := config.Load()

	db, err := database.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)

	jwtManager := auth.NewManager(cfg.JWTSecret)

	authHandler := handlers.NewAuthHandler(userRepo, jwtManager)

	httpServer := httprouter.NewServer(
		cfg,
		authHandler,
		middleware.RequireAuth(jwtManager),
	)
	tcpServer := tcp.NewServer(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		errCh <- httpServer.Start()
	}()
	go func() {
		errCh <- tcpServer.Start(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server stopped: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
}
