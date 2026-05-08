# MangaHub - Net-centric Programming Project

MangaHub is a multi-protocol manga tracking system for IT096IU (Net-centric Programming).  
It combines HTTP REST, TCP, UDP, WebSocket, and gRPC services with a shared SQLite database.

## Features

- HTTP API for auth, manga search/detail, library, and reading progress
- TCP server for progress synchronization broadcasts
- UDP server for lightweight notification broadcasts
- WebSocket chat for real-time manga discussion
- gRPC internal service for manga query/update operations
- SQLite persistence with schema initialization and manga seed data
- Docker Compose for multi-service local deployment

## Project Structure

```text
mangahub/
├── cmd/
│   ├── api-server/main.go
│   ├── tcp-server/main.go
│   ├── udp-server/main.go
│   ├── grpc-server/main.go
│   └── grpc-client/client.go
├── internal/
│   ├── auth/
│   ├── manga/
│   ├── user/
│   ├── tcp/
│   ├── udp/
│   ├── ws/
│   └── grpc/
├── pkg/
│   ├── models/
│   └── database/
├── proto/
├── data/
├── Dockerfile
└── docker-compose.yml
```

## Ports

- HTTP API: `8080`
- TCP Sync: `9000`
- UDP Notification: `9001/udp`
- gRPC: `9090`

## Prerequisites

- Go 1.25+
- Docker + Docker Compose (optional, for containerized run)
- `protoc` + plugins (for regenerating protobuf, optional)
  - `protoc-gen-go`
  - `protoc-gen-go-grpc`

## Run Locally (without Docker)

Start each server in separate terminals from project root:

```bash
go run ./cmd/tcp-server/main.go
go run ./cmd/udp-server/main.go
go run ./cmd/grpc-server/main.go
go run ./cmd/api-server/main.go
```

## Run with Docker Compose

```bash
docker compose up --build
```

This starts:
- `api-server` on `8080`
- `tcp-server` on `9000`
- `udp-server` on `9001/udp`
- `grpc-server` on `9090`

## Database

- SQLite path defaults to `data/mangahub.db`
- API server initializes schema on startup
- API server seeds manga from `data/manga_seed.json` on startup

Environment variables:
- `SQLITE_PATH` (optional)
- `TCP_SERVER_ADDR` for API to forward progress updates (default `localhost:9000`)

## HTTP API

Base URL: `http://localhost:8080`

### Auth

- `POST /auth/register`
```json
{
  "username": "alice",
  "password": "123456"
}
```

- `POST /auth/login`
```json
{
  "username": "alice",
  "password": "123456"
}
```

Response:
```json
{
  "token": "<jwt>"
}
```

### Manga

- `GET /manga?title=piece&status=ongoing&genre=action`
- `GET /manga/{id}`

### User Library / Progress (JWT required)

Header:
```text
Authorization: Bearer <jwt>
```

- `POST /users/library`
```json
{
  "manga_id": "one-piece"
}
```

- `GET /users/library`

- `PUT /users/progress`
```json
{
  "manga_id": "one-piece",
  "current_chapter": 10,
  "status": "reading"
}
```

## WebSocket Chat

Endpoint:
- `ws://localhost:8080/ws/chat`

Required header:
- `Authorization: Bearer <jwt>`

Send message:
```json
{
  "message": "Hello everyone"
}
```

Receive broadcast format:
```json
{
  "user_id": "user-id",
  "username": "alice",
  "message": "Hello everyone",
  "timestamp": 1715140000
}
```

## gRPC

Service:
- `mangahub.v1.MangaService`
  - `GetManga`
  - `SearchManga`
  - `UpdateProgress`

### Run gRPC client demo

```bash
go run ./cmd/grpc-client/client.go
```

### grpcurl examples

List services:
```bash
grpcurl -plaintext localhost:9090 list
```

Get manga:
```bash
grpcurl -plaintext -d '{"id":"one-piece"}' localhost:9090 mangahub.v1.MangaService/GetManga
```

Search manga:
```bash
grpcurl -plaintext -d '{"keyword":"chain"}' localhost:9090 mangahub.v1.MangaService/SearchManga
```

Update progress:
```bash
grpcurl -plaintext -d '{"user_id":"1","manga_id":"one-piece","current_chapter":5,"status":"reading"}' localhost:9090 mangahub.v1.MangaService/UpdateProgress
```

## Protobuf Regeneration (optional)

```bash
protoc --go_out=. --go_opt=module=project --go-grpc_out=. --go-grpc_opt=module=project proto/service.proto
```

## Troubleshooting

- If `protoc` is not recognized on Windows, add protoc `bin` directory to PATH.
- If `grpcurl` is not recognized after install, open a new shell or call full executable path.
- If API startup fails with database file error, ensure write permission for `data/`.
- If WebSocket auth fails, verify `Authorization: Bearer <jwt>` header is included.

