package grpcserver

import (
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
)

const (
	_defaultShutdownTimeout = 3 * time.Second
)

type Server struct {
	Server          *grpc.Server
	port            uint32
	notify          chan error
	shutdownTimeout time.Duration
}

func New(port uint32, opts ...grpc.ServerOption) (*Server, error) {
	grpcServer := grpc.NewServer(opts...)

	server := &Server{
		Server:          grpcServer,
		port:            port,
		notify:          make(chan error, 1),
		shutdownTimeout: _defaultShutdownTimeout,
	}

	return server, nil
}

func (s *Server) Start() error {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen %d port: %w", s.port, err)
	}

	go func() {
		s.notify <- s.Server.Serve(l)
		close(s.notify)
	}()

	return nil
}

func (s *Server) Notify() <-chan error {
	return s.notify
}

func (s *Server) Shutdown() {
	s.Server.GracefulStop()
}
