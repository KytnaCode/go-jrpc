package jrpc

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
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
func (s *Server) ServeConn(ctx context.Context, conn io.ReadWriteCloser) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	panic("not implemented")
}

// parseError returns a JSON-RPC response with a parse error.
func parseError(err error) *Response {
	data := ""

	if err != nil {
		data = err.Error()
	}

	return &Response{
		JSONRPC: JsonRPCVersion,
		Error: &Error{
			Code:    ParseError,
			Message: "Parse error",
			Data:    data,
		},
	}
}

// handleBatchRPC handles concurrently a batch of requests, the order of the responses can be different from the order of the requests,
// the response's ID must be used to match the request's ID.
func (s *Server) handleBatchRPC(msg *json.RawMessage) ([]Response, error) {
	type result struct {
		res *Response
		err error
	}

	var reqs []Request // Batch of requests.

	if err := json.Unmarshal(*msg, &reqs); err != nil { // Unmarshal batch of requests.
		return nil, err
	}

	resCh := make(chan result, len(reqs)) // Channel of results.

	var wg sync.WaitGroup
	wg.Add(len(reqs))

	for _, req := range reqs {
		go func(req Request) {
			defer wg.Done()

			var res Response

			err := s.handleRPC(&req, &res) // Handle request.

			resCh <- result{&res, err}
		}(req)
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	res := make([]Response, 0, len(reqs))

	for result := range resCh {
		if result.err != nil {
			return nil, result.err
		}

		res = append(res, *result.res)
	}

	return res, nil
}

func (s *Server) handleRPC(_ *Request, _ *Response) error {
	return nil // TODO: not implemented
}
