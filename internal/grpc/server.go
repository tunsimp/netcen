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
	"mangahub/internal/services"
)

const (
	MethodHealthCheck         = "/mangahub.InternalService/HealthCheck"
	MethodPublishNotification = "/mangahub.InternalService/PublishNotification"
)

type Server struct {
	cfg           config.Config
	notifications *services.NotificationService

	grpcServer *gogrpc.Server
	listener   net.Listener
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

func NewServer(cfg config.Config, notifications *services.NotificationService) *Server {
	encoding.RegisterCodec(jsonCodec{})

	server := &Server{
		cfg:           cfg,
		notifications: notifications,
	}
	server.grpcServer = gogrpc.NewServer()
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

func (s *Server) PublishNotification(_ context.Context, req *PublishNotificationRequest) (*PublishNotificationResponse, error) {
	if err := s.notifications.Publish(strings.TrimSpace(req.MangaID), strings.TrimSpace(req.Message), req.Timestamp); err != nil {
		return nil, err
	}
	return &PublishNotificationResponse{Status: "ok"}, nil
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
