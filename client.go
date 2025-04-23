package jrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/kytnacode/go-jrpc/parse"
)

// Errors returned by the client.
var (
	// Server returned an unknown error.
	ErrUnknownError = errors.New("unknown server error")

	// Server returned a response with an ID that does not match any pending call.
	ErrUnmatchedCall = errors.New("response ID does not match any pending call")

	// Client was shutdown.
	ErrClientShutdown = errors.New("client shutdown")

	// Server returned an empty response on a non-notify call.
	ErrEmptyResponse = errors.New("unexpected empty response")

	// An error occurred in one or more requests in a batch call.
	ErrBatch = errors.New("error in batch response")

	// ErrNullID is returned when the server returned a response with a null ID. Returned from [Client.Input] error
	// channel, single requests will remain pending, as is not possible to match the response with the request, for batch
	// requests, if all requests have null IDs, the call will remain pending, but if at least one request has an ID, the
	// client will try to match the response with the request, and the call will be marked as done with this error in
	// [CallState].
	ErrNullID = errors.New("response contain null ID(s)")
)

// Client represents a JSON-RPC client, it is used to make calls to the server, is necessary to call [Client.Input]
// to start reading the responses from the server.
//
// Is safe for concurrent use.
//
// Implements [Generator].
//
// Example:
//
//	  conn, err := net.Dial("tcp", "localhost:8080")
//	  if err != nil {
//	      log.Fatal(err)
//	  }
//
//	  c := jrpc.NewClient(conn)
//	  go c.Input(context.Background(), nil)
//
//	  call := c.Call(jrpc.Call("method").Args(args))
//
//		 if call.Error != nil {
//		     // Handle error
//		 }
//
//	  result := call.Result[0] // Single call, the result will be the element with key 0.
type Client struct {
	seq  atomic.Uint64
	conn io.ReadWriteCloser
	enc  *json.Encoder
	dec  *json.Decoder

	pendingMu sync.Mutex
	pending   map[uint64]*CallState

	closeCh    chan struct{}
	closedOnce sync.Once
	closed     bool
}

// NewClient creates a new [Client] over the given connection, the client may call conn methods concurrently. The
// once the client is created is necessary to call [Client.Input] to start reading the responses from the server:
//
//	errCh := make(chan error, 1)
//
//	c := jrpc.NewClient(conn)
//	go c.Input(contex.Background(), errCh)
//
//	for err := range errCh {
//	    // Handle errors
//	}
func NewClient(conn io.ReadWriteCloser) *Client {
	return &Client{
		conn:    conn,
		enc:     json.NewEncoder(conn),
		dec:     json.NewDecoder(conn),
		pending: make(map[uint64]*CallState),
		closeCh: make(chan struct{}, 1),
	}
}

// Call makes a synchronous call to the server, and returns the result of the call. It blocks until the call is done,
// [CallState].Done will be always closed, and the [CallState].Result will be set to the result of the call. If the
// call itself fails, the [CallState].Error will be set to the error.
//
// [CallState].Result will have only one element, the response to request indexed by 0.
//
// [CallState].Batch will be false.
//
// Example:
//
//	 call := c.Call(jrpc.Call("method").Args(args))
//
//	 // Is not necessary to check <-call.Done, it will be always closed.
//
//	 if call.Error != nil {
//	     // Handle error
//	 }
//
//	result := call.Result[0] // Single call, the result will have key 0.
func (c *Client) Call(data *CallData) *CallState {
	call := c.Go(nil, data)

	return <-call.Done
}

// CallBatch makes a synchronous batch call to the server, and returns the result of the call. It blocks until then
// server responds to all requests, and the call is done. To make asynchronous batch calls, use [Client.GoBatch].
//
// [CallState].Done will be always closed, and the [CallState].Result will be a map of reesponsed indexed by their id.
// If the call itself fails, the [CallState].Error will be set to the error, and the [CallState].Result will be empty.
//
// If the request is a notification, the [CallState].Result slice will not contain a response for that request.
// [CallState].Result may be empty but never nil.
//
// [CallState].Batch will be true.
//
// Example:
//
//	   var id1, id2 uint64
//
//		  call := c.CallBatch(
//		      c.MakeCall("method1").Args(args1).GetID(&id1),
//		      c.MakeCall("method2").Args(args2).GetID(&id2),
//		  )
//
//		  // Is not necessary to check <-call.Done, it will be always closed.
//
//		  if call.Error != nil {
//		      // Handle error
//		  }
//
//		  // Handle responses
//	   result1 := call.Result[id1] // Result of method1.
//	   result2 := call.Result[id2] // Result of method2.
func (c *Client) CallBatch(data ...*CallData) *CallState {
	call := c.GoBatch(nil, data...) // Make the call.

	return <-call.Done // Wait for the call to finish.
}

