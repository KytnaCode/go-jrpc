package jrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kytnacode/go-jrpc"
)

// mock io.ReadWriteCloser.
type ioReadWriteCloser struct {
	net.Conn

	r      io.Reader
	w      io.Writer
	closed bool
}

func (ioReadWriteCloser *ioReadWriteCloser) Write(p []byte) (n int, err error) {
	if ioReadWriteCloser.closed {
		return 0, errors.New("connection closed")
	}

	return ioReadWriteCloser.w.Write(p)
}

func (ioReadWriteCloser *ioReadWriteCloser) Read(p []byte) (n int, err error) {
	if ioReadWriteCloser.closed {
		return 0, errors.New("connection closed")
	}

	return ioReadWriteCloser.r.Read(p)
}

func (rwc *ioReadWriteCloser) Close() error {
	rwc.closed = true

	return nil
}

func newIOReadWriteCloser(r io.Reader, w io.Writer) *ioReadWriteCloser {
	return &ioReadWriteCloser{r: r, w: w, closed: false, Conn: nil}
}

type listener struct {
	net.Listener
}

// Accept waits for and returns the next connection to the listener.
func (listener *listener) Accept() (net.Conn, error) {
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1}`
	r := strings.NewReader(request)

	return newIOReadWriteCloser(r, io.Discard), nil
}

func TestServer_ServeConnInvalidJSON(t *testing.T) {
	t.Parallel()

	const invalidJSON = `{"jsonrpc": "2.0", "method": "foo", "params": "bar", "id": 1` // missing closing brace

	r, w := io.Pipe() // Create a pipe to simulate a connection.

	conn := newIOReadWriteCloser(strings.NewReader(invalidJSON), w)

	testSucceedc := make(chan struct{}) // Expect request to fail
	testFailc := make(chan error)       // Test will fail if the request succeeds or an error occurs.

	errorLog := func(string, ...any) {
		testSucceedc <- struct{}{} // Error log should be called.
	}

	s := jrpc.NewServer(errorLog)

	go s.ServeConn(context.Background(), conn)

	go func() {
		if err := json.NewDecoder(r).Decode(&struct{}{}); err != nil {
			testFailc <- err
		}
	}()

	select {
	case <-testSucceedc:
	case err := <-testFailc:
		t.Fatalf("expected invalid JSON: %v", err)
	}
}

func TestServer_ServeConnValidBatch(t *testing.T) {
	t.Parallel()

	// A valid batch request.
	const batch = `[
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1 },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 2 },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 3 }
	]`

	r, w := io.Pipe() // Create a pipe to simulate a connection.

	conn := newIOReadWriteCloser(strings.NewReader(batch), w)

	s := jrpc.NewServer(nil)

	go s.ServeConn(context.Background(), conn)

	// Wait for the server to finish processing the request.
	if err := json.NewDecoder(r).Decode(&[]any{}); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}
}

func TestServer_ServeConnInvalidBatch(t *testing.T) {
	t.Parallel()

	// An invalid batch request.
	const batch = `[
	  { "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1 }
	  { "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 2 }
		]` // missing comma between the two requests.
	r, w := io.Pipe() // Create a pipe to simulate a connection.

	conn := newIOReadWriteCloser(strings.NewReader(batch), w)

	testSucceedc := make(chan struct{}) // Expect request to fail.
	testFailc := make(chan error)       // Test will fail if the request succeeds or an error occurs.

	errorLog := func(string, ...any) {
		testSucceedc <- struct{}{} // Error log should be called.
	}

	s := jrpc.NewServer(errorLog)

	go s.ServeConn(context.Background(), conn)

	go func() {
		if err := json.NewDecoder(r).Decode(&[]any{}); err != nil {
			testFailc <- err
		}
	}()

	select {
	case <-testSucceedc:
	case err := <-testFailc:
		t.Fatalf("expected invalid JSON: %v", err)
	}
}

func TestServer_ServeConnValidSingle(t *testing.T) {
	t.Parallel()

	// A valid single request.
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1}`

	r, w := io.Pipe() // Create a pipe to simulate a connection.

	conn := newIOReadWriteCloser(strings.NewReader(request), w)

	s := jrpc.NewServer(nil)

	go s.ServeConn(context.Background(), conn)

	// Wait for the server to finish processing the request.
	var res any
	if err := json.NewDecoder(r).Decode(&res); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}
}

