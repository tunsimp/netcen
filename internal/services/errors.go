package services

import "errors"

var (
	ErrInvalidUserID              = errors.New("user_id is required")
	ErrInvalidMangaID             = errors.New("manga_id is required")
	ErrInvalidChapter             = errors.New("chapter must be greater than 0")
	ErrInvalidProgressStatus      = errors.New("status must be one of: reading, completed, plan_to_read")
	ErrInvalidNotificationMessage = errors.New("message is required")
	ErrMangaNotFound              = errors.New("manga not found")
)