// Closed returns a channel that will be closed when the client is closed, and all pending calls are done. If arleady
// closed, the channel will be return immediately.
func (c *Client) Closed() <-chan struct{} {
	ch := make(chan struct{})

	go func() {
		<-c.closeCh
		close(ch)
	}()

	return ch
}

// Go makes an asynchronous non-batch call to the server, and returns a [CallState] object. If done is nil, a new
// channel will be allocated, if non-nil, done must be buffered.
//
// If an error occurrs on the client itself, the [CallState].Error will be set to the error, if the error occurs on the
// method call then the error will be in the [CallState].Result response to that request.
//
//	call := c.Go(nil, jrpc.Call("method").Args(args))
//
//	// Wait for the call to finish
//	<-call.Done
//
//	if call.Error != nil {
//	    // Handle error
//	}
//
//	// Handle response
//	result := call.Result[0] // Single call, the result will have key 0.
func (c *Client) Go(done chan *CallState, data *CallData) *CallState {
	call := new(CallState)
	call.ids = make(map[uint64]struct{}, 1)

	if done == nil {
		call.Done = make(chan *CallState, 1)
	} else {
		call.Done = done
	}

	var req Request
	req.JSONRPC = JSONRPCVersion
	req.Method = data.method

	// Get the request ID, and increment the sequence number.
	var seq uint64
	if data.gen != nil {
		data.GetID(&seq) // Get the ID from the generator.
	} else {
		seq = c.Next()
	}

	id := json.Number(strconv.FormatUint(seq, 10))

	if data.notify {
		req.ID = nil // Notification, no ID.
	} else {
		req.ID = &id
		call.ids[seq] = struct{}{} // Store the ID of the request.
	}

	if data.args == nil {
		req.Params = nil // No params.
	} else {
		params, err := json.Marshal(data.args)
		if err != nil {
			call.Error = err
			done <- call

			return call
		}

		req.Params = new(json.RawMessage)
		*req.Params = json.RawMessage(params)
	}

	if data.notify {
		call.Done <- call
	} else {
		// Store the call in the pending map.
		c.pendingMu.Lock()
		defer c.pendingMu.Unlock()

		if c.closed {
			call.Error = fmt.Errorf("failed to send the request: %w", ErrClientShutdown)
			call.Done <- call

			return call
		}

		c.pending[seq] = call
	}

	// Send the request to the server.
	if err := c.enc.Encode(req); err != nil {
		call.Error = err
		call.Done <- call

		delete(c.pending, seq) // Remove the call from the pending map.

		return call
	}

	return call
}

// GoBatch makes an asynchronous batch call to the server, and returns a [CallState] object. The [CallState].Done will
// close when the call is done, and will send the same [CallState]. If an error occurrs on the client itself, the
// [CallState.Error] will be set, if the error occurs on a request, then the error will be set on the
// [CallResult].Error field of the response to that request in the [CallState].Result map:
//
//	var id1, id2 uint64
//
//	call := c.GoBatch(nil,
//	  c.MakeCall("method1").Args(args1).GetID(&id1),
//	  c.MakeCall("method2").Args(args2).GetID(&id2),
//	)
//
//	// Wait for the call to finish
//	<-call.Done
//
//	if call.Error != nil {
//	  // Handle errors
//	}
//
//	// Handle responses
//	result1 := call.Result[id1] // Result of method1.
//	result2 := call.Result[id2] // Result of method2.
func (c *Client) GoBatch(done chan *CallState, data ...*CallData) *CallState {
	call := new(CallState)
	call.ids = make(map[uint64]struct{}, len(data))
	call.Batch = true

	if done != nil {
		call.Done = done
	} else {
		call.Done = make(chan *CallState, 1)
	}

	reqs := make([]Request, 0, len(data))

	seqs := make([]uint64, 0, len(data)) // IDs of all requests.

	for _, d := range data {
		var req Request
		req.JSONRPC = JSONRPCVersion
		req.Method = d.method

		if d.notify {
			req.ID = nil // Notification, no ID.
		} else {
			var seq uint64
			if d.gen != nil {
				d.GetID(&seq) // Get the ID from the generator.
			} else {
				seq = c.Next()
			}

			id := json.Number(strconv.FormatUint(seq, 10))

			req.ID = &id

			call.ids[seq] = struct{}{} // Store the ID of the request.

			seqs = append(seqs, seq) // Store the ID of the request.
		}

		if d.args == nil {
			req.Params = nil
		} else {
			params, err := json.Marshal(d.args)
			if err != nil {
				call.Error = err
				call.Done <- call

				return call
			}

			req.Params = new(json.RawMessage)
			*req.Params = json.RawMessage(params)
		}

		reqs = append(reqs, req)
	}

	// All requests are mapped to the same call.
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	if c.closed {
		call.Error = fmt.Errorf("failed to send the request: %w", ErrClientShutdown)
		call.Done <- call

		return call
	}

	for _, seq := range seqs {
		c.pending[seq] = call
	}

	if err := c.enc.Encode(reqs); err != nil {
		call.Error = err
		call.Done <- call

		for _, seq := range seqs {
			delete(c.pending, seq) // Remove the call from the pending map.
		}

		return call
	}

	return call
}

