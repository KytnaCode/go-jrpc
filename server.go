package jrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
)

const (
	JsonRPCVersion = "2.0" // Must be "2.0" for all JSON-RPC 2.0 messages.
	ParseError     = -32700
)

var (
	ErrParse = errors.New("failed to parse JSON-RPC message")
)

type Server struct {
	errorLog func(string, ...any) // Log errors.
}

func NewServer(errorLog func(string, ...any)) *Server {
	if errorLog == nil {
		errorLog = func(string, ...any) {}
	}

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

// ServeConn reads and writes JSON-RPC messages from conn.
// It decodes the requests from conn and encodes the responses back to conn.
// Closes conn when done.
func (s *Server) ServeConn(ctx context.Context, conn io.ReadWriteCloser) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg json.RawMessage // Raw JSON-RPC message.

			if err := dec.Decode(&msg); err == io.EOF { // Connection closed.
				return
			} else if err != nil { // Decode error.
				s.errorLog("failed to decode message: %v", err)
				continue
			}

			res, batch, err := s.handleMessage(&msg) // Handle message.
			if err != nil {
				if err := enc.Encode(parseError(err)); err != nil {
					s.errorLog("failed to encode parse error: %v", err)
				}
			}

			// If no responses, don't write anything at all.
			if len(res) == 0 {
				continue
			}

			if batch {
				// For batch requests always encode responses as a batch although there is only one response.
				if err := enc.Encode(res); err != nil {
					s.errorLog("failed to encode batch response: %v", err)
				}
			} else {
				if err := enc.Encode(res[0]); err != nil {
					s.errorLog("failed to encode response: %v", err)
				}
			}
		}
	}
}

// ServeHTTP implements the http.Handler interface.
// Return a 400 status code if a parse error occurs.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(w)

	var msg json.RawMessage // Raw JSON-RPC message.

	if err := dec.Decode(&msg); err != nil {
		s.errorLog("failed to decode message: %v", err)

		w.WriteHeader(http.StatusBadRequest)
		if err := enc.Encode(parseError(err)); err != nil {
			s.errorLog("failed to encode parse error: %v", err)
		}

		return
	}

	res, batch, err := s.handleMessage(&msg) // Handle message.
	if err != nil {
		w.WriteHeader(http.StatusBadRequest) // Parse error.
		if err := enc.Encode(parseError(err)); err != nil {
			s.errorLog("failed to encode parse error: %v", err)
		}

		return
	}

	// If no responses, don't write anything at all.
	if len(res) == 0 {
		return
	}

	if batch {
		// For batch requests always encode responses as a batch although there is only one response.
		if err := enc.Encode(res); err != nil {
			s.errorLog("failed to encode batch response: %v", err)
		}
	} else {
		// Parse errors on batch requests are replied as a non-batch response.
		if res[0].Error != nil && res[0].Error.Code == ParseError {
			w.WriteHeader(http.StatusBadRequest) // Parse error.
		}

		if err := enc.Encode(res[0]); err != nil {
			s.errorLog("failed to encode response: %v", err)
		}
	}
}

// handleMessage takes a raw JSON-RPC message and returns the response(s) to it, return whether it's a batch request or not,
// if not a batch request, the response slice will contain at most one element, if there isn't any response, the slice will be empty.
// Caller must handle the case where the response slice is empty.
// If an error ocurrs responses slice will be nil and batch will be false.
func (s *Server) handleMessage(msg *json.RawMessage) (res []Response, batch bool, err error) {
	trimmedMsg := bytes.TrimLeft(*msg, " \t\n") // Trim leading whitespace.
	if len(trimmedMsg) == 0 {
		return nil, false, errors.New("empty message received")
	}
	firstByte := trimmedMsg[0] // First non-whitespace byte.

	if firstByte == '[' { // Batch request.
		res, err := s.handleBatchRPC(msg)
		if errors.Is(err, ErrParse) {
			return []Response{*parseError(err)}, false, nil // Parse error must be replied as a non-batch response.
		} else if err != nil {
			return nil, false, err
		}

		return res, true, nil
	}

	if firstByte == '{' { // Single request.
		var req Request
		if err := json.Unmarshal(*msg, &req); err != nil {
			// Send parse error response.
			return []Response{*parseError(err)}, false, nil
		}

		var res Response
		if err := s.handleRPC(&req, &res); err != nil {
			return nil, false, err
		}

		return []Response{res}, false, nil
	}

	return []Response{*parseError(nil)}, false, nil
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
		return nil, ErrParse
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
