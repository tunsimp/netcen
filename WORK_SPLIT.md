# Backend Work Split

## Feature Map
Backend MangaHub được chia theo feature nghiệp vụ, sau đó mới map sang protocol và module kỹ thuật.

| Feature | Owner | Depends on | Notes |
| --- | --- | --- | --- |
| Auth | Người B | HTTP, JWT, SQLite `users` | `register`, `login`, `me` |
| Manga/Catalog API | Người B | HTTP, SQLite `manga` | list, detail, search/filter |
| Reading Progress | Shared: A propose data, B expose API | SQLite `user_progress`, HTTP | logic dữ liệu và API phải cùng contract |
| Realtime Progress Sync | Người A | TCP, progress contract | sync/broadcast progress giữa client |
| Notifications | Người A | UDP, manga/progress events | chapter release hoặc event broadcast |
| WebSocket Chat | Người B | WebSocket, auth/user identity | chat/discussion realtime |
| gRPC Internal Service | Người B | protobuf, shared models | internal communication theo rubric |
| SQLite Persistence | Người A | schema + repository/service | schema mở rộng ngoài `users` |

Nguyên tắc chung:
- Chia theo ownership, không chia trùng cùng một flow nghiệp vụ.
- Mỗi flow chỉ có một người chịu trách nhiệm chính; phần còn lại chỉ consume contract đã chốt.
- Nếu một feature chạm cả HTTP và data, phải chốt contract trước khi hai người code song song.

## Ownership
### Người A
- `TCP Progress Sync`
  Deliverable: TCP protocol, connection handling, progress broadcast behavior, socket-side tests.
- `UDP Notification`
  Deliverable: UDP message format, client registration mechanism, notification broadcast behavior.
- `SQLite integration`
  Deliverable: schema `manga`, `user_progress`, repository/service cho progress + notification data nếu cần.
- `Error handling/logging pattern`
  Deliverable: quy ước wrap lỗi, log lỗi, phân loại lỗi cho socket + DB layer.

### Người B
- `HTTP REST API (Gin + JWT)`
  Deliverable: endpoint contract, auth middleware, request/response schema, status code behavior.
- `WebSocket Chat`
  Deliverable: chat message flow, connection lifecycle, auth/user identity mapping.
- `gRPC Service`
  Deliverable: protobuf definition, service contract, client-server communication.
- `Code structure / project setup`
  Deliverable: folder convention, app bootstrap, server wiring tổng thể, shared package organization.

## Ownership Boundaries
### Module ownership
- Người A sở hữu:
  - `internal/tcp`
  - `internal/udp`
  - repository/service liên quan progress + notification
  - phần mở rộng schema trong `internal/database`
- Người B sở hữu:
  - `internal/http`
  - `internal/auth`
  - `internal/ws`
  - `internal/grpc`
  - wiring tổng thể trong `cmd/api-server` nếu cần

### Shared files cần trao đổi trước khi sửa
- `internal/config`
- `internal/models`
- `internal/database`
- `cmd/api-server/main.go`
- `go.mod`

### Quy tắc tránh đụng nhau
- Không sửa file người kia đang own nếu chưa báo trước.
- Shared contract đổi thì phải chốt trước trong nhóm.
- Ai tạo interface/struct dùng chung thì người kia chỉ consume, không tự ý đổi shape.
- Khi sửa shared file, phải ghi rõ mục tiêu sửa và ảnh hưởng đến phần người kia.

## Shared Contracts
Các contract phải thống nhất sớm trước khi code sâu:
- SQLite schema:
  - `users`
  - `manga`
  - `user_progress`
- Shared models:
  - `User`
  - `Manga`
  - `ProgressUpdate`
  - `Notification`
- Naming/status enum:
  - `reading`
  - `completed`
  - `plan_to_read`
- Payload contract giữa HTTP và TCP/UDP
- Port/config names cho HTTP, TCP, UDP, WebSocket, gRPC

Quy tắc propose/review:
- Người A propose phần progress/notification/data contract.
- Người B propose phần HTTP/auth/chat/gRPC contract.
- Mọi thay đổi contract phải cập nhật lại file này trước hoặc cùng lúc với code thay đổi.

## Integration Order
### Mốc tích hợp
- Week 3:
  - TCP basic
- Week 4:
  - nối TCP với HTTP progress + DB
- Week 5:
  - UDP notifications

### Dependency cần giữ
- HTTP auth có thể làm song song TCP basic.
- Schema `manga` + `user_progress` phải chốt trước khi tích hợp Week 4.
- WebSocket/gRPC không được tự tạo model progress khác với contract DB/HTTP đã chốt.
- Notification payload phải bám cùng naming convention với progress/manga data.

### Handoff checklist giữa 2 người
- Endpoint list
- Payload examples
- Schema migration notes
- Integration test ownership

### Acceptance mục tiêu
- Mỗi feature backend có đúng một owner chính.
- Không có flow nào bị 2 người cùng sửa cùng lúc mà không có boundary.
- Người mới đọc file biết muốn sửa phần nào thì cần hỏi ai trước.
