# Manga Module Explanation

## 1. Mục tiêu của module manga

Module `internal/manga` phục vụ phần **Week 3: Data Collection & Integration**.

Nhiệm vụ chính:

- Lưu dữ liệu manga thủ công từ file JSON.
- Validate dữ liệu manga trước khi lưu.
- Cung cấp API để list, search, get detail và add manga.
- Tích hợp MangaDex API để import manga đơn giản từ nguồn bên ngoài.

Các file chính:

```text
internal/manga/
├── types.go
├── store.go
├── http.go
└── mangadex.go
```

Data được lưu ở:

```text
data/manga_seed.json
```

## 2. `types.go`

File:

```text
internal/manga/types.go
```

File này định nghĩa data model chính của manga:

```go
type Manga struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	TotalChapters int      `json:"total_chapters"`
	Description   string   `json:"description"`
	Source        string   `json:"source"`
}
```

Ý nghĩa các field:

| Field | Ý nghĩa |
|---|---|
| `ID` | Mã định danh manga, dùng cho endpoint `/manga/{id}` |
| `Title` | Tên manga |
| `Author` | Tác giả |
| `Genres` | Danh sách thể loại |
| `Status` | Trạng thái như `ongoing`, `completed`, `hiatus` |
| `TotalChapters` | Tổng số chapter hiện biết |
| `Description` | Mô tả ngắn |
| `Source` | Nguồn dữ liệu, ví dụ `manual` hoặc `mangadex` |

File này chỉ chứa struct, không xử lý logic. Các file khác dùng `Manga` để đọc, lưu, validate và trả response API.

## 3. `store.go`

File:

```text
internal/manga/store.go
```

File này là lớp lưu trữ dữ liệu manga bằng JSON file.

Struct chính:

```go
type Store struct {
	filePath string
	mangas   []Manga
	mu       sync.Mutex
}
```

Ý nghĩa:

- `filePath`: đường dẫn tới file `data/manga_seed.json`.
- `mangas`: danh sách manga được load vào memory.
- `mu`: mutex để tránh nhiều request cùng sửa dữ liệu một lúc.

### `NewStore`

```go
func NewStore(filePath string) (*Store, error)
```

Tạo store mới và gọi `Load()` để đọc dữ liệu từ JSON file.

Trong `cmd/api-server/main.go`, store được tạo như sau:

```go
mangaStore, err := manga.NewStore("data/manga_seed.json")
```

### `Load`

```go
func (s *Store) Load() error
```

Đọc file JSON bằng `os.ReadFile`, sau đó parse JSON thành `[]Manga` bằng:

```go
json.Unmarshal(data, &s.mangas)
```

### `Save`

```go
func (s *Store) Save() error
```

Chuyển danh sách manga trong memory thành JSON đẹp bằng:

```go
json.MarshalIndent(s.mangas, "", "  ")
```

Sau đó ghi lại vào file bằng `os.WriteFile`.

### `List`

```go
func (s *Store) List(query SearchQuery) []Manga
```

Trả về danh sách manga, có hỗ trợ filter:

- `title`
- `author`
- `status`
- `genre`

Ví dụ:

```text
GET /manga?title=one
GET /manga?status=completed
GET /manga?genre=Action
```

### `GetByID`

```go
func (s *Store) GetByID(id string) (Manga, bool)
```

Tìm manga theo `id`.

Nếu có:

```go
return manga, true
```

Nếu không có:

```go
return Manga{}, false
```

### `Add`

```go
func (s *Store) Add(manga Manga) error
```

Thêm manga mới vào store.

Các bước:

1. Trim dữ liệu input.
2. Nếu `Source` rỗng thì set mặc định là `manual`.
3. Validate dữ liệu bằng `validateManga`.
4. Kiểm tra trùng `ID`.
5. Append vào slice.
6. Gọi `Save()` để ghi lại vào JSON file.

### Validation

Function:

```go
func validateManga(manga Manga) error
```

Kiểm tra:

- `id` không rỗng.
- `title` không rỗng.
- `total_chapters >= 0`.
- `status` nếu có thì phải thuộc một trong:

```text
ongoing
completed
hiatus
cancelled
unknown
```

## 4. `http.go`

File:

```text
internal/manga/http.go
```

File này định nghĩa các HTTP endpoint cho manga.

Route được register bằng:

```go
func RegisterHTTPRoutes(mux *http.ServeMux, store *Store)
```

Trong `cmd/api-server/main.go`, code gọi:

```go
manga.RegisterHTTPRoutes(mux, mangaStore)
```

Các endpoint chính:

| Method | Endpoint | Ý nghĩa |
|---|---|---|
| `GET` | `/manga` | List/search manga |
| `GET` | `/manga/{id}` | Lấy chi tiết manga |
| `POST` | `/manga` | Thêm manga thủ công |
| `POST` | `/manga/import/mangadex?title=...` | Import manga từ MangaDex |

