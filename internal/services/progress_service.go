package services

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"mangahub/internal/models"
	"mangahub/internal/repository"
)

type ProgressService struct {
	mangaRepo    *repository.MangaRepository
	progressRepo *repository.ProgressRepository

	mu          sync.RWMutex
	nextSubID   int
	subscribers map[int]chan models.ProgressUpdate
}

func NewProgressService(mangaRepo *repository.MangaRepository, progressRepo *repository.ProgressRepository) *ProgressService {
	return &ProgressService{
		mangaRepo:    mangaRepo,
		progressRepo: progressRepo,
		subscribers:  make(map[int]chan models.ProgressUpdate),
	}
}

func (s *ProgressService) Upsert(userID, mangaID string, chapter int, status string, timestamp int64) (*models.UserProgress, error) {
	userID = strings.TrimSpace(userID)
	mangaID = strings.TrimSpace(mangaID)
	status = strings.TrimSpace(strings.ToLower(status))

	if userID == "" {
		return nil, ErrInvalidUserID
	}
	if mangaID == "" {
		return nil, ErrInvalidMangaID
	}
	if chapter < 1 {
		return nil, ErrInvalidChapter
	}
	if !isValidProgressStatus(status) {
		return nil, ErrInvalidProgressStatus
	}

	manga, err := s.mangaRepo.FindByID(mangaID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manga: %w", err)
	}
	if manga == nil {
		return nil, ErrMangaNotFound
	}

	progress, err := s.progressRepo.Upsert(models.UserProgress{
		UserID:         userID,
		MangaID:        mangaID,
		CurrentChapter: chapter,
		Status:         status,
	})
	if err != nil {
		return nil, err
	}

	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}

	s.publish(models.ProgressUpdate{
		Type:      "progress_broadcast",
		UserID:    progress.UserID,
		MangaID:   progress.MangaID,
		Chapter:   progress.CurrentChapter,
		Status:    progress.Status,
		Timestamp: timestamp,
	})

	return progress, nil
}

func (s *ProgressService) Subscribe(buffer int) (<-chan models.ProgressUpdate, func()) {
	ch := make(chan models.ProgressUpdate, buffer)

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

func (s *ProgressService) publish(update models.ProgressUpdate) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- update:
		default:
		}
	}
}

func isValidProgressStatus(status string) bool {
	switch status {
	case models.ProgressStatusReading, models.ProgressStatusCompleted, models.ProgressStatusPlanToRead:
		return true
	default:
		return false
	}
}
