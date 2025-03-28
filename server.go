package jrpc

import (
	"context"
	"net"
	"net/http"
)

const (
	JsonRPCVersion = "2.0" // Must be "2.0" for all JSON-RPC 2.0 messages.
	ParseError     = -32700
)

type Server struct {
	errorLog func(string, ...any) // Log errors.
}

func NewServer(errorLog func(string, ...any)) *Server {
	return &Server{errorLog: errorLog}
}

func (s *Server) Register(method string, handler any) {
	panic("not implemented")
}

func (s *Server) Accept(ctx context.Context, lis net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := lis.Accept()
			if err != nil {
				return err
			}
			go s.ServeConn(ctx, conn)
		}
	}
}

func (s *Server) ServeConn(ctx context.Context, conn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	panic("not implemented")
}
