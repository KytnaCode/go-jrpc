package jrpc

import (
	"net"
	"net/http"
)

type Server struct{}

func NewServer() *Server {
	return &Server{}
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
