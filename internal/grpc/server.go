package grpcserver

import (
	"context"
	"database/sql"
	"errors"
	mangapb "project/internal/grpc/gen"
	"strings"
)

type MangaServiceServer struct {
	mangapb.UnimplementedMangaServiceServer
	db *sql.DB
}

func NewMangaServiceServer(db *sql.DB) *MangaServiceServer {
	return &MangaServiceServer{db: db}
}

func (s *MangaServiceServer) GetManga(ctx context.Context, req *mangapb.GetMangaRequest) (*mangapb.MangaResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return &mangapb.MangaResponse{}, nil
	}

	var mangaID string
	var title, author, genres, status, description sql.NullString
	var totalChapters sql.NullInt64
	err := s.db.QueryRow(
		`SELECT id, title, author, genres, status, total_chapters, description FROM manga WHERE id = ?`,
		id,
	).Scan(&mangaID, &title, &author, &genres, &status, &totalChapters, &description)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &mangapb.MangaResponse{}, nil
		}
		return nil, err
	}

	return &mangapb.MangaResponse{
		Id:            mangaID,
		Title:         title.String,
		Author:        author.String,
		Genres:        genres.String,
		Status:        status.String,
		TotalChapters: int32(totalChapters.Int64),
		Description:   description.String,
	}, nil
}

func (s *MangaServiceServer) SearchManga(ctx context.Context, req *mangapb.SearchRequest) (*mangapb.SearchResponse, error) {
	keyword := strings.TrimSpace(req.GetKeyword())
	status := strings.TrimSpace(req.GetStatus())
	genre := strings.TrimSpace(req.GetGenre())

	query := `SELECT id, title, author, genres, status, total_chapters, description FROM manga WHERE 1=1`
	args := make([]interface{}, 0)
	if keyword != "" {
		query += ` AND (title LIKE ? OR author LIKE ?)`
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if genre != "" {
		query += ` AND genres LIKE ?`
		args = append(args, "%"+genre+"%")
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*mangapb.MangaResponse, 0)
	for rows.Next() {
		var mangaID string
		var title, author, genres, status, description sql.NullString
		var totalChapters sql.NullInt64

		err = rows.Scan(&mangaID, &title, &author, &genres, &status, &totalChapters, &description)
		if err != nil {
			return nil, err
		}

		items = append(items, &mangapb.MangaResponse{
			Id:            mangaID,
			Title:         title.String,
			Author:        author.String,
			Genres:        genres.String,
			Status:        status.String,
			TotalChapters: int32(totalChapters.Int64),
			Description:   description.String,
		})
	}

	return &mangapb.SearchResponse{Items: items}, nil
}

func (s *MangaServiceServer) UpdateProgress(ctx context.Context, req *mangapb.ProgressRequest) (*mangapb.ProgressResponse, error) {
	userID := strings.TrimSpace(req.GetUserId())
	mangaID := strings.TrimSpace(req.GetMangaId())
	readingStatus := strings.TrimSpace(req.GetStatus())
	if userID == "" || mangaID == "" || readingStatus == "" || req.GetCurrentChapter() < 0 {
		return &mangapb.ProgressResponse{
			Success: false,
			Message: "invalid progress request",
		}, nil
	}

	_, err := s.db.Exec(
		`INSERT INTO user_progress(user_id, manga_id, current_chapter, status, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id, manga_id) DO UPDATE SET
		 current_chapter = excluded.current_chapter,
		 status = excluded.status,
		 updated_at = CURRENT_TIMESTAMP`,
		userID, mangaID, req.GetCurrentChapter(), readingStatus,
	)
	if err != nil {
		return nil, err
	}

	return &mangapb.ProgressResponse{
		Success: true,
		Message: "progress updated",
	}, nil
}