func TestServer_ServeConnInvalidSingle(t *testing.T) {
	t.Parallel()

	// An invalid single request.
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": , "id": 1}` // params have no value.

	r, w := io.Pipe() // Create a pipe to simulate a connection.

	conn := newIOReadWriteCloser(strings.NewReader(request), w)

	testSucceedc := make(chan struct{}) // Expect request to fail.
	testFailc := make(chan error)       // Test will fail if the request succeeds or an error occurs.

	errorLog := func(string, ...any) {
		testSucceedc <- struct{}{} // Error log should be called.
	}

	s := jrpc.NewServer(errorLog)

	go s.ServeConn(context.Background(), conn)

	go func() {
		if err := json.NewDecoder(r).Decode(&struct{}{}); err != nil {
			testFailc <- err
		}
	}()

	select {
	case <-testSucceedc:
	case err := <-testFailc:
		t.Fatalf("expected invalid JSON: %v", err)
	}
}

func TestServer_ServeConnShouldCloseConn(t *testing.T) {
	t.Parallel()

	// A valid single request.
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1}`

	conn := newIOReadWriteCloser(strings.NewReader(request), io.Discard)

	s := jrpc.NewServer(nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.ServeConn(ctx, conn)

	if !conn.closed {
		t.Fatalf("expected connection to be closed")
	}
}

func TestServer_ServeConnShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	// A valid single request.
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1}`

	s := jrpc.NewServer(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 10

	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			conn := newIOReadWriteCloser(strings.NewReader(request), io.Discard)
			s.ServeConn(ctx, conn)
			wg.Done()
		}()
	}

	cancel()

	wg.Wait()
}