// Input reads the responses from the server, and sends them to the [CallState] objects. It blocks until the
// connection is closed, an error occurs, or the context is cancelled. If an error occurs, it will be sent to the
// error channel, and all pending calls will be marked as done with its [CallState].Error set to [ErrClientShutdown].
// Is mandatory to call this method after creating the client, otherwise the client will not be able to get the
// responses from the server.
//
// Is usually called in a goroutine:
//
//	errCh := make(chan error, 1)
//
//	c := jrpc.NewClient(conn)
//	go c.Input(context.Background(), errCh)
//
//	for err := range errCh {
//	    // Handle errors
//	}
func (c *Client) Input(ctx context.Context, errCh chan error) {
	var m msg

	var err error

	go func() {
		<-ctx.Done()

		err := c.conn.Close()
		if err != nil {
			errCh <- fmt.Errorf("failed to close connection: %w", err)
		}
	}()

	for err == nil {
		select {
		case <-ctx.Done():
			err = fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
			if err = c.dec.Decode(&m); err != nil {
				if !errors.Is(err, io.EOF) {
					errCh <- fmt.Errorf("failed to decode message: %w", err)
				}

				continue
			}

			if len(m.res) == 0 {
				errCh <- fmt.Errorf("empty JSON-RPC message: %w", ErrEmptyResponse)

				continue
			}

			if !m.batch && m.res[0].ID == nil {
				errCh <- ErrNullID

				continue
			}

			if m.batch {
				var missing bool

				var batchID *uint64

				// Check if an ID is missing in the batch response.
				nullIDCount := 0

				for _, res := range m.res {
					if res.ID == nil {
						nullIDCount++
						missing = true

						continue
					}

					id, err := strconv.ParseUint(res.ID.String(), 10, 64)
					if err != nil {
						errCh <- fmt.Errorf("failed to parse ID: %w", err)

						missing = true
					}

					batchID = &id // If at least one ID is present, we can use it to find the call and resolve it.
				}

				if nullIDCount > 0 {
					errCh <- fmt.Errorf("batch response contains %d null IDs: %w", nullIDCount, ErrNullID)
				}

				// If at least one ID is present, resolve the call.
				if missing && batchID != nil {
					c.pendingMu.Lock()

					call, ok := c.pending[*batchID]
					if !ok {
						continue
					}

					call.Error = fmt.Errorf(
						"response contain null IDs, some requests may be invalid: %w",
						ErrNullID,
					)
					call.Done <- call

					for id := range call.ids {
						delete(c.pending, id)
					}
				}

				if missing {
					errCh <- resToError(m.res[0])

					continue
				}
			}

			id, err := strconv.ParseUint(m.res[0].ID.String(), 10, 64)
			if err != nil {
				errCh <- fmt.Errorf("failed to parse ID: %w", err)

				continue
			}

			c.pendingMu.Lock()

			call, ok := c.pending[id]
			if ok {
				// Batch calls are mapped to the same call, so we need to remove all of them from the pending map.
				for _, res := range m.res {
					id, err := strconv.ParseUint(res.ID.String(), 10, 64)
					if err != nil {
						call.Done <- call

						errCh <- fmt.Errorf("failed to parse ID: %w", err)
						c.pendingMu.Unlock()

						continue
					}

					delete(c.pending, id)
				}
			} else {
				errCh <- fmt.Errorf("failed to find call with ID %d: %w", id, ErrUnmatchedCall)
				c.pendingMu.Unlock()

				continue
			}

			c.pendingMu.Unlock()

			results := make(map[uint64]CallResult, len(m.res))

			if !m.batch && m.res[0].Error != nil {
				call.Error = resToError(m.res[0])
				call.Result = results
				call.Done <- call

				continue
			}

			var batchErr bool

			for _, res := range m.res {
				var id uint64

				if m.batch {
					id, err = strconv.ParseUint(res.ID.String(), 10, 64)
					if err != nil {
						errCh <- fmt.Errorf("failed to parse ID: %w", err)

						batchErr = true
						results[id] = CallResult{
							Error: fmt.Errorf("failed to parse ID: %w", ErrParse),
						}

						continue
					}
				} else {
					id = 0
				}

				if res.Error != nil {
					batchErr = true
					results[id] = CallResult{
						Error: resToError(res),
					}

					continue
				}

				results[id] = CallResult{
					Result: res.Result,
				}
			}

			if batchErr {
				call.Error = fmt.Errorf("error in batch response: %w", ErrBatch)
			}

			call.Result = results
			call.Done <- call
		}
	}

	// Close the connection and mark all pending calls as done.
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()

	responded := make(map[*CallState]struct{})

	for id, call := range c.pending {
		// Batch calls are mapped to the same call, so we only need to close the call once.
		if _, ok := responded[call]; !ok {
			call.Error = fmt.Errorf("failed to close connection: %w", ErrClientShutdown)
			call.Done <- call
		}

		delete(c.pending, id)

		responded[call] = struct{}{}
	}

	c.closedOnce.Do(func() {
		close(c.closeCh)
		c.closed = true
	})
}

