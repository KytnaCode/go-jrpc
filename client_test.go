package jrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kytnacode/go-jrpc"
)

type rwc struct {
	io.Reader
	io.Writer

	closed bool
}

type signalWriter struct {
	done chan struct{}
}

func (w *signalWriter) Write(p []byte) (n int, err error) {
	if w.done != nil {
		close(w.done)
	}

	return 0, nil
}

func (rwc *rwc) Close() error {
	rwc.closed = true

	return nil
}

func TestClient_GoShouldReturnResult(t *testing.T) {
	t.Parallel()

	const result = 4.0

	request := fmt.Sprintf(`{"jsonrpc":"2.0", "id":0, "result": %v}`+"\n", result)

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	call := c.Go(nil, jrpc.Call("foo").Args("bar"))

	go func() {
		_, err := w.Write([]byte(request))
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-call.Done:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if call.Result[0].Error != nil {
			t.Errorf("Expected no error, got %v", call.Result[0].Error)
		}

		if call.Result[0].Result != result {
			t.Errorf("Expected %v, got %v", result, call.Result[0].Result)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Error handling request: %v", err)
	}
}

func TestClient_GoShouldReturnError(t *testing.T) {
	t.Parallel()

	response := `{"jsonrpc":"2.0", "id":0, "error": {"code": -32603, "message": "internal error"}}` + "\n"

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	call := c.Go(nil, jrpc.Call("foo").Args("bar"))

	go func() {
		_, err := w.Write([]byte(response))
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-call.Done:
		if call.Error != nil { // Client error.
			t.Fatal("Expected call to return a server error, but not a client error")
		}

		if call.Result[0].Error == nil { // Server response error.
			t.Fatalf("Expected call return an error,  got %v", call.Result[0])
		}

		if !errors.Is(call.Result[0].Error, jrpc.ErrInternalError) {
			t.Fatalf("Expected internal error, got %v", call.Result[0].Error)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_GoShouldNotReturnAnResponseToANotification(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})

	conn := &rwc{
		Reader: strings.NewReader(""),
		Writer: &signalWriter{done: done},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	c.Go(nil, jrpc.Call("foo").Args("bar").Notify())

	select {
	case <-done:
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_GoShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const result = 4.0

	const n = 10

	format := `{"jsonrpc":"2.0", "id":%v, "result": %v}` + "\n"

	responses := make([]string, n)
	for i := range n {
		responses[i] = fmt.Sprintf(format, i, result)
	}

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(n)

	for range n {
		call := c.Go(nil, jrpc.Call("foo").Args("bar"))

		go func() {
			<-call.Done
			wg.Done()
		}()
	}

	go func() {
		for i := range n {
			_, err := w.Write([]byte(responses[i]))
			if err != nil {
				errCh <- err
			}
		}
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_GoBatchShouldReturnResult(t *testing.T) {
	t.Parallel()

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	results := map[uint64]float64{
		0: 4.0,
		1: 5.0,
		2: 6.0,
	}

	responses := []jrpc.Response{
		{JSONRPC: jrpc.JSONRPCVersion, ID: getID(0), Result: results[0]},
		{JSONRPC: jrpc.JSONRPCVersion, ID: getID(1), Result: results[1]},
		{JSONRPC: jrpc.JSONRPCVersion, ID: getID(2), Result: results[2]},
	}

	b, err := json.Marshal(responses)
	if err != nil {
		t.Fatalf("Failed to marshal responses: %v", err)
	}

	conn := &rwc{
		Reader: strings.NewReader(string(b)),
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	var id1, id2, id3 uint64

	call := c.GoBatch(nil,
		c.MakeCall("foo").Args("bar").GetID(&id1),
		c.MakeCall("baz").Args("qux").GetID(&id2),
		c.MakeCall("quux").Args("corge").GetID(&id3),
	)

	ids := []uint64{
		id1,
		id2,
		id3,
	}

	select {
	case <-call.Done:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if len(call.Result) != 3 {
			t.Fatalf("Expected 3 results, got %d", len(call.Result))
		}

		for _, id := range ids {
			if call.Result[id].Error != nil {
				t.Errorf("Expected no error, got %v", call.Result[id].Error)
			}

			if call.Result[id].Result != results[id] {
				t.Errorf("Expected %v, got %v", results[id], call.Result[id].Result)
			}
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_GoBatchShouldReturnError(t *testing.T) {
	t.Parallel()

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	responses := []jrpc.Response{
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(0),
			Error:   &jrpc.Error{Code: jrpc.InternalError, Message: "internal error"},
		},
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(1),
			Error:   &jrpc.Error{Code: jrpc.InternalError, Message: "internal error"},
		},
	}

	b, err := json.Marshal(responses)
	if err != nil {
		t.Fatalf("Failed to marshal responses: %v", err)
	}

	conn := &rwc{
		Reader: strings.NewReader(string(b)),
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	call := c.GoBatch(nil,
		jrpc.Call("foo").Args("bar"),
		jrpc.Call("baz").Args("qux"),
	)

	select {
	case <-call.Done:
		if call.Error != nil { // Client error.
			t.Fatal("Expected call to return a server error, but not a client error")
		}

		if len(call.Result) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(call.Result))
		}

		for _, res := range call.Result {
			if res.Error == nil { // Server response error.
				t.Fatalf("Expected call return an error,  got %v", res)
			}
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_GoBatchShouldNotReturnAnResponseToANotification(t *testing.T) {
	t.Parallel()

	conn := &rwc{
		Reader: strings.NewReader(`[{ "jsonrpc":"2.0", "id":0, "result": 4.0}]` + "\n"),
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	call := c.GoBatch(nil,
		jrpc.Call("foo").Args("bar").Notify(), // notification
		jrpc.Call("baz").Args("qux"),          // non-notification
	)

	select {
	case <-call.Done:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if len(call.Result) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(call.Result))
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_GoBatchShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const n = 10

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	responses := make([][]jrpc.Response, n)
	for i := range n {
		responses[i] = make([]jrpc.Response, n)
		for j := range n {
			responses[i][j] = jrpc.Response{
				JSONRPC: jrpc.JSONRPCVersion,
				// i*n + j will never be negative nor overflow, so it's safe to ignore gosec warning.
				ID:     getID(uint64(i*n + j)), //nolint:gosec
				Result: float64(i*n + j),
			}
		}
	}

	calls := make([][]*jrpc.CallData, n)
	for i := range n {
		calls[i] = make([]*jrpc.CallData, n)
		for j := range n {
			calls[i][j] = jrpc.Call("foo").Args("bar")
		}
	}

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(n)

	for i := range n {
		call := c.GoBatch(nil, calls[i]...)

		go func() {
			<-call.Done
			wg.Done()
		}()
	}

	go func() {
		for i := range n {
			b, err := json.Marshal(responses[i])
			if err != nil {
				errCh <- err
			}

			_, err = w.Write(b)
			if err != nil {
				errCh <- err
			}
		}
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	}
}

func TestClient_InputShouldNotPanicWithANillErrorChannel(t *testing.T) {
	t.Parallel()

	request := `{"jsonrpc":"2.0", "id":0, "error": {"code": -32603, "message": "internal error"}}` + "\n"

	conn := &rwc{
		Reader: strings.NewReader(request),
		Writer: io.Discard,
	}

	c := jrpc.NewClient(conn)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Expected no panic, but got: %v", r)
			}
		}()

		c.Input(context.Background(), nil)
	}()
}

func TestClient_InputShouldCloseConn(t *testing.T) {
	t.Parallel()

	request := `{"jsonrpc":"2.0", "id":0, "error": {"code": -32603, "message": "internal error"}}` + "\n"

	conn := &rwc{
		Reader: strings.NewReader(request),
		Writer: io.Discard,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, nil)

	cancel()

	select {
	case <-c.Closed():
	case <-time.After(time.Second):
		if !conn.closed {
			t.Error("Expected conn to be closed, but it wasn't")
		}
	}
}

func TestClient_InputShouldCancelPendingCalls(t *testing.T) {
	t.Parallel()

	conn := &rwc{
		Reader: strings.NewReader(""),
		Writer: io.Discard,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, nil)

	call := c.Go(nil, jrpc.Call("foo").Args("bar"))

	cancel()

	select {
	case <-call.Done:
		if call.Error == nil {
			t.Fatal("Expected error, got nil")
		}

		if !errors.Is(call.Error, jrpc.ErrClientShutdown) {
			t.Errorf("Expected client shutdown error, got %v", call.Error)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	}
}
