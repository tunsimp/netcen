# Backend Work Split

## Feature Map
Backend MangaHub duoc chia theo feature nghiep vu, sau do moi map sang protocol va module ky thuat.

| Feature | Owner | Depends on | Notes |
| --- | --- | --- | --- |
| Auth | Nguoi B | HTTP, JWT, SQLite `users` | `register`, `login`, `me` |
| Manga/Catalog API | Nguoi B | HTTP, SQLite `manga` | list, detail, search/filter |
| Reading Progress | Shared: A propose data, B expose API | SQLite `user_progress`, HTTP | data va API phai cung contract |
| Realtime Progress Sync | Nguoi A | TCP, progress contract | sync/broadcast progress giua client |
| Notifications | Nguoi A | UDP, manga/progress events | chapter release hoac event broadcast |
| WebSocket Chat | Nguoi B | WebSocket, auth/user identity | chat/discussion realtime |
| gRPC Internal Service | Nguoi B | protobuf, shared models | internal communication theo rubric |
| SQLite Persistence | Nguoi A | schema + repository/service | schema mo rong ngoai `users` |

Nguyen tac chung:
- Chia theo ownership, khong chia trung cung mot flow nghiep vu.
- Moi flow chi co mot nguoi chiu trach nhiem chinh; phan con lai chi consume contract da chot.
- Neu mot feature cham ca HTTP va data, phai chot contract truoc khi hai nguoi code song song.

## Ownership
### Nguoi A
- `TCP Progress Sync`
  Deliverable: TCP protocol, connection handling, progress broadcast behavior, socket-side tests.
- `UDP Notification`
  Deliverable: UDP message format, client registration mechanism, notification broadcast behavior.
- `SQLite integration`
  Deliverable: schema `manga`, `user_progress`, repository/service cho progress + notification data neu can.
- `Error handling/logging pattern`
  Deliverable: quy uoc wrap loi, log loi, phan loai loi cho socket + DB layer.

### Nguoi B
- `HTTP REST API (Gin + JWT)`
  Deliverable: endpoint contract, auth middleware, request/response schema, status code behavior.
- `WebSocket Chat`
  Deliverable: chat message flow, connection lifecycle, auth/user identity mapping.
- `gRPC Service`
  Deliverable: protobuf definition, service contract, client-server communication.
- `Code structure / project setup`
  Deliverable: folder convention, app bootstrap, server wiring tong the, shared package organization.

## Ownership Boundaries
### Module ownership
- Nguoi A so huu:
  - `internal/tcp`
  - `internal/udp`
  - repository/service lien quan progress + notification
  - phan mo rong schema trong `internal/database`
- Nguoi B so huu:
  - `internal/http`
  - `internal/auth`
  - `internal/ws`
  - `internal/grpc`
  - wiring tong the trong `cmd/api-server` neu can

### Shared files can trao doi truoc khi sua
- `internal/config`
- `internal/models`
- `internal/database`
- `cmd/api-server/main.go`
- `go.mod`

### Quy tac tranh dung nhau
- Khong sua file nguoi kia dang own neu chua bao truoc.
- Shared contract doi thi phai chot truoc trong nhom.
- Ai tao interface/struct dung chung thi nguoi kia chi consume, khong tu y doi shape.
- Khi sua shared file, phai ghi ro muc tieu sua va anh huong den phan nguoi kia.

## Shared Contracts
### Week 4 update
- Nguoi A da propose va implement:
  - bang `manga`
  - bang `user_progress`
  - struct `Manga`
  - struct `UserProgress`
  - struct `ProgressUpdate`
- semantics cua progress:
  - `user_progress.updated_at` la timestamp lan cap nhat gan nhat trong DB
  - `ProgressUpdate.timestamp` la event timestamp dung de broadcast qua TCP
  - status hop le cho Week 4: `reading`, `completed`, `plan_to_read`
- Nguoi B chua can sua HTTP route moi o week nay; chi can consume contract nay tu Week 5 tro di.
- Neu can them HTTP progress route o buoc sau, phai dung lai dung:
  - bang `manga`
  - bang `user_progress`
  - struct `ProgressUpdate`
  - naming status enum o tren

### Contract can giu on dinh
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
- Payload contract giua HTTP va TCP/UDP
- Port/config names cho HTTP, TCP, UDP, WebSocket, gRPC

Quy tac propose/review:
- Nguoi A propose phan progress/notification/data contract.
- Nguoi B propose phan HTTP/auth/chat/gRPC contract.
- Moi thay doi contract phai cap nhat lai file nay truoc hoac cung luc voi code thay doi.

## Integration Order
### Moc tich hop
- Week 3:
  - TCP basic
- Week 4:
  - noi TCP voi SQLite progress data
  - chua them HTTP progress endpoint
- Week 5:
  - nguoi B consume contract `manga` + `user_progress` de noi HTTP
  - nguoi A chot UDP register/broadcast contract va `NotificationService`

### Dependency can giu
- HTTP auth co the lam song song TCP basic.
- Schema `manga` + `user_progress` phai giu on dinh truoc khi tich hop HTTP progress.
- WebSocket/gRPC khong duoc tu tao model progress khac voi contract DB/HTTP da chot.
- Notification payload phai bam cung naming convention voi progress/manga data.
- Neu can trigger notification tu HTTP o buoc sau, handler phai goi `NotificationService` thay vi tu broadcast truc tiep tu route.

### Handoff checklist giua 2 nguoi
- endpoint list
- payload examples
- schema migration notes
- integration test ownership
- shared files can bao truoc khi sua:
  - `internal/config`
  - `internal/models`
  - `internal/database`
  - `cmd/api-server/main.go`
  - `go.mod`
