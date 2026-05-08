package grpcserver

import (
	"context"
	"database/sql"
	"testing"

	"project/internal/auth"
	mangapb "project/internal/grpc/gen"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupGRPCTestServer(t *testing.T) (*MangaServiceServer, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)

	authService := auth.NewService(db, []byte("test-secret"))
	require.NoError(t, authService.EnsureSchema())

	_, err = db.Exec(`INSERT INTO manga(id, title, author, genres, status, total_chapters, description)
	VALUES ('one-piece', 'One Piece', 'Eiichiro Oda', 'action, adventure', 'ongoing', 1100, 'Pirate journey')`)
	require.NoError(t, err)

	return NewMangaServiceServer(db), db
}

func TestGetManga(t *testing.T) {
	server, db := setupGRPCTestServer(t)
	defer db.Close()

	resp, err := server.GetManga(context.Background(), &mangapb.GetMangaRequest{Id: "one-piece"})
	require.NoError(t, err)
	require.Equal(t, "one-piece", resp.GetId())
	require.Equal(t, "One Piece", resp.GetTitle())
}

func TestSearchManga(t *testing.T) {
	server, db := setupGRPCTestServer(t)
	defer db.Close()

	resp, err := server.SearchManga(context.Background(), &mangapb.SearchRequest{Keyword: "piece"})
	require.NoError(t, err)
	require.Len(t, resp.GetItems(), 1)
	require.Equal(t, "one-piece", resp.GetItems()[0].GetId())
}

func TestUpdateProgress(t *testing.T) {
	server, db := setupGRPCTestServer(t)
	defer db.Close()

	resp, err := server.UpdateProgress(context.Background(), &mangapb.ProgressRequest{
		UserId:         "user-1",
		MangaId:        "one-piece",
		CurrentChapter: 10,
		Status:         "reading",
	})
	require.NoError(t, err)
	require.True(t, resp.GetSuccess())
	require.Equal(t, "progress updated", resp.GetMessage())

	var chapter int
	var status string
	err = db.QueryRow(`SELECT current_chapter, status FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"user-1", "one-piece").Scan(&chapter, &status)
	require.NoError(t, err)
	require.Equal(t, 10, chapter)
	require.Equal(t, "reading", status)
}
