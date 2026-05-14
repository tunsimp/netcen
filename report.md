# MangaHub Implementation Report

-- _Student 1: Nguyen Quoc Tuan. ID: ITITIU22177_
-- _Student 2: Nguyen Duc Bao. ID: ITITIU22238_

## 1) Overview

This report summarizes the implementation flow of the MangaHub project:

1. HTTP Authentication and Basic Database setup
2. HTTP APIs & TCP Multi-Device Synchronization (Advanced)
3. UDP Notification Server
4. WebSocket Real-time Chat
5. gRPC Internal Service
6. Run instructions and Demo checklist

---

## 2) Phase 1: HTTP + Auth Service + Basic Database

I started by building the HTTP layer and authentication flow first.

### What was implemented

- Gin-based HTTP server (`cmd/api-server/main.go`)
- SQLite connection helper (`pkg/database/sqlite.go`)
- Auth service (`internal/auth/service.go`) with:
  - user registration
  - user login
  - JWT generation and parsing
  - schema initialization
- User model (`pkg/models/user.go`)

### Database initialization

At server startup, the app:

- opens SQLite database
- ensures required tables exist (`users`, `manga`, `user_progress`)
- seeds manga data from `data/manga_seed.json`

This created a stable base for all later protocol features.

---

## 3) Phase 2: HTTP APIs & TCP Multi-Device Sync

With auth and DB stable, I implemented the main CRUD APIs and the advanced TCP synchronization server. The HTTP API is tightly integrated with the TCP server to broadcast progress updates.

### 1. HTTP Endpoints

- **Manga:** `GET /manga` (search/filter), `GET /manga/:id` (detail)
- **User (JWT-protected):** `POST /users/library` (add), `GET /users/library` (list), `PUT /users/progress` (update reading progress)

_Notes:_ Input validation is applied for required fields. JSON request/response format is used consistently.

### 2. TCP Synchronization Server (Advanced)

I implemented an advanced synchronization feature for the TCP module (`internal/tcp/server.go`).

- **TCP Device Registration:** Clients connect to `9000` and must register with a valid JWT and a unique `DeviceID`.
- **Smart Routing:** TCP progress updates are only broadcast to _other_ registered devices of the same user, rather than all connected TCP clients.
- **Spoofing Protection:** The server verifies that incoming `ProgressSyncMessage` matches the registered `UserID` and `DeviceID` to prevent users from spoofing data for other users or devices.
- **Conflict Resolution:** A basic "last-write-wins" strategy is implemented by tracking the latest progress update timestamp per user and manga. Stale updates return `ignored_stale_update`, while successful ones return `accepted_last_write_wins`.
- **HTTP Integration:** The `PUT /users/progress` HTTP API endpoint automatically forwards an `X-Device-ID` header (defaulting to `http-api`) and the JWT token to the TCP server, making HTTP updates seamlessly trigger TCP syncs to mobile or desktop clients.

---

## 4) Phase 3: UDP Notification Server

Implemented a lightweight UDP server for broadcasting non-critical notifications.

### Components

- UDP server in `internal/udp/server.go`.
- Server executable in `cmd/udp-server/main.go`.

### Behavior

- Connectionless model listening on port `9001/udp`.
- Clients send a raw `"register"` string to subscribe.
- Incoming JSON payloads are immediately broadcast to all registered UDP clients (excluding the sender).
- Ideal for fast, "fire-and-forget" announcements like new chapter releases.

---

## 5) Phase 4: WebSocket Chat

Next, I implemented real-time chat for manga discussion.

### Components

- Chat hub in `internal/ws/http.go` (`Clients`, `Broadcast`, `Register`, `Unregister`).
- WebSocket endpoint: `GET /ws/chat`

### Behavior

- JWT-based user identity is reused from auth middleware.
- User join/leave messages are broadcast to connected clients.
- Basic connection cleanup is handled on disconnect.

### Client Testing

- Added a command-line WebSocket client in `cmd/ws-client/main.go`.
- Allows users to connect via terminal, providing the JWT token as an argument.
- Users can send messages via `stdin` and receive broadcast messages concurrently.

---

## 6) Phase 5: gRPC Internal Service

After the external-facing protocols were complete, I implemented gRPC support for internal service-to-service communication.

### Proto definition

- `proto/service.proto`
- `MangaService` with unary RPC methods: `GetManga`, `SearchManga`, `UpdateProgress`

### gRPC server & client

- Server: `cmd/grpc-server/main.go` (listening on `9090`, reflection enabled)
- Client demo: `cmd/grpc-client/client.go` (calls all three RPC methods)

---

## 7) Testing

I added `testify`-based tests for key protocol areas:

- HTTP auth tests (`internal/auth/http_test.go`)
- WebSocket broadcast test (`internal/ws/http_test.go`)
- gRPC service tests (`internal/grpc/server_test.go`)

All tests run successfully with `go test ./...`.

---

## 8) Deployment Files Added

To support deployment and grading requirements, I added:

- `Dockerfile`: Multi-stage build with configurable target binary (`TARGET`).
- `docker-compose.yml`: Starts `api-server`, `tcp-server`, `udp-server`, `grpc-server`, and a shared volume for SQLite.

---

## 9) How To Run

### A) Run locally (separate terminals)

From project root:

```bash
go run ./cmd/tcp-server/main.go
go run ./cmd/udp-server/main.go
go run ./cmd/grpc-server/main.go
go run ./cmd/api-server/main.go
```

Ports: HTTP (`8080`), TCP (`9000`), UDP (`9001/udp`), gRPC (`9090`)

### B) Run with Docker Compose

```bash
docker compose up --build
```

---

## 10) Demo Checklist

1. Register and login via HTTP (`/auth/register`, `/auth/login`)
2. Call manga APIs (`/manga`, `/manga/:id`)
3. Use JWT on user APIs (`/users/library`, `/users/progress`). Notice that progress updates are automatically synced via TCP.
4. Connect 2 WebSocket clients to `/ws/chat` using `go run ./cmd/ws-client/main.go -token <jwt>` and exchange messages.
5. Run gRPC client (`cmd/grpc-client/client.go`) to call all RPC methods.

---

## 11) Conclusion

The project is implemented in modular phases, starting from core HTTP/auth/database and extending to multi-protocol communication (TCP, UDP, WebSocket, gRPC).  
This structure supports both course demonstration requirements and further extension.