func TestServer_ServeHTTPInvalidJSON(t *testing.T) {
	t.Parallel()

	const invalidJSON = `{"jsonrpc": "2.0", "method": "foo", "params": "bar", "id": 1` // missing closing brace

	r := strings.NewReader(invalidJSON)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestServer_ServeHTTPValidBatch(t *testing.T) {
	t.Parallel()

	// A valid batch request.
	const batch = `[
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1 },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 2 },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 3 }
	]`

	r := strings.NewReader(batch)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestServer_ServeHTTPInvalidBatch(t *testing.T) {
	t.Parallel()

	// An invalid batch request.
	const batch = `[
	  { "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1 }
	  { "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 2 }
		]` // missing comma between the two requests.

	r := strings.NewReader(batch)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestServer_ServeHTTPValidSingle(t *testing.T) {
	t.Parallel()

	// A valid single request.
	const request = `{"jsonrpc": "2.0", "method": "foo", "id": 1}`

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	if err := s.Register("foo", func(args, reply *struct{}) error { return nil }); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestServer_ServeHTTPInvalidSingle(t *testing.T) {
	t.Parallel()

	// An invalid single request.
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": , "id": 1}` // params have no value.

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestServer_ServeHTTPNonObjectNonBatch(t *testing.T) {
	t.Parallel()

	// A non-object, non-batch request.
	const request = `5` // A number.

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestServer_ServeHTTPShouldNotWriteResponseToASingleNotification(t *testing.T) {
	t.Parallel()

	// A single notification.
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ]}` // No id.

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.Len() != 0 {
		t.Fatalf("expected no response, got %s", w.Body.String())
	}
}

func TestServer_ServeHTTPShouldNotWriteResponseToABatchOfNotifications(t *testing.T) {
	t.Parallel()

	// A batch of notifications.
	const batch = `[
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ] },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ] }
		]` // No id.

	r := strings.NewReader(batch)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.Len() != 0 {
		t.Fatalf("expected no response, got %s", w.Body.String())
	}

}

func TestServer_ServeHTTPShouldReturnABatchOfResponsesExcludingNotifications(t *testing.T) {
	t.Parallel()

	const expectedRequests = 2 // Without notifications.

	// A batch of requests with notifications.
	const batch = `[
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1 },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ] },
		{ "jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 2 }
	]` // One notification and two requests.

	r := strings.NewReader(batch)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var res []any
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	if len(res) != expectedRequests {
		t.Fatalf("expected %d responses, got %d", expectedRequests, len(res))
	}
}

func TestServer_ServeHTTPShouldNotPanicWithNilParams(t *testing.T) {
	t.Parallel()

	const request = `{"jsonrpc": "2.0", "method": "foo", "params": null, "id": 1}`

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	if err := s.Register("foo", func(args, reply *struct{}) error { return nil }); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestServer_ServeHTTPShouldNotPanicWithoutParams(t *testing.T) {
	t.Parallel()

	const request = `{"jsonrpc": "2.0", "method": "foo", "id": 1}`

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	if err := s.Register("foo", func(args, reply *struct{}) error { return nil }); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestServer_ServeHTTPMissingJSONRPCVersion(t *testing.T) {
	t.Parallel()

	const request = `{"method": "foo", "params": [ "bar" ], "id": 1}` // Missing jsonrpc version.

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var res jrpc.Response
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	if res.Error == nil {
		t.Fatalf("expected an error, got nil")
	}

	if res.Error.Code != jrpc.InvalidRequest {
		t.Fatalf("expected error code %d, got %d", jrpc.InvalidRequest, res.Error.Code)
	}
}

func TestServer_ServeHTTPEmptyMethodShouldReturnBadRequest(t *testing.T) {
	t.Parallel()

	const request = `{"jsonrpc": "2.0", "method": "", "params": [ "bar" ], "id": 1}` // Empty method.

	r := strings.NewReader(request)

	req := &http.Request{
		Body: io.NopCloser(r),
	}

	w := httptest.NewRecorder()

	s := jrpc.NewServer(nil)

	s.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var res jrpc.Response
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	if res.Error == nil {
		t.Fatalf("expected an error, got nil")
	}

	if res.Error.Code != jrpc.InvalidRequest {
		t.Fatalf("expected error code %d, got %d", jrpc.InvalidRequest, res.Error.Code)
	}
}

func TestServer_ServeHTTPShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1}`

	s := jrpc.NewServer(nil)

	const n = 10

	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			r := strings.NewReader(request)

			req := &http.Request{
				Body: io.NopCloser(r),
			}

			w := httptest.NewRecorder()

			s.ServeHTTP(w, req)

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestServer_AcceptShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 10

	var wg sync.WaitGroup
	wg.Add(n)

	errCh := make(chan error)

	for range n {
		go func() {
			l := &listener{}

			s := jrpc.NewServer(nil)

			err := s.Accept(ctx, l)
			if err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}

			wg.Done()
		}()
	}

	cancel() // Cancel the context to stop the servers.

	doneCh := make(chan struct{})

	go func() {
		wg.Wait()

		close(doneCh)
	}()

	select {
	case <-doneCh:
	case err := <-errCh:
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestServerShouldServerOverHTTPAndConnectionAndListenerConcurrenlty(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 10
	const request = `{"jsonrpc": "2.0", "method": "foo", "params": [ "bar" ], "id": 1}`

	var wg sync.WaitGroup
	wg.Add(n * 3)

	errCh := make(chan error)

	for range n {
		go func() {
			l := &listener{}

			s := jrpc.NewServer(nil)

			err := s.Accept(ctx, l)
			if err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}

			wg.Done()
		}()

		go func() {
			r := strings.NewReader(request)

			conn := newIOReadWriteCloser(r, io.Discard)

			s := jrpc.NewServer(nil)

			s.ServeConn(ctx, conn)

			wg.Done()
		}()

		go func() {
			r := strings.NewReader(request)

			req := &http.Request{
				Body: io.NopCloser(r),
			}

			w := httptest.NewRecorder()

			s := jrpc.NewServer(nil)

			s.ServeHTTP(w, req)

			wg.Done()
		}()
	}

	cancel() // Cancel the context to stop the servers.

	// Wait for all goroutines to finish.
	doneCh := make(chan struct{})

	go func() {
		wg.Wait()

		close(doneCh)
	}()

	select {
	case <-doneCh:
	case err := <-errCh:
		t.Fatalf("expected no error, got %v", err)
	}
}
