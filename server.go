package jrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/kytnacode/go-jrpc/internal/jsonutil"
	"github.com/kytnacode/go-jrpc/parse"
)

// JSONRPCVersion must be "2.0" for all JSON-RPC 2.0 messages.
const JSONRPCVersion = "2.0"

// Server is the entry point for the JSON-RPC server, it allows registering methods and handling requests via HTTP, a
// listener, or a connection:
//
//	  server := jrpc.NewServer(log.Printf)
//
//	  err := server.Register("method", func(args struct{ A int }, reply *struct{ B int }) error {
//	    reply.B = args.A + 1
//
//	    return nil
//	  })
//	  if err != nil {
//	    log.Fatal(err)
//	  }
//
//	  // via HTTP
//	  http.Handle("/rpc", server)
//	  log.Fatal(http.ListenAndServe(":8080", nil))
//
//	  // or via a Listener
//	  lis, err := net.Listen("tcp", ":8080")
//	  if err != nil {
//			 log.Fatal(err)
//	  }
//
//	  err = server.Accept(context.Background(), lis)
//	  if err != nil {
//	    log.Fatal(err)
//	  }
//
// Implements [Register], [Caller], [MethodRegister] and [http.Handler] interfaces.
type Server struct {
	registry MethodRegister       // Registry of methods.
	errorLog func(string, ...any) // Log errors.
}

// NewServer creates a new Server with the given error log function.
// If errorLog is nil, it will be a no-op function.
func NewServer(errorLog func(string, ...any)) *Server {
	if errorLog == nil {
		errorLog = func(string, ...any) {}
	}

	return &Server{errorLog: errorLog, registry: NewRegistry()}
}

// SetRegistry allows to use a custom [MethodRegister] implementation, if nil the default [Registry] will be used.
func (s *Server) SetRegistry(registry MethodRegister) {
	if registry == nil {
		s.registry = NewRegistry()

		return
	}

	s.registry = registry
}

// Register registers a method with the given name and handler, handler must be a function that satisfies these
// criteria:
//   - Must take two arguments.
//   - The first argument must be the method's arguments type.
//   - The second argument must be a pointer to the method's reply type.
//   - Both arguments and the reply type must be a struct or a pointer to a struct.
//   - The handler must return an error.
//
// The handler must look like this:
//
//	func(args ArgsType, reply *ReplyType) error
//
// Implements the [Register] interface.
// Is safe for concurrent use.
func (s *Server) Register(method string, handler any) error {
	if s.registry.Register(method, handler) != nil {
		return fmt.Errorf("failed to register method %q: %w", method, ErrInvalidHandlerType)
	}

	return nil
}

// Accept accepts connections from the given listener and serves them using the [ServeConn] method, it blocks until the
// context is done or an error occurs on the listener:
//
//	  lis, err := net.Listen("tcp", ":8080")
//	  if err != nil {
//			 log.Fatal(err)
//	  }
//
//	  err = server.Accept(context.Background(), lis)
//	  if err != nil {
//	    log.Fatal(err)
//	  }
func (s *Server) Accept(ctx context.Context, lis net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("accepting connections cancelled: %w", ctx.Err())
		default:
			conn, err := lis.Accept()
			if err != nil {
				return fmt.Errorf("failed to accept connection: %w", err)
			}

			go s.ServeConn(ctx, conn)
		}
	}
}

// ServeConn reads requests from conn and writes responses to it, it blocks until the context is done or an error
// occurs, it closes conn when done, conn must be safe for concurrent use:
//
//	  conn, err := net.Dial("tcp", ":8080")
//	  if err != nil {
//			 log.Fatal(err)
//	  }
//
//	  err = server.ServeConn(context.Background(), conn)
//		 if err != nil {
//			 log.Fatal(err) // An error occurred or the context was cancelled.
//		 }
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

// ServeHTTP implements the [http.Handler] interface.
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
		if errObj := res[0].Error; errObj != nil {
			if errObj.Code == ParseError {
				w.WriteHeader(http.StatusBadRequest) // Parse error.
			}

			if errObj.Code == InvalidRequest {
				w.WriteHeader(http.StatusBadRequest) // Invalid request.
			}

			if errObj.Code == MethodNotFound {
				w.WriteHeader(http.StatusNotFound) // Method not found.
			}

			if errObj.Code == InternalError {
				w.WriteHeader(http.StatusInternalServerError) // Internal error.
			}

			if errObj.Code == InvalidParams {
				w.WriteHeader(http.StatusBadRequest) // Invalid params.
			}
		}

		if err := enc.Encode(res[0]); err != nil {
			s.errorLog("failed to encode response: %v", err)
		}
	}
}

