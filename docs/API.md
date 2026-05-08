# MangaHub API Documentation

## Base Endpoints

- HTTP: `http://localhost:8080`
- WebSocket: `ws://localhost:8080/ws/chat`
- gRPC: `localhost:9090`

## Authentication

JWT is required for protected HTTP and WebSocket endpoints.

Header format:

```text
Authorization: Bearer <token>
```

---

## HTTP API

## `POST /auth/register`

Register a new user.

Request body:

```json
{
  "username": "alice",
  "password": "123456"
}
```

Success:
- `201 Created`

---

## `POST /auth/login`

Authenticate user and get JWT.

Request body:

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

---

## `GET /manga`

Search manga with optional filters.

Query params:
- `title`
- `author`
- `status`
- `genre`

Example:

```text
GET /manga?title=piece&status=ongoing
```

---

## `GET /manga/:id`

Get manga detail by id.

Example:

```text
GET /manga/one-piece
```

---

## `POST /users/library` (Protected)

Add manga to logged-in user's library.

Request body:

```json
{
  "manga_id": "one-piece"
}
```

Success:
- `201 Created`

---

## `GET /users/library` (Protected)

Get logged-in user's library entries.

Success:
- `200 OK`

---

## `PUT /users/progress` (Protected)

Update reading progress for a manga.

Request body:

```json
{
  "manga_id": "one-piece",
  "current_chapter": 10,
  "status": "reading"
}
```

Success:
- `200 OK`

Note:
- This endpoint also forwards progress updates to the TCP sync server.

---

## WebSocket API

## `GET /ws/chat` (Protected)

Upgrade to WebSocket chat connection.

Client send:

```json
{
  "message": "Hello everyone"
}
```

Server broadcast format:

```json
{
  "user_id": "user-id",
  "username": "alice",
  "message": "Hello everyone",
  "timestamp": 1715140000
}
```

---

## gRPC API

Service:
- `mangahub.v1.MangaService`

Unary RPC methods:
- `GetManga(GetMangaRequest) returns (MangaResponse)`
- `SearchManga(SearchRequest) returns (SearchResponse)`
- `UpdateProgress(ProgressRequest) returns (ProgressResponse)`

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
