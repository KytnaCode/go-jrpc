package jrpc_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/kytnacode/go-jrpc"
)

// mock io.ReadWriteCloser.
type ioReadWriteCloser struct {
	io.Reader
	io.Writer

	closed bool
}

func (rwc *ioReadWriteCloser) Close() error {
	rwc.closed = true

	return nil
}

func newIOReadWriteCloser(r io.Reader, w io.Writer) *ioReadWriteCloser {
	return &ioReadWriteCloser{r, w, false}
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
