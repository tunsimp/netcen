package tcp

type ProgressUpdate struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}
