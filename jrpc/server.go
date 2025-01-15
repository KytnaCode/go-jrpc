package jrpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

// Server is JSON RPC server.
type Server struct {
	logger     *slog.Logger
	dispatcher ClientDispatcher // Handles client connections.
}

// NewServer creates a new server with the given logger and dispatcher.
func NewServer(l *slog.Logger, d ClientDispatcher) *Server {
	return &Server{logger: l, dispatcher: d}
}

// ServeListener serves JSON-RPC over a net.Listener.
func (s *Server) ServeListener(ctx context.Context, listener net.Listener) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server stopped: %w", ctx.Err())
		default:
			conn, err := listener.Accept()
			if err != nil {
				s.logger.Error("could not listen for new requests", slog.Any("error", err))

				return fmt.Errorf("could not listen for connections: %w", err)
			}

			go s.dispatcher.Dispatch(ctx, conn) // Handle connection.
		}
	}
}
