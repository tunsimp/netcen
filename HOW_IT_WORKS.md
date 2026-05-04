# MangaHub Backend - How It Works

This document explains how the current MangaHub backend works, based on the implementation in this repository.

## 1) What this app is

MangaHub is a multi-protocol backend server written in Go.  
One process starts multiple servers at the same time:

- HTTP API (Gin) for auth and health checks
- TCP server for reading progress sync/broadcast
- UDP server for notification broadcast
- WebSocket server for realtime chat
- gRPC server for internal RPC calls

All of them share the same SQLite database and service layer.

## 2) High-level architecture

At startup (`cmd/api-server/main.go`), the app:

1. Loads config from environment variables
2. Opens/initializes SQLite database
3. Creates repositories
4. Creates business services
5. Creates protocol servers (HTTP/TCP/UDP/WS/gRPC)
6. Runs all servers concurrently
7. Waits for shutdown signal and gracefully stops servers

### Layers

- **Config** (`internal/config`): env vars + defaults
- **Database** (`internal/database`): schema creation + seed manga
- **Repository** (`internal/repository`): DB access
- **Service** (`internal/services`): business validation + publish-subscribe events
- **Protocol servers** (`internal/http`, `internal/tcp`, `internal/udp`, `internal/ws`, `internal/grpc`): transport/adapters

## 3) Configuration

The app reads these environment variables:

- `HTTP_PORT` (default `8080`)
- `TCP_PORT` (default `9090`)
- `UDP_PORT` (default `9091`)
- `WS_PORT` (default `8081`)
- `GRPC_PORT` (default `50051`)
- `DB_PATH` (default `./cmd/api-server/data/mangahub.db`)
- `JWT_SECRET` (default `dev-secret-change-me`)

## 4) Database and models

On startup, the database layer auto-creates tables:

- `users`: account data
- `manga`: catalog entries
- `user_progress`: user chapter progress per manga

If `manga` is empty, seed data is inserted (currently 3 manga records: One Piece, Kingdom, Frieren).

Important models:

- `User`
- `Manga`
- `UserProgress`
- `ProgressUpdate` (for TCP broadcast)
- `Notification` (for UDP broadcast)
- `ChatMessage` (for WebSocket chat)

## 5) HTTP API behavior

HTTP server uses Gin and CORS middleware.

### Endpoints

- `GET /healthz` -> service health
- `POST /auth/register` -> create user
- `POST /auth/login` -> login and receive JWT
- `GET /auth/me` -> requires Bearer token, returns token identity

### Auth flow

1. Register hashes password using bcrypt and stores user
2. Login verifies hash and issues JWT with:
   - `sub` (user ID)
   - `username`
3. `RequireAuth` middleware parses Bearer token and injects user info into request context

## 6) TCP progress sync behavior

TCP server accepts line-delimited JSON messages.

### Supported incoming message types

- `hello`
- `progress_update`

### Example messages

```json
{"type":"hello","client_id":"cli-1"}
```

```json
{"type":"progress_update","user_id":"u1","manga_id":"one-piece","chapter":1095,"status":"reading"}
```

### Processing logic

For `progress_update`:

1. Validate payload fields
2. Call `ProgressService.Upsert(...)`
3. `ProgressService` validates domain rules:
   - non-empty `user_id` and `manga_id`
   - `chapter > 0`
   - status in `reading|completed|plan_to_read`
   - manga must exist
4. Save into `user_progress`
5. Publish a `ProgressUpdate` event
6. TCP server broadcasts that event to all connected TCP clients

### Typical server responses

- Success ack:

```json
{"type":"ack","status":"ok"}
```

- Error response:

```json
{"type":"error","error":"...reason..."}
```

## 7) UDP notification behavior

UDP server is subscriber-based.

### Client registration

A UDP client sends:

```json
{"type":"register"}
```

Server replies:

```json
{"type":"registered","status":"ok","timestamp":1710000000}
```

### Broadcast source

UDP server subscribes to `NotificationService`.  
When a notification is published, server sends payload to all registered UDP clients.

Example broadcast payload:

```json
{"type":"chapter_release","manga_id":"one-piece","message":"new chapter","timestamp":1710000000}
```

If a client fails too many sends (3 consecutive failures), it is removed.

## 8) WebSocket chat behavior

WebSocket endpoint: `/ws`

Connection requires query token:

- `ws://host:WS_PORT/ws?token=<jwt>`

### Connection lifecycle

1. Validate token
2. Upgrade HTTP -> WebSocket
3. Store connection with user identity
4. Read incoming messages and broadcast valid chat messages
5. Remove client on disconnect/error

### Client message format

```json
{"type":"chat","message":"hello everyone"}
```

### Broadcast format

```json
{"type":"chat","user_id":"u1","username":"alice","message":"hello everyone","timestamp":1710000000}
```

## 9) gRPC internal service behavior

gRPC server exposes `mangahub.InternalService` with:

- `HealthCheck`
- `PublishNotification`

Proto contract file:

- `api/proto/mangahub_internal.proto`

### Methods

- `HealthCheck`: returns `{status: "ok", timestamp: ...}`
- `PublishNotification`: validates and publishes notification through `NotificationService`

Publishing through gRPC can trigger UDP broadcasts (via shared service event subscription).

## 10) How protocols are connected

Key integration points:

- `ProgressService` is shared by TCP server + repositories
  - TCP writes progress
  - service emits progress events
  - TCP broadcasts events to connected TCP clients

- `NotificationService` is shared by gRPC + UDP server
  - gRPC publishes notification
  - service emits notification events
  - UDP server broadcasts events to registered UDP clients

- JWT auth manager is shared by HTTP auth and WebSocket auth

## 11) Current implementation scope

Implemented now:

- Full auth basics (`register/login/me`)
- TCP progress sync + persistence + broadcast
- UDP register + notification broadcast
- WebSocket chat
- gRPC internal service
- Integration tests across protocols

Not yet implemented in HTTP:

- Manga CRUD endpoints
- User library/progress HTTP endpoints

## 12) Running and testing

Run app:

```bash
go run ./cmd/api-server
```

Run tests:

```bash
go test ./...
```

Integration coverage includes protocol interaction tests (HTTP/TCP/UDP/WS/gRPC).
