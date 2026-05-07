package user

import (
	"database/sql"
	"net/http"
	"project/internal/tcp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type addLibraryRequest struct {
	MangaID string `json:"manga_id"`
}

type updateProgressRequest struct {
	MangaID        string `json:"manga_id"`
	CurrentChapter int    `json:"current_chapter"`
	ReadingStatus  string `json:"status"`
}

func RegisterHTTPRoutes(group *gin.RouterGroup, db *sql.DB, tcpAddress string) {
	group.POST("/library", addToLibraryHandler(db))
	group.GET("/library", getLibraryHandler(db))
	group.PUT("/progress", updateProgressHandler(db, tcpAddress))
}

func addToLibraryHandler(db *sql.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID, exists := ctx.Get("userID")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req addLibraryRequest
		err := ctx.ShouldBindJSON(&req)
		if err != nil || strings.TrimSpace(req.MangaID) == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "manga_id is required"})
			return
		}

		_, err = db.Exec(
			`INSERT INTO user_progress(user_id, manga_id, current_chapter, status) VALUES (?, ?, 0, 'plan_to_read')
			 ON CONFLICT(user_id, manga_id) DO NOTHING`,
			userID.(string), req.MangaID,
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add manga to library"})
			return
		}

		ctx.JSON(http.StatusCreated, gin.H{"message": "manga added to library"})
	}
}

func getLibraryHandler(db *sql.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID, exists := ctx.Get("userID")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		rows, err := db.Query(
			`SELECT up.manga_id, m.title, m.author, up.current_chapter, up.status, up.updated_at
			 FROM user_progress up
			 LEFT JOIN manga m ON m.id = up.manga_id
			 WHERE up.user_id = ?
			 ORDER BY up.updated_at DESC`,
			userID.(string),
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch library"})
			return
		}
		defer rows.Close()

		library := make([]gin.H, 0)
		for rows.Next() {
			var mangaID string
			var title, author, status, updatedAt sql.NullString
			var currentChapter sql.NullInt64

			err = rows.Scan(&mangaID, &title, &author, &currentChapter, &status, &updatedAt)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse library row"})
				return
			}

			library = append(library, gin.H{
				"manga_id":        mangaID,
				"title":           title.String,
				"author":          author.String,
				"current_chapter": currentChapter.Int64,
				"status":          status.String,
				"updated_at":      updatedAt.String,
			})
		}

		ctx.JSON(http.StatusOK, gin.H{"data": library})
	}
}

func updateProgressHandler(db *sql.DB, tcpAddress string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID, exists := ctx.Get("userID")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req updateProgressRequest
		err := ctx.ShouldBindJSON(&req)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(req.MangaID) == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "manga_id is required"})
			return
		}
		if req.CurrentChapter < 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "current_chapter must be >= 0"})
			return
		}
		if strings.TrimSpace(req.ReadingStatus) == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
			return
		}

		_, err = db.Exec(
			`INSERT INTO user_progress(user_id, manga_id, current_chapter, status, updated_at)
			 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(user_id, manga_id) DO UPDATE SET
			 current_chapter = excluded.current_chapter,
			 status = excluded.status,
			 updated_at = CURRENT_TIMESTAMP`,
			userID.(string), req.MangaID, req.CurrentChapter, req.ReadingStatus,
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update progress"})
			return
		}

		update := tcp.ProgressUpdate{
			UserID:    userID.(string),
			MangaID:   req.MangaID,
			Chapter:   req.CurrentChapter,
			Status:    req.ReadingStatus,
			Timestamp: time.Now().Unix(),
		}
		if err := tcp.SendProgressUpdate(tcpAddress, update); err != nil {
			ctx.JSON(http.StatusBadGateway, gin.H{"error": "failed to forward progress update to TCP server"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message":         "progress updated",
			"manga_id":        req.MangaID,
			"current_chapter": req.CurrentChapter,
			"status":          req.ReadingStatus,
			"user_id":         userID.(string),
		})
	}
}
