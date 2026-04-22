package main

import (
	"log"

	"mangahub/internal/config"
	"mangahub/internal/database"
	httprouter "mangahub/internal/http/router"
)

func main() {
	cfg := config.Load()

	db, err := database.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()

	server := httprouter.NewServer(cfg, db)
	if err := server.Run(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
