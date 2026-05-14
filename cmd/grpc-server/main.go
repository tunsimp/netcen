package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	grpcserver "project/internal/grpc"
	mangapb "project/internal/grpc/gen"
	"project/pkg/database"
)

func main() {
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "data/mangahub.db"
	}

	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := database.EnsureSchema(db); err != nil {
		log.Fatalf("failed to ensure database schema: %v", err)
	}

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	mangapb.RegisterMangaServiceServer(grpcServer, grpcserver.NewMangaServiceServer(db))
	reflection.Register(grpcServer)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