// MakeCall behaves like [Call] but sets the generator to an autoincrementing ID:
//
//	// jrpc.Call("method").GenID(c) is equivalent to
//	c.MakeCall("method")
func (c *Client) MakeCall(method string) *CallData {
	return Call(method).GenID(c)
}

// Next returns an incrementing ID starting from 0. Implements [Generator].
func (c *Client) Next() uint64 {
	return c.seq.Add(1) - 1
}

// Generator defines the interface for generate IDs for the calls. The [Client] implements this interface.
type Generator interface {
	Next() uint64 // Generate the next ID.
}

// CallState contains the state of a call to the server, If an error occurrs on the whole process,
// (ex. invalid batch json) the [CallState.Error] will be set, if the error occurs on a single call (ex. invalid ID,
// invalid params, etc), then the error will be set on the [CallResult.Error] field of the [CallState.Result] map.
//
// [CallState.Result] is a map of the responses to the requests, the key is the ID of the request and the value is a
// [CallResult] object, if a request is a notification, the [CallResult] will not contain a response for that request.
// For non-batch calls, the [CallState.Result] will contain only one element, and the key will be 0.
//
// [CallState.Batch] is true for all batch calls, and false for single calls.
//
// [CallState.Done] is a channel that will be closed when the call is done, and will contain the [CallState] itself.
//
// Example:
//
//	  c := jrpc.NewClient(conn)
//	  go c.Input(ctx, errCh)
//
//	  // call.Batch = false
//	  call := c.Go(jrpc.Call("method").Args(args))
//
//	  // Wait for the call to finish
//	  replyCall := <-call.Done // replyCall is equal to call.
//
//	  if call.Error != nil {
//	      // Handle error
//	  }
//
//	  // Handle responses
//		res := call.Result[0] // Single call, the result will be the element with key 0.
type CallState struct {
	// Error is set if the call itself failed, for example if the connection was closed.
	Error error

	// Result contains the result of all non-notification requests, the key is the ID of the request, see [CallData.GetID].
	Result map[uint64]CallResult

	// Batch will be true if the call is a batch call, and false if the call is a single call.
	Batch bool

	// Done is a channel that will be closed when the call is done, and will contain the [CallState] itself.
	Done chan *CallState

	ids map[uint64]struct{}
}

// CallResult contains the result of a call to the server, if successful, the [CallResult.Error] will be nil, and
// the [CallResult.Result] will contain the result of the call, if the call failed, the [CallResult.Error] will
// contain the error, and the [CallResult.Result] will be nil.
type CallResult struct {
	// Result is the result of the request.
	Result any

	// Error is the error of the request.
	Error error
}