### `GET /manga`

Handler:

```go
listMangaHandler
```

Đọc query parameters:

```go
title
author
status
genre
```

Sau đó gọi:

```go
store.List(query)
```

Response mẫu:

```json
{
  "count": 24,
  "data": [
    {
      "id": "one-piece",
      "title": "One Piece",
      "author": "Eiichiro Oda"
    }
  ]
}
```

### `GET /manga/{id}`

Handler:

```go
mangaDetailHandler
```

Ví dụ:

```text
GET /manga/one-piece
```

Handler lấy `id` bằng:

```go
strings.TrimPrefix(r.URL.Path, "/manga/")
```

Sau đó gọi:

```go
store.GetByID(id)
```

Nếu không tìm thấy thì trả:

```json
{
  "error": "manga not found"
}
```

### `POST /manga`

Handler:

```go
createMangaHandler
```

Nhận JSON body, decode thành `Manga`, rồi gọi:

```go
store.Add(manga)
```

Nếu trùng ID thì trả `409 Conflict`.

### `POST /manga/import/mangadex`

Handler:

```go
mangaDexImportHandler
```

Ví dụ:

```text
POST /manga/import/mangadex?title=chainsaw%20man
```

Handler gọi:

```go
FetchFromMangaDex(title)
```

Sau đó lưu manga import vào JSON store bằng:

```go
store.Add(manga)
```

## 5. `mangadex.go`

File:

```text
internal/manga/mangadex.go
```

File này xử lý tích hợp MangaDex API.

Function chính:

```go
func FetchFromMangaDex(title string) (Manga, error)
```

Luồng xử lý:

1. Trim title.
2. Tạo URL:

```go
https://api.mangadex.org/manga?limit=1&title=...
```

3. Gửi HTTP GET bằng:

```go
http.Get(apiURL)
```

4. Decode response JSON vào `MangaDexResponse`.
5. Lấy kết quả đầu tiên.
6. Convert dữ liệu MangaDex thành struct `Manga`.
7. Set `Source` thành `"mangadex"`.

### `MangaDexResponse`

Struct này chỉ chứa các field cần thiết từ MangaDex response:

```go
type MangaDexResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title       map[string]string `json:"title"`
			Description map[string]string `json:"description"`
			Status      string            `json:"status"`
			Tags        []struct {
				Attributes struct {
					Name map[string]string `json:"name"`
				} `json:"attributes"`
			} `json:"tags"`
		} `json:"attributes"`
	} `json:"data"`
}
```

MangaDex trả title/description theo nhiều ngôn ngữ, nên code dùng:

```go
pickLocalized
```

để ưu tiên tiếng Anh (`en`), nếu không có thì lấy giá trị đầu tiên không rỗng.

### `normalizeStatus`

Function:

```go
func normalizeStatus(status string) string
```

Chuẩn hóa status từ MangaDex về các giá trị hợp lệ trong hệ thống:

```text
ongoing
completed
hiatus
cancelled
unknown
```

Nếu MangaDex trả status lạ, hệ thống dùng `"unknown"`.

## 6. Data file `manga_seed.json`

File:

```text
data/manga_seed.json
```

File này hiện chứa 24 manga series:

- 23 manga nhập thủ công.
- 1 manga import từ MangaDex.

Mục tiêu của file này là đáp ứng yêu cầu:

```text
Manual manga data entry (20-30 series)
```

Khi API server start, `Store` load file này vào memory.

Khi gọi `POST /manga` hoặc `POST /manga/import/mangadex`, store sẽ ghi dữ liệu mới ngược lại vào file.

## 7. Cách test nhanh

Chạy API server:

```powershell
go run .\cmd\api-server
```

List manga:

```powershell
Invoke-RestMethod -Method GET -Uri "http://localhost:8080/manga"
```

Search theo title:

```powershell
Invoke-RestMethod -Method GET -Uri "http://localhost:8080/manga?title=one"
```

Get detail:

```powershell
Invoke-RestMethod -Method GET -Uri "http://localhost:8080/manga/one-piece"
```

Import từ MangaDex:

```powershell
Invoke-RestMethod `
  -Method POST `
  -Uri "http://localhost:8080/manga/import/mangadex?title=chainsaw%20man"
```

Xem JSON đẹp:

```powershell
$response = Invoke-RestMethod -Method GET -Uri "http://localhost:8080/manga"
$response | ConvertTo-Json -Depth 10
```

## 8. Kết luận

Module manga hiện tại hoàn thành phần cơ bản của Week 3:

- Có data model rõ ràng.
- Có JSON storage.
- Có validation trước khi lưu.
- Có 20-30 manga series thủ công.
- Có API endpoint để list/search/detail/create.
- Có tích hợp MangaDex API đơn giản.

Đây là foundation để các phần khác như user library, progress tracking, TCP sync hoặc UDP notification có dữ liệu manga thật để sử dụng.
