package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mangahub/internal/repository"
)

type MangaHandler struct {
	manga *repository.MangaRepository
}

func NewMangaHandler(manga *repository.MangaRepository) *MangaHandler {
	return &MangaHandler{manga: manga}
}

func (h *MangaHandler) List(c *gin.Context) {
	mangaList, err := h.manga.List(repository.MangaFilters{
		Query:  c.Query("q"),
		Genre:  c.Query("genre"),
		Status: c.Query("status"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load manga"})
		return
	}

	c.JSON(http.StatusOK, mangaList)
}

func (h *MangaHandler) Get(c *gin.Context) {
	manga, err := h.manga.FindByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load manga"})
		return
	}
	if manga == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}

	c.JSON(http.StatusOK, manga)
}
