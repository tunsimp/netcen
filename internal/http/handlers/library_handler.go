package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"mangahub/internal/http/middleware"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

type LibraryHandler struct {
	progressRepo    *repository.ProgressRepository
	progressService *services.ProgressService
}

type progressRequest struct {
	MangaID        string `json:"manga_id" binding:"required"`
	CurrentChapter int    `json:"current_chapter" binding:"required"`
	Status         string `json:"status" binding:"required"`
}

func NewLibraryHandler(progressRepo *repository.ProgressRepository, progressService *services.ProgressService) *LibraryHandler {
	return &LibraryHandler{
		progressRepo:    progressRepo,
		progressService: progressService,
	}
}

func (h *LibraryHandler) Add(c *gin.Context) {
	h.upsertProgress(c)
}

func (h *LibraryHandler) UpdateProgress(c *gin.Context) {
	h.upsertProgress(c)
}

func (h *LibraryHandler) List(c *gin.Context) {
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user identity"})
		return
	}

	progressList, err := h.progressRepo.ListByUser(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load library"})
		return
	}

	c.JSON(http.StatusOK, progressList)
}

func (h *LibraryHandler) upsertProgress(c *gin.Context) {
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user identity"})
		return
	}

	var req progressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	progress, err := h.progressService.Upsert(userID.(string), req.MangaID, req.CurrentChapter, req.Status, 0)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMangaNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrInvalidUserID),
			errors.Is(err, services.ErrInvalidMangaID),
			errors.Is(err, services.ErrInvalidChapter),
			errors.Is(err, services.ErrInvalidProgressStatus):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save progress"})
		}
		return
	}

	c.JSON(http.StatusOK, progress)
}
