# MangaHub Implementation Report

## 1) Overview

This report summarizes the implementation flow of the MangaHub project:

1. HTTP API with authentication service and SQLite database
2. Basic CRUD-style API endpoints for manga and user library/progress
3. WebSocket real-time chat system
4. gRPC internal service
5. Run instructions for all protocols

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

## 3) Phase 2: CRUD-style HTTP APIs

After auth and DB were stable, I implemented the main API endpoints.

### Auth endpoints

- `POST /auth/register`
- `POST /auth/login`

### Manga endpoints

- `GET /manga` (search/filter)
- `GET /manga/:id` (detail)

### User endpoints (JWT-protected)

- `POST /users/library` (add manga to library)
- `GET /users/library` (get user library)
- `PUT /users/progress` (update reading progress)

### Notes

- Input validation is applied for required fields.
- JSON request/response format is used consistently.
- Basic error handling is included for malformed input and DB errors.

---

## 4) Phase 3: WebSocket Chat

Next, I implemented real-time chat for manga discussion.

### Components

- Chat hub in `internal/ws/http.go`:
  - `Clients`
  - `Broadcast`
  - `Register`
  - `Unregister`
- WebSocket endpoint:
  - `GET /ws/chat`

### Behavior

- JWT-based user identity is reused from auth middleware.
- User join/leave messages are broadcast to connected clients.
- Incoming messages are broadcast to all active clients in real time.
- Basic connection cleanup is handled on disconnect.

---

## 5) Phase 4: gRPC Internal Service

After HTTP and WebSocket were complete, I implemented gRPC support.

### Proto definition

- `proto/service.proto`
- `MangaService` with unary RPC methods:
  - `GetManga`
  - `SearchManga`
  - `UpdateProgress`

### gRPC server

- `cmd/grpc-server/main.go`
- Service implementation in `internal/grpc/server.go`
- Reflection enabled for grpcurl testing

### gRPC client demo

- `cmd/grpc-client/client.go`
- Calls all three RPC methods and prints responses

---

## 6) Testing

I added `testify`-based tests for key protocol areas:

- HTTP auth tests (`internal/auth/http_test.go`)
- WebSocket broadcast test (`internal/ws/http_test.go`)
- gRPC service tests (`internal/grpc/server_test.go`)

All tests run successfully with:

```bash
go test ./...
```

---

## 7) Deployment Files Added

To support deployment and grading requirements, I added:

- `Dockerfile`
  - multi-stage build
  - configurable target binary (`TARGET`) so one Dockerfile can build API/TCP/UDP/gRPC services
- `docker-compose.yml`
  - `api-server` on `8080`
  - `tcp-server` on `9000`
  - `udp-server` on `9001/udp`
  - `grpc-server` on `9090`
  - shared named volume for SQLite data
- `testify`-based tests:
  - `internal/auth/http_test.go`
  - `internal/ws/http_test.go`
  - `internal/grpc/server_test.go`

---

## 8) How To Run

## A) Run locally (separate terminals)

From project root:

```bash
go run ./cmd/tcp-server/main.go
go run ./cmd/udp-server/main.go
go run ./cmd/grpc-server/main.go
go run ./cmd/api-server/main.go
```

Ports:

- HTTP: `8080`
- TCP: `9000`
- UDP: `9001/udp`
- gRPC: `9090`

## B) Run with Docker Compose

```bash
docker compose up --build
```

This starts all services with the same port mapping as local mode.

---

## 9) Demo Checklist

1. Register and login via HTTP (`/auth/register`, `/auth/login`)
2. Call manga APIs (`/manga`, `/manga/:id`)
3. Use JWT on user APIs (`/users/library`, `/users/progress`)
4. Connect 2 WebSocket clients to `/ws/chat` and exchange messages
5. Run gRPC client (`cmd/grpc-client/client.go`) to call all RPC methods

---

## 10) TCP Multi-Device Sync Bonus

I added an advanced synchronization feature for the TCP module.

### Features
- **TCP Device Registration:** Clients must register with a valid JWT and a unique `DeviceID` upon connecting.
- **Smart Routing:** TCP progress updates are only broadcast to *other* registered devices of the same user, rather than all connected TCP clients.
- **Spoofing Protection:** The server verifies that incoming `ProgressSyncMessage` matches the registered `UserID` and `DeviceID` to prevent users from spoofing data for other users or devices.
- **Conflict Resolution:** A basic "last-write-wins" strategy is implemented by tracking the latest progress update timestamp per user and manga. Stale updates return `ignored_stale_update`, while successful ones return `accepted_last_write_wins`.
- **HTTP Integration:** The `/users/progress` endpoint now forwards an `X-Device-ID` header (defaulting to `http-api`) and the JWT token to the TCP server, making HTTP updates seamlessly trigger TCP syncs to mobile or desktop clients.

---

## 11) Conclusion

The project is implemented in modular phases, starting from core HTTP/auth/database and extending to multi-protocol communication (TCP, UDP, WebSocket, gRPC).  
This structure supports both course demonstration requirements and further extension.


