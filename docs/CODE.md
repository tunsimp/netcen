# MangaHub Code Documentation

## Architecture Overview

MangaHub is a multi-protocol system composed of:

- HTTP REST service
- TCP progress synchronization service
- UDP notification service
- WebSocket chat service
- gRPC internal service
- Shared SQLite database

Each protocol is implemented as an independent server entrypoint under `cmd/`, with reusable logic under `internal/`.

---

## Main Modules

## `cmd/`

- `cmd/api-server/main.go`
  - boots HTTP API (`:8080`)
  - initializes schema and seed data
  - registers auth, manga, user, websocket routes
- `cmd/tcp-server/main.go`
  - starts TCP server (`:9000`)
- `cmd/udp-server/main.go`
  - starts UDP server (`:9001/udp`)
- `cmd/grpc-server/main.go`
  - starts gRPC server (`:9090`)
- `cmd/grpc-client/client.go`
  - demo client for gRPC unary calls

## `internal/auth`

- `service.go`
  - schema creation (`users`, `manga`, `user_progress`)
  - register/login logic
  - password hashing (bcrypt)
  - JWT generation and token parsing
- `http.go`
  - `/auth/register`
  - `/auth/login`
  - JWT auth middleware

## `internal/manga`

- `http.go`
  - `GET /manga`
  - `GET /manga/:id`
- `seed.go`
  - seed loader from `data/manga_seed.json`
- `types.go`, `mangadex.go`
  - manga model/types and source helper logic

## `internal/user`

- `http.go`
  - `/users/library` and `/users/progress` routes
  - progress update persisted in DB
  - progress forwarded to TCP server

## `internal/ws`

- `http.go`
  - WebSocket chat hub and route
  - connection register/unregister
  - message broadcasting to all clients

## `internal/tcp`

- `server.go`
  - multi-client TCP listener
  - JSON progress update broadcast
- `client.go`
  - helper to send progress update from HTTP service

## `internal/udp`

- `server.go`
  - UDP registration and notification broadcasting
- `types.go`
  - notification payload model

## `internal/grpc`

- `server.go`
  - `MangaService` implementation
  - methods:
    - `GetManga`
    - `SearchManga`
    - `UpdateProgress`
- `gen/`
  - protobuf-generated Go code

## `pkg/database`

- `sqlite.go`
  - SQLite connection helper
  - ensures database directory exists

## `pkg/models`

- shared model structs (`User`, etc.)

---

## Data Flow Notes

## Auth flow

1. User registers via `/auth/register`
2. Password is hashed and stored
3. User logs in via `/auth/login`
4. JWT token returned
5. Protected routes use middleware to parse token and inject `userID`, `username`

## Progress sync flow

1. Client calls `PUT /users/progress`
2. API updates `user_progress` table
3. API sends JSON update to TCP sync server (`internal/tcp/client.go`)
4. TCP server broadcasts update to connected TCP clients

## Chat flow

1. Client opens `ws://.../ws/chat` with JWT
2. Middleware injects user info
3. Hub registers client and handles join/leave events
4. Incoming message is broadcast to all connected clients

## gRPC flow

1. gRPC server starts and registers `MangaService`
2. Client calls unary methods (`GetManga`, `SearchManga`, `UpdateProgress`)
3. Service reads/writes SQLite and returns protobuf responses

---

## Testing

Current `testify` test files:

- `internal/auth/http_test.go`
- `internal/ws/http_test.go`
- `internal/grpc/server_test.go`

Run:

```bash
go test ./...
```

