package services

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"mangahub/internal/models"
	"mangahub/internal/repository"
)

type NotificationService struct {
	mangaRepo *repository.MangaRepository

	mu          sync.RWMutex
	nextSubID   int
	subscribers map[int]chan models.Notification
}

func NewNotificationService(mangaRepo *repository.MangaRepository) *NotificationService {
	return &NotificationService{
		mangaRepo:   mangaRepo,
		subscribers: make(map[int]chan models.Notification),
	}
}

func (s *NotificationService) Publish(mangaID, message string, timestamp int64) error {
	mangaID = strings.TrimSpace(mangaID)
	message = strings.TrimSpace(message)

	if mangaID == "" {
		return ErrInvalidMangaID
	}
	if message == "" {
		return ErrInvalidNotificationMessage
	}

	manga, err := s.mangaRepo.FindByID(mangaID)
	if err != nil {
		return fmt.Errorf("failed to load manga: %w", err)
	}
	if manga == nil {
		return ErrMangaNotFound
	}

	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}

	s.publish(models.Notification{
		Type:      "chapter_release",
		MangaID:   mangaID,
		Message:   message,
		Timestamp: timestamp,
	})

	return nil
}

func (s *NotificationService) Subscribe(buffer int) (<-chan models.Notification, func()) {
	ch := make(chan models.Notification, buffer)

	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		if existing, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(existing)
		}
		s.mu.Unlock()
	}
}

func (s *NotificationService) publish(notification models.Notification) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- notification:
		default:
		}
	}
}
