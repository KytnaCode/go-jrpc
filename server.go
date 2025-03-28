package jrpc

import (
	"net"
	"net/http"
)

const JsonRPCVersion = "2.0" // Must be "2.0" for all JSON-RPC 2.0 messages.

type Server struct {
	errorLog func(string, ...any) // Log errors.
}

func NewServer(errorLog func(string, ...any)) *Server {
	return &Server{errorLog: errorLog}
}

func (s *Server) Register(method string, handler any) {
	panic("not implemented")
}

func (s *Server) Accept(lis net.Listener) error {
	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}
		go s.ServeConn(conn)
	}
}

func (s *Server) ServeConn(conn net.Conn) {
	panic("not implemented")
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	panic("not implemented")
}