// handleMessage takes a raw JSON-RPC message and returns the response(s) to it, return whether it's a batch request
// or not, if not a batch request, the response slice will contain at most one element, if there isn't any response,
// the slice will be empty. Caller must handle the case where the response slice is empty. If an error occurs
// responses slice will be nil and batch will be false.
func (s *Server) handleMessage(msg *json.RawMessage) (res []Response, batch bool, err error) {
	trimmedMsg := jsonutil.TrimLeftWhitespace(*msg) // Trim leading whitespace.
	if len(trimmedMsg) == 0 {                       // Empty message.
		return nil, false, ErrEmptyRequest
	}

	firstByte := trimmedMsg[0] // First non-whitespace byte.

	if firstByte == '[' { // Batch request.
		res, err := s.handleBatchRPC(msg)
		if errors.Is(err, ErrParse) {
			return []Response{
				*parseError(err),
			}, false, nil // Parse error must be replied as a non-batch response.
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

		s.handleRPC(&req, &res)

		if res.noreply {
			return []Response{}, false, nil
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

	res := new(Response)

	rpcError(ParseError, "Parse error", data, res)

	return res
}

// invalidRequest sets response's error to an invalid request error. data may be nil.
func invalidRequest(data any, res *Response) {
	rpcError(InvalidRequest, "Invalid request", data, res)
}

// internalError sets response's error to an internal error. data may be nil.
func internalError(data any, res *Response) {
	rpcError(InternalError, "Internal error", data, res)
}

// methodNotFound sets response's error to a method not found error.
func methodNotFound(method string, res *Response) {
	rpcError(MethodNotFound, "Method not found", method, res)
}

// rpcError sets response's error to a custom error. data may be nil.
func rpcError(code int, message string, data any, res *Response) {
	res.Error = &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// handleBatchRPC handles concurrently a batch of requests, the order of the responses can be different from the order
// of the requests, the response's ID must be used to match the request's ID.
func (s *Server) handleBatchRPC(msg *json.RawMessage) ([]Response, error) {
	var reqs []Request // Batch of requests.

	if err := json.Unmarshal(*msg, &reqs); err != nil { // Unmarshal batch of requests.
		return nil, ErrParse
	}

	resCh := make(chan *Response, len(reqs)) // Channel of results.

	var wg sync.WaitGroup

	wg.Add(len(reqs))

	for _, req := range reqs {
		go func(req Request) {
			defer wg.Done()

			var res Response

			s.handleRPC(&req, &res) // Handle request.

			resCh <- &res
		}(req)
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	responses := make([]Response, 0, len(reqs))

	for res := range resCh {
		if res.noreply { // Don't include responses that are notifications.
			continue
		}

		responses = append(responses, *res)
	}

	return responses, nil
}

// handleRPC handles a single request.
func (s *Server) handleRPC(req *Request, res *Response) {
	if req.ID == nil { // A notification, don't send a response.
		res.noreply = true
	} else {
		res.ID = req.ID // Set response ID.
	}

	if err := validateRequest(req); errors.Is(err, ErrInvalidRequest) {
		invalidRequest(err, res) // validateRequest don't expose internal errors.

		return
	} else if err != nil {
		internalError(err, res) // validateRequest don't expose internal errors.

		return
	}

	res.JSONRPC = JSONRPCVersion // Must be "2.0" for all JSON-RPC 2.0 messages.

	paramsT, err := s.registry.MethodParamsType(req.Method)
	if errors.Is(err, ErrMethodNotFound) {
		methodNotFound(req.Method, res)

		return
	} else if err != nil {
		internalError(nil, res) // Don't expose internal errors.

		return
	}

	var params any

	if req.Params != nil {
		params, err = parse.ParamsType(paramsT, *req.Params) // Parse params.
		if err != nil {
			internalError(nil, res)

			return
		}
	}

	result, err := s.registry.Call(req.Method, params) // Call method.

	if errors.Is(err, ErrMethodNotFound) {
		methodNotFound(req.Method, res)

		return
	} else if err != nil {
		internalError(nil, res)

		return
	}

	if res.noreply {
		return
	}

	res.Result = result // Set result.
}

// validateRequest validates a request. Returns an ErrInvalidRequest error if the request is invalid.
// Don't check for the method existence, it's the responsibility of the caller.
// Don't validate the params, it's the responsibility of the caller.
// Only allow JSON-RPC 2.0 messages.
// Don't expose internal errors.
func validateRequest(req *Request) error {
	if req.JSONRPC != JSONRPCVersion {
		return fmt.Errorf(
			"only JSON-RPC %q is supported, got %v: %w",
			JSONRPCVersion,
			req.JSONRPC,
			ErrInvalidRequest,
		)
	}

	if req.Method == "" {
		return fmt.Errorf("method is empty: %w", ErrInvalidRequest)
	}

	return nil
}
