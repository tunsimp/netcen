package grpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/encoding"

	"mangahub/internal/config"
	"mangahub/internal/models"
	"mangahub/internal/repository"
	"mangahub/internal/services"
)

const (
	MethodGetManga            = "/mangahub.MangaService/GetManga"
	MethodSearchManga         = "/mangahub.MangaService/SearchManga"
	MethodUpdateProgress      = "/mangahub.MangaService/UpdateProgress"
	MethodHealthCheck         = "/mangahub.InternalService/HealthCheck"
	MethodPublishNotification = "/mangahub.InternalService/PublishNotification"
)

type Server struct {
	cfg           config.Config
	mangaRepo     *repository.MangaRepository
	progress      *services.ProgressService
	notifications *services.NotificationService

	grpcServer *gogrpc.Server
	listener   net.Listener
}

type GetMangaRequest struct {
	MangaID string `json:"manga_id"`
}

type MangaResponse struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	TotalChapters int32    `json:"total_chapters"`
	Description   string   `json:"description"`
}

type SearchRequest struct {
	Query string `json:"query"`
}

type SearchResponse struct {
	Manga []MangaResponse `json:"manga"`
}

type ProgressRequest struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int32  `json:"chapter"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type ProgressResponse struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int32  `json:"chapter"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

type HealthCheckRequest struct {
	Service string `json:"service"`
}

type HealthCheckResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type PublishNotificationRequest struct {
	MangaID   string `json:"manga_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type PublishNotificationResponse struct {
	Status string `json:"status"`
}

type internalServiceAPI interface {
	HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error)
	PublishNotification(context.Context, *PublishNotificationRequest) (*PublishNotificationResponse, error)
}

type mangaServiceAPI interface {
	GetManga(context.Context, *GetMangaRequest) (*MangaResponse, error)
	SearchManga(context.Context, *SearchRequest) (*SearchResponse, error)
	UpdateProgress(context.Context, *ProgressRequest) (*ProgressResponse, error)
}

func NewServer(
	cfg config.Config,
	mangaRepo *repository.MangaRepository,
	progress *services.ProgressService,
	notifications *services.NotificationService,
) *Server {
	encoding.RegisterCodec(jsonCodec{})

	server := &Server{
		cfg:           cfg,
		mangaRepo:     mangaRepo,
		progress:      progress,
		notifications: notifications,
	}
	server.grpcServer = gogrpc.NewServer()

	server.grpcServer.RegisterService(&gogrpc.ServiceDesc{
		ServiceName: "mangahub.MangaService",
		HandlerType: (*mangaServiceAPI)(nil),
		Methods: []gogrpc.MethodDesc{
			{
				MethodName: "GetManga",
				Handler:    getMangaHandler,
			},
			{
				MethodName: "SearchManga",
				Handler:    searchMangaHandler,
			},
			{
				MethodName: "UpdateProgress",
				Handler:    updateProgressHandler,
			},
		},
	}, server)

	server.grpcServer.RegisterService(&gogrpc.ServiceDesc{
		ServiceName: "mangahub.InternalService",
		HandlerType: (*internalServiceAPI)(nil),
		Methods: []gogrpc.MethodDesc{
			{
				MethodName: "HealthCheck",
				Handler:    healthCheckHandler,
			},
			{
				MethodName: "PublishNotification",
				Handler:    publishNotificationHandler,
			},
		},
	}, server)
	return server
}

func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", ":"+s.cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to start grpc listener: %w", err)
	}
	s.listener = listener

	go func() {
		<-ctx.Done()
		s.grpcServer.GracefulStop()
		_ = s.listener.Close()
	}()

	log.Printf("grpc server listening on :%s", s.cfg.GRPCPort)
	if err := s.grpcServer.Serve(listener); err != nil {
		if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Server) HealthCheck(_ context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	_ = req
	return &HealthCheckResponse{
		Status:    "ok",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (s *Server) GetManga(_ context.Context, req *GetMangaRequest) (*MangaResponse, error) {
	mangaID := strings.TrimSpace(req.MangaID)
	if mangaID == "" {
		return nil, services.ErrInvalidMangaID
	}

	manga, err := s.mangaRepo.FindByID(mangaID)
	if err != nil {
		return nil, err
	}
	if manga == nil {
		return nil, services.ErrMangaNotFound
	}

	return toMangaResponse(*manga), nil
}

func (s *Server) SearchManga(_ context.Context, req *SearchRequest) (*SearchResponse, error) {
	results, err := s.mangaRepo.Search(req.Query)
	if err != nil {
		return nil, err
	}

	response := &SearchResponse{
		Manga: make([]MangaResponse, 0, len(results)),
	}
	for _, item := range results {
		response.Manga = append(response.Manga, *toMangaResponse(item))
	}
	return response, nil
}

func (s *Server) UpdateProgress(_ context.Context, req *ProgressRequest) (*ProgressResponse, error) {
	progress, err := s.progress.Upsert(
		req.UserID,
		req.MangaID,
		int(req.Chapter),
		req.Status,
		req.Timestamp,
	)
	if err != nil {
		return nil, err
	}

	return &ProgressResponse{
		UserID:    progress.UserID,
		MangaID:   progress.MangaID,
		Chapter:   int32(progress.CurrentChapter),
		Status:    progress.Status,
		UpdatedAt: progress.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) PublishNotification(_ context.Context, req *PublishNotificationRequest) (*PublishNotificationResponse, error) {
	if err := s.notifications.Publish(strings.TrimSpace(req.MangaID), strings.TrimSpace(req.Message), req.Timestamp); err != nil {
		return nil, err
	}
	return &PublishNotificationResponse{Status: "ok"}, nil
}

func getMangaHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (any, error) {
	in := new(GetMangaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(mangaServiceAPI).GetManga(ctx, in)
	}
	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MethodGetManga,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(mangaServiceAPI).GetManga(ctx, req.(*GetMangaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func searchMangaHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (any, error) {
	in := new(SearchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(mangaServiceAPI).SearchManga(ctx, in)
	}
	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MethodSearchManga,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(mangaServiceAPI).SearchManga(ctx, req.(*SearchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func updateProgressHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (any, error) {
	in := new(ProgressRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(mangaServiceAPI).UpdateProgress(ctx, in)
	}
	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MethodUpdateProgress,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(mangaServiceAPI).UpdateProgress(ctx, req.(*ProgressRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func healthCheckHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (any, error) {
	in := new(HealthCheckRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(internalServiceAPI).HealthCheck(ctx, in)
	}
	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MethodHealthCheck,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(internalServiceAPI).HealthCheck(ctx, req.(*HealthCheckRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func publishNotificationHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (any, error) {
	in := new(PublishNotificationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(internalServiceAPI).PublishNotification(ctx, in)
	}
	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MethodPublishNotification,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(internalServiceAPI).PublishNotification(ctx, req.(*PublishNotificationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func toMangaResponse(manga models.Manga) *MangaResponse {
	return &MangaResponse{
		ID:            manga.ID,
		Title:         manga.Title,
		Author:        manga.Author,
		Genres:        manga.Genres,
		Status:        manga.Status,
		TotalChapters: int32(manga.TotalChapters),
		Description:   manga.Description,
	}
}
