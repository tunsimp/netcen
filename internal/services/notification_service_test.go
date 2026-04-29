package services_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mangahub/internal/database"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

func TestNotificationServicePublish(t *testing.T) {
	t.Parallel()

	service := newTestNotificationService(t)
	ch, unsubscribe := service.Subscribe(1)
	defer unsubscribe()

	before := time.Now().Unix()
	if err := service.Publish("one-piece", "One Piece chapter 1096 released", 0); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case notification := <-ch:
		if notification.Type != "chapter_release" {
			t.Fatalf("Type = %s, want chapter_release", notification.Type)
		}
		if notification.MangaID != "one-piece" {
			t.Fatalf("MangaID = %s, want one-piece", notification.MangaID)
		}
		if notification.Message != "One Piece chapter 1096 released" {
			t.Fatalf("Message = %s", notification.Message)
		}
		if notification.Timestamp < before {
			t.Fatalf("Timestamp = %d, want >= %d", notification.Timestamp, before)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive notification")
	}
}

func TestNotificationServiceRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	service := newTestNotificationService(t)

	testCases := []struct {
		name    string
		mangaID string
		message string
		want    error
	}{
		{name: "missing manga id", mangaID: " ", message: "release", want: services.ErrInvalidMangaID},
		{name: "missing message", mangaID: "one-piece", message: " ", want: services.ErrInvalidNotificationMessage},
		{name: "unknown manga", mangaID: "missing", message: "release", want: services.ErrMangaNotFound},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := service.Publish(tc.mangaID, tc.message, 0)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func newTestNotificationService(t *testing.T) *services.NotificationService {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return services.NewNotificationService(repository.NewMangaRepository(db))
}
