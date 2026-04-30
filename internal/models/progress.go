package models

import "time"

const (
	ProgressStatusReading    = "reading"
	ProgressStatusCompleted  = "completed"
	ProgressStatusPlanToRead = "plan_to_read"
)

type UserProgress struct {
	UserID         string    `json:"user_id"`
	MangaID        string    `json:"manga_id"`
	CurrentChapter int       `json:"current_chapter"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProgressUpdate struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}
