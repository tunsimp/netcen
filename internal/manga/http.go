package manga

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterHTTPRoutes(router *gin.Engine, db *sql.DB) {
	router.GET("/manga", searchMangaHandler(db))
	router.GET("/manga/:id", getMangaByIDHandler(db))
}

func searchMangaHandler(db *sql.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		title := strings.TrimSpace(ctx.Query("title"))
		author := strings.TrimSpace(ctx.Query("author"))
		status := strings.TrimSpace(ctx.Query("status"))
		genre := strings.TrimSpace(ctx.Query("genre"))

		query := `SELECT id, title, author, genres, status, total_chapters, description FROM manga WHERE 1=1`
		args := make([]interface{}, 0)

		if title != "" {
			query += ` AND title LIKE ?`
			args = append(args, "%"+title+"%")
		}
		if author != "" {
			query += ` AND author LIKE ?`
			args = append(args, "%"+author+"%")
		}
		if status != "" {
			query += ` AND status = ?`
			args = append(args, status)
		}
		if genre != "" {
			query += ` AND genres LIKE ?`
			args = append(args, "%"+genre+"%")
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch manga"})
			return
		}
		defer rows.Close()

		mangaList := make([]gin.H, 0)
		for rows.Next() {
			var id string
			var mangaTitle, mangaAuthor, genres, mangaStatus, description sql.NullString
			var totalChapters sql.NullInt64

			err = rows.Scan(&id, &mangaTitle, &mangaAuthor, &genres, &mangaStatus, &totalChapters, &description)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse manga row"})
				return
			}

			mangaList = append(mangaList, gin.H{
				"id":             id,
				"title":          mangaTitle.String,
				"author":         mangaAuthor.String,
				"genres":         genres.String,
				"status":         mangaStatus.String,
				"total_chapters": totalChapters.Int64,
				"description":    description.String,
			})
		}

		ctx.JSON(http.StatusOK, gin.H{"data": mangaList})
	}
}

func getMangaByIDHandler(db *sql.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.Param("id")
		if id == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "manga id is required"})
			return
		}

		var mangaID string
		var title, author, genres, status, description sql.NullString
		var totalChapters sql.NullInt64

		err := db.QueryRow(
			`SELECT id, title, author, genres, status, total_chapters, description FROM manga WHERE id = ?`,
			id,
		).Scan(&mangaID, &title, &author, &genres, &status, &totalChapters, &description)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch manga"})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"id":             mangaID,
			"title":          title.String,
			"author":         author.String,
			"genres":         genres.String,
			"status":         status.String,
			"total_chapters": totalChapters.Int64,
			"description":    description.String,
		})
	}
}
