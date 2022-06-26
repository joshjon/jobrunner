package server

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpczap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	servicepb "github.com/joshjon/jobrunner/proto/gen/service/v1"
)

type Config struct {
	Address string
	Service servicepb.ServiceServer
	TLS     *tls.Config
}

type Server struct {
	address    string
	grpcServer *grpc.Server
	listener   net.Listener
}

func New(config Config, opts ...grpc.ServerOption) (*Server, error) {
	server := &Server{
		address: config.Address,
	}

	opts = append(opts,
		grpc.Creds(credentials.NewTLS(config.TLS)),
		grpc.ChainUnaryInterceptor(
			grpczap.UnaryServerInterceptor(zap.L(), grpczap.WithDurationField(
				func(duration time.Duration) zapcore.Field {
					return zap.Duration("duration", duration)
				},
			)),
			grpcauth.UnaryServerInterceptor(authenticate),
		),
		grpc.ChainStreamInterceptor(
			grpczap.StreamServerInterceptor(zap.L(), grpczap.WithDurationField(
				func(duration time.Duration) zapcore.Field {
					return zap.Duration("duration", duration)
				},
			)),
			grpcauth.StreamServerInterceptor(authenticate),
		),
	)

	srv := grpc.NewServer(opts...)

	lis, err := net.Listen("tcp", server.address)
	if err != nil {
		return nil, err
	}
	server.listener = lis

	servicepb.RegisterServiceServer(srv, config.Service)
	reflection.Register(srv)

	server.grpcServer = srv
	return server, nil
}

func (s *Server) Serve() error {
	return s.grpcServer.Serve(s.listener)
}

func (s *Server) Address() string {
	return s.address
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}

func authenticate(ctx context.Context) (context.Context, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return ctx, status.New(codes.Unknown, "couldn't find peer info").Err()
	}

	if peer.AuthInfo == nil {
		return ctx, status.New(codes.Unauthenticated, "no transport security being used").Err()
	}

	tlsInfo := peer.AuthInfo.(credentials.TLSInfo)
	subject := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName
	ctx = context.WithValue(ctx, subjectContextKey{}, subject)
	return ctx, nil
}
