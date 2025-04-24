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
	"sync/atomic"
	"testing"
	"time"

	"github.com/kytnacode/go-jrpc"
	"github.com/kytnacode/go-jrpc/internal/test"
)

type rwc struct {
	io.Reader
	io.Writer

	closed atomic.Bool
}

type signalWriter struct {
	done chan struct{}
	once sync.Once
	n    int
	mu   sync.Mutex
}

func (w *signalWriter) Write(_ []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.done != nil && w.n <= 0 {
		w.once.Do(func() {
			close(w.done)
		})
	}

	w.n--

	return 0, nil
}

type counter struct {
	n uint64
}

func (c *counter) Next() uint64 {
	c.n++

	return c.n - 1
}

func (rwc *rwc) Write(p []byte) (n int, err error) {
	if rwc.closed.Load() {
		return 0, io.EOF
	}

	return rwc.Writer.Write(p) //nolint:wrapcheck
}

func (rwc *rwc) Read(p []byte) (n int, err error) {
	if rwc.closed.Load() {
		return 0, io.EOF
	}

	return rwc.Reader.Read(p) //nolint:wrapcheck
}

func (rwc *rwc) Close() error {
	rwc.closed.Store(true)

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
		if call.Error == nil {
			t.Fatal("Expected call to return an error")
		}

		if !errors.Is(call.Error, jrpc.ErrInternalError) {
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
		Reader: &test.NoOpReader{},
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

func TestClient_GoShouldReturnAnErrorOnArleadyClosedClient(t *testing.T) {
	t.Parallel()

	conn := &rwc{
		Reader: &test.NoOpReader{},
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	if err := c.Close(); err != nil {
		t.Fatalf("Failed to close client: %v", err)
	}

	call := c.Go(nil, jrpc.Call("foo").Args("bar"))

	select {
	case <-call.Done:
		if call.Error == nil {
			t.Fatal("Expected call to return an error")
		}

		if !errors.Is(call.Error, jrpc.ErrClientShutdown) {
			t.Fatalf("Expected client shutdown error, got %v", call.Error)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	}
}

func TestClient_GoShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const n = 10

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	responses := make([]jrpc.Response, n)
	for i := range n {
		responses[i] = jrpc.Response{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(uint64(i)), //nolint:gosec
			Result:  float64(i),
		}
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{n: n - 1, done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(n)

	for range n {
		go func() {
			<-c.Go(nil, jrpc.Call("foo").Args("bar")).Done

			wg.Done()
		}()
	}

	<-written

	go func() {
		for i := range n {
			b, err := json.Marshal(responses[i])
			if err != nil {
				errCh <- err

				continue
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

func TestClient_CallShouldReturnResult(t *testing.T) {
	t.Parallel()

	const result = 4.0

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	request := jrpc.Response{
		JSONRPC: jrpc.JSONRPCVersion,
		ID:      getID(0),
		Result:  result,
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	go func() {
		<-written

		b, err := json.Marshal(request)
		if err != nil {
			t.Errorf("Failed to marshal response: %v", err)
		}

		_, err = w.Write(b)
		if err != nil {
			t.Errorf("Failed to write to pipe: %v", err)
		}
	}()

	call := c.Call(jrpc.Call("foo").Args("bar"))

	select {
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	default:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if call.Result[0].Error != nil {
			t.Errorf("Expected no error, got %v", call.Result[0].Error)
		}

		if call.Result[0].Result != result {
			t.Errorf("Expected %v, got %v", result, call.Result[0].Result)
		}
	}
}

func TestClient_CallShouldReturnError(t *testing.T) {
	t.Parallel()

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	response := jrpc.Response{
		JSONRPC: jrpc.JSONRPCVersion,
		ID:      getID(0),
		Error:   &jrpc.Error{Code: jrpc.InternalError, Message: "internal error"},
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	go func() {
		<-written

		b, err := json.Marshal(response)
		if err != nil {
			t.Errorf("Failed to marshal response: %v", err)
		}

		_, err = w.Write(b)
		if err != nil {
			t.Errorf("Failed to write to pipe: %v", err)
		}
	}()

	call := c.Call(jrpc.Call("foo").Args("bar"))

	select {
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	default:
		if call.Error == nil {
			t.Fatal("Expected call to return an error")
		}

		if !errors.Is(call.Error, jrpc.ErrInternalError) {
			t.Fatalf("Expected internal error, got %v", call.Result[0].Error)
		}
	}
}

func TestClient_CallShouldNotReturnAnResponseToANotification(t *testing.T) {
	t.Parallel()

	conn := &rwc{
		Reader: &test.NoOpReader{},
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan *jrpc.CallState)

	go func() {
		done <- c.Call(jrpc.Call("foo").Args("bar").Notify())
	}()

	select {
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	case call := <-done:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if call.Result != nil {
			t.Errorf("Expected no result, got %v", call.Result)
		}
	}
}

func TestClient_CallShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const n = 10

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	responses := make([][]byte, n)

	for i := range n {
		res := jrpc.Response{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(uint64(i)), //nolint:gosec
			Result:  float64(i),
		}

		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("Failed to marshal response: %v", err)
		}

		responses[i] = b
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{n: n - 1, done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(n)

	for range n {
		go func() {
			c.Call(jrpc.Call("foo").Args("bar"))

			wg.Done()
		}()
	}

	<-written

	go func() {
		for i := range n {
			_, err := w.Write(responses[i])
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

func TestClient_CallBatchShouldReturnResult(t *testing.T) {
	t.Parallel()

	const result = 4.0

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	responses := []jrpc.Response{
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(0),
			Result:  result,
		},
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(1),
			Result:  result,
		},
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	go func() {
		<-written

		b, err := json.Marshal(responses)
		if err != nil {
			errCh <- err
		}

		_, err = w.Write(b)
		if err != nil {
			errCh <- err
		}
	}()

	call := c.CallBatch(
		jrpc.Call("foo").Args("bar"),
		jrpc.Call("baz").Args("qux"),
	)

	select {
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	default:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if len(call.Result) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(call.Result))
		}

		for _, res := range call.Result {
			if res.Error != nil {
				t.Errorf("Expected no error, got %v", res.Error)
			}

			if res.Result != result {
				t.Errorf("Expected %v, got %v", result, res.Result)
			}
		}
	}
}

func TestClient_CallBatchShouldReturnAnBatchError(t *testing.T) {
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
			Error:   &jrpc.Error{Code: jrpc.MethodNotFound, Message: "method not found"},
		},
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	go func() {
		<-written

		b, err := json.Marshal(responses)
		if err != nil {
			errCh <- err
		}

		_, err = w.Write(b)
		if err != nil {
			errCh <- err
		}
	}()

	call := c.CallBatch(
		jrpc.Call("foo").Args("bar"),
		jrpc.Call("baz").Args("qux"),
	)

	select {
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	default:
		if call.Error == nil {
			t.Fatal("Expected call to return an error")
		}

		if !errors.Is(call.Error, jrpc.ErrBatch) {
			t.Fatalf("Expected batch error, got %v", call.Error)
		}

		if len(call.Result) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(call.Result))
		}

		for _, res := range call.Result {
			if res.Error == nil {
				t.Fatalf("Expected call to return an error, got %v", res)
			}
		}
	}
}

func TestClient_CallBatchShouldNotReturnAnResponseToANotification(t *testing.T) {
	t.Parallel()

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	response := []jrpc.Response{
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(0),
			Result:  4.0,
		},
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan *jrpc.CallState)

	go func() {
		done <- c.CallBatch(
			jrpc.Call("foo").Args("bar").Notify(), // notification
			jrpc.Call("baz").Args("qux"),          // non-notification
		)
	}()

	go func(t *testing.T) {
		t.Helper()

		<-written

		fmt.Println("Writing response")

		b, err := json.Marshal(response)
		if err != nil {
			t.Errorf("Failed to marshal response: %v", err)
		}

		if _, err := w.Write(b); err != nil {
			t.Errorf("Failed to write to pipe: %v", err)
		}
	}(t)

	select {
	case err := <-errCh:
		t.Errorf("Failed handling request: %v", err)
	case call := <-done:
		if call.Error != nil {
			t.Errorf("Expected no error, got %v", call.Error)
		}

		if len(call.Result) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(call.Result))
		}
	}
}

func TestClient_CallBatchShouldBeSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	const n = 10

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, n))

		return &id
	}

	responses := make([][]jrpc.Response, n)
	for i := range n {
		responses[i] = make([]jrpc.Response, n)
		for j := range n {
			responses[i][j] = jrpc.Response{
				JSONRPC: jrpc.JSONRPCVersion,
				ID:      getID(uint64(i*n + j)), //nolint:gosec
				Result:  float64(i*n + j),
			}
		}
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{n: n - 1, done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	counter := &counter{}

	calls := make([][]*jrpc.CallData, n)
	for i := range n {
		calls[i] = make([]*jrpc.CallData, n)
		for j := range n {
			calls[i][j] = jrpc.Call("foo").
				GenID(counter).
				Args(struct{ A int }{A: i*n + j}).
				GetID(new(uint64))
		}
	}

	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(n)

	for i := range n {
		go func() {
			defer wg.Done()
			c.GoBatch(nil, calls[i]...)
		}()
	}

	go func() {
		<-written

		for i := range n {
			b, err := json.Marshal(responses[i])
			if err != nil {
				errCh <- err

				continue
			}

			if _, err = w.Write(b); err != nil {
				errCh <- fmt.Errorf("Failed to write to pipe: %w", err)
			}
		}
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case err := <-errCh:
		t.Fatalf("Failed handling request: %v", err)
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

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
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

	b, err := json.Marshal(responses)
	if err != nil {
		t.Fatalf("Failed to marshal responses: %v", err)
	}

	if _, err := w.Write(b); err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
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

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	call := c.GoBatch(nil,
		jrpc.Call("foo").Args("bar"),
		jrpc.Call("baz").Args("qux"),
	)

	if _, err := w.Write(b); err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
	}

	select {
	case <-call.Done:
		if call.Error == nil {
			t.Error("Expected call to return an error")
		}

		if !errors.Is(call.Error, jrpc.ErrBatch) {
			t.Fatalf("Expected batch error, got %v", call.Error)
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

	r, w := io.Pipe()

	conn := &rwc{
		Reader: r,
		Writer: io.Discard,
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	call := c.GoBatch(nil,
		jrpc.Call("foo").Args("bar").Notify(), // notification
		jrpc.Call("baz").Args("qux"),          // non-notification
	)

	if _, err := w.Write([]byte(`[{ "jsonrpc":"2.0", "id":0, "result": 4.0}]` + "\n")); err != nil {
		t.Fatalf("Failed to write to pipe: %v", err)
	}

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

	counter := &counter{}

	calls := make([][]*jrpc.CallData, n)
	for i := range n {
		calls[i] = make([]*jrpc.CallData, n)
		for j := range n {
			// Pregenerate ID's to match with the response. Client will be called from different goroutines, so it not will
			// generate ID's in order, and the response's ids are in order.
			calls[i][j] = jrpc.Call("foo").Args("bar").GenID(counter).GetID(new(uint64))
		}
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{n: n - 1, done: written},
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(n)

	for i := range n {
		go func() {
			<-c.GoBatch(nil, calls[i]...).Done
			wg.Done()
		}()
	}

	<-written

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

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, errCh)

	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context canceled error, got %v", err)
	}

	if !c.Closed() {
		t.Error("Expected conn to be closed, but it wasn't")
	}
}

func TestClient_InputShouldCancelPendingCalls(t *testing.T) {
	t.Parallel()

	written := make(chan struct{})

	conn := &rwc{
		Reader: &test.NoOpReader{},
		Writer: &signalWriter{done: written},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, errCh)

	call := c.Go(nil, jrpc.Call("foo").Args("bar"))

	<-written

	cancel()

	<-call.Done

	if call.Error == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(call.Error, jrpc.ErrClientShutdown) {
		t.Errorf("Expected client shutdown error, got %v", call.Error)
	}
}

func TestClient_InputShouldHandleNullIDOnSingleRequests(t *testing.T) {
	t.Parallel()

	responses := jrpc.Response{
		JSONRPC: jrpc.JSONRPCVersion,
		ID:      nil,
		Error:   &jrpc.Error{Code: jrpc.ParseError, Message: "parse error"},
	}

	b, err := json.Marshal(responses)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	conn := &rwc{
		Reader: strings.NewReader(string(b)),
		Writer: io.Discard,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, errCh)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		if !errors.Is(err, jrpc.ErrNullID) {
			t.Errorf("Expected parse error, got %v", err)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected error, but it timed out")
	}
}

func TestClient_InputShouldHandleNullIDOnAllBatchRequests(t *testing.T) {
	t.Parallel()

	responses := []jrpc.Response{
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      nil,
			Error:   &jrpc.Error{Code: jrpc.ParseError, Message: "parse error"},
		},
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      nil,
			Error:   &jrpc.Error{Code: jrpc.InvalidRequest, Message: "invalid requests"},
		},
	}

	b, err := json.Marshal(responses)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	conn := &rwc{
		Reader: strings.NewReader(string(b)),
		Writer: io.Discard,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, errCh)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		if !errors.Is(err, jrpc.ErrNullID) {
			t.Errorf("Expected parse error, got %v", err)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected error, but it timed out")
	}
}

func TestClient_InputShouldHandleNullIDInSomeBatchRequests(t *testing.T) {
	t.Parallel()

	getID := func(i uint64) *json.Number {
		id := json.Number(strconv.FormatUint(i, 10))

		return &id
	}

	responses := []jrpc.Response{
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      nil,
			Error:   &jrpc.Error{Code: jrpc.ParseError, Message: "parse error"},
		},
		{
			JSONRPC: jrpc.JSONRPCVersion,
			ID:      getID(1),
			Result:  4.0,
		},
	}

	r, w := io.Pipe()

	written := make(chan struct{})

	conn := &rwc{
		Reader: r,
		Writer: &signalWriter{done: written},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(ctx, errCh)

	go func() {
		<-written

		b, err := json.Marshal(responses)
		if err != nil {
			t.Errorf("Failed to marshal response: %v", err)
		}

		if _, err := w.Write(b); err != nil {
			t.Errorf("Failed to write to pipe: %v", err)
		}
	}()

	call := c.GoBatch(nil,
		jrpc.Call("foo").Args("bar"),
		jrpc.Call("baz").Args("qux"),
	)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		if !errors.Is(err, jrpc.ErrNullID) {
			t.Errorf("Expected parse error, got %v", err)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected error, but it timed out")
	}

	select {
	case <-call.Done:
		if call.Error == nil {
			t.Fatal("Expected error, got nil")
		}

		if !errors.Is(call.Error, jrpc.ErrNullID) {
			t.Errorf("Expected batch error, got %v", call.Error)
		}
	case <-time.After(1 * time.Second): // Timeout
		t.Error("Expected call to be done, but it timed out")
	}
}
