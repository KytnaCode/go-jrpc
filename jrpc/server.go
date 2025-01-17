package jrpc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
)

// Server is JSON RPC server.
type Server struct {
	register   CallbackRegister
	logger     *slog.Logger
	dispatcher ClientDispatcher // Handles client connections.
}

// CreateServer creates a new server without logging.
func CreateServer() *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return CreateServerWithLogging(logger)
}

// CreateServerWithLogging creates a new server using a given logger.
func CreateServerWithLogging(logger *slog.Logger) *Server {
	registry := NewRegistry(logger)

	return NewServerWithRegistry(
		logger,
		NewClientDispatcher(
			logger,
			NewCallbackHandler(
				logger,
				registry,
			),
		),
		registry,
	)
}

// NewServer creates a new server with the given logger and dispatcher and a default registry.
// If you don't need a custom ClientDispatcher implementation use CreateServer or CreateServerWithLogging
// instead.
func NewServer(l *slog.Logger, d ClientDispatcher) *Server {
	return &Server{logger: l, dispatcher: d, register: NewRegistry(l)}
}

// NewServerWithRegistry creates a new server with the given logger, dispatcher, and register.
// If you don't need a custom ClientDispatcher or CallbackRegister implementation use
// CreateServer or CreateServerWithLogging instead.
func NewServerWithRegistry(l *slog.Logger, d ClientDispatcher, r CallbackRegister) *Server {
	return &Server{logger: l, dispatcher: d, register: r}
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

func (s *Server) Register(method string, handler any) error {
	if err := s.register.Register(method, handler); err != nil {
		return fmt.Errorf("could not register method: %w", err)
	}

	return nil
}

/*
RegisterMap register a map of methods where the key is method's name and value the handler.
If any method fails to register RegisterMap stops and return the error, all the methods
registered before the failed one, will still registered.
*/
func (s *Server) RegisterMap(methods map[string]any) error {
	for k, v := range methods {
		if err := s.Register(k, v); err != nil {
			return err
		}
	}

	return nil
}