// CallData contains the method and arguments to be sent to the server, is used by [Client] to make a call to the
// server. To create a new CallData, use [Call] function. To set the arguments, use [CallData.Args] method:
//
//	call := jrpc.Call("method").Args(args)
//	call := jrpc.Call("method").Args([]string{"arg1", "arg2"})
type CallData struct {
	method string
	args   any
	id     *uint64
	gen    Generator
	notify bool
}

// Args sets the arguments for the call, and returns the [CallData] object itself:
//
//	call := jrpc.Call("method").Args(args)
func (c *CallData) Args(args any) *CallData {
	c.args = args

	return c
}

// Notify makes the request a notification, a request that does not expect a response from the server. In non-batch
// calls [CallState].Done will be never closed, in batch calls [CallState].Done will be closed if there's at least
// one non-notification request in the batch.
func (c *CallData) Notify() *CallData {
	c.notify = true

	return c
}

// GenID sets the generator for the call, and returns the [CallData] object itself. [Client] implements the [Generator]
// interface, so you can use it to generate IDs for the calls:
//
//	var id uint64
//
//	call := jrpc.Call("method").GenID(c).GetID(&id).Args(args)
//
//	// id will contain the ID for the request.
//
// To avoid setting id generator for each call you can use [Client.MakeCall] method:
//
//	var id uint64
//
//	call := c.MakeCall("method").ID(&id).Args(args)
//
//	// id will contain the ID for the request.
func (c *CallData) GenID(gen Generator) *CallData {
	c.gen = gen

	return c
}

// GetID sets id to the ID of the call, and returns the [CallData] object itself. If the ID is not set, it will be
// generated. GetID should be called after [CallData.GenID] unless you are using the [Client.MakeCall] method.
func (c *CallData) GetID(id *uint64) *CallData {
	if c.id == nil {
		c.id = new(uint64)
		*c.id = c.gen.Next()
	}

	*id = *c.id

	return c
}

// Method overrides the method of the call, and returns the [CallData] object itself:
//
//	call := jrpc.Call("method").Method("new_method").Args(args) // Method will be "new_method"
func (c *CallData) Method(method string) *CallData {
	c.method = method

	return c
}

// Custom unmarshaler for the Response type, to handle both single and batch responses.
type msg struct {
	res   []Response
	batch bool
}

func (msg *msg) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("empty JSON-RPC message: %w", ErrEmptyRequest)
	}

	switch b[0] {
	case '[': // Batch response
		msg.batch = true

		if err := json.Unmarshal(b, &msg.res); err != nil {
			return fmt.Errorf("failed to unmarshal batch response: %w", err)
		}

		return nil
	case '{': // Single response.
		var m Response
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("failed to unmarshal single response: %w", err)
		}

		msg.res = make([]Response, 1)
		msg.res[0] = m

		return nil
	}

	return fmt.Errorf("invalid JSON-RPC message: %w", ErrParse)
}

// Call wraps the necessary data to make a call to the server into a [CallData] object, and pass it to the [Client] to
// make a call to the server:
//
//	  // Single call.
//	  client.Call(jrpc.Call("method").Args(args))
//
//		 // Batch call.
//	  client.CallBatch(
//	      jrpc.Call("method1").Args(args1),
//	      jrpc.Call("method2").Args(args2),
//	  )
func Call(method string) *CallData {
	return &CallData{
		method: method,
	}
}

// Maps error responses to an error type.
func resToError(res Response) error {
	switch res.Error.Code {
	case ParseError:
		return fmt.Errorf("parse error: %v: %w", res.Error.Message, ErrParse)
	case InvalidRequest:
		return fmt.Errorf(
			"invalid request: %v: %w",
			res.Error.Message,
			ErrInvalidRequest)
	case MethodNotFound:
		return fmt.Errorf(
			"method not found: %v: %w",
			res.Error.Message,
			ErrMethodNotFound,
		)
	case InvalidParams:
		return fmt.Errorf(
			"invalid params: %v: %w",
			res.Error.Message,
			parse.ErrInvalidParams,
		)
	case InternalError:
		return fmt.Errorf(
			"internal error: %v: %w",
			res.Error.Message,
			ErrInternalError,
		)
	}

	return fmt.Errorf(
		"unknown error: %v: %w",
		res.Error.Message,
		ErrUnknownError,
	)
}
