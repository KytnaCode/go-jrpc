package jrpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/kytnacode/go-jrpc/jrpc"
)

const (
	jsonrpc      = "2.0"
	validRequest = `{ "jsonrpc": "2.0", "method": "sum", "params": [2, 6], "id": 2}`
)

// New dispatcher creates a new DefaultDispatcher and create a pair of pipes, the first one for
// the dispatcher, and the second one for write requests and recive responses.
func NewDispatcher(t *testing.T) (*jrpc.DefaultDispatcher, net.Conn, net.Conn) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	d := jrpc.NewClientDispatcher(logger, &mockCallbackHandler{})

	server, client := net.Pipe()

	return d, server, client
}

type mockCallbackHandler struct{}

// HandleBatch mocks CallbackHandler.HandleBatch. Just echo messages.
func (ch *mockCallbackHandler) HandleBatch(_ context.Context, msgs []jrpc.Message) []jrpc.Message {
	return msgs
}

// HandleMsg mocks CallbackHandler.HandleMsg. return always the same test message.
func (ch *mockCallbackHandler) HandleMsg(_ context.Context, msg jrpc.Message) jrpc.Message {
	return jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      msg.ID,
		Method:  "",
		Params:  nil,
		Error:   nil,
		Result:  2,
	}
}

func TestDefaultDispatcher_DispatchShouldRespondAnErrorWithAnInvalidRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, "invalid json"); err != nil {
		t.Errorf("could not write request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.Error == nil {
		t.Error("client dispatcher should return an error message")
	}
}

func TestDefaultDispatcher_DispatchShouldRespondANullIdWithAnInvalidRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, "invalid json"); err != nil {
		t.Errorf("could not send a request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.ID != nil {
		t.Errorf("id should be null: got %v", msg.ID)
	}
}

func TestDefaultDispatcher_DispatchShouldNotReturnAResultWithAnInvalidRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, "invalid json"); err != nil {
		t.Errorf("could not write request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.Result != nil {
		t.Errorf("result must not exist when an error occurs: got %v", msg)
	}
}

func TestDefaultDispatcher_DispatchShouldReturnCorrectJsonRpcVersionWithAnInvalidRequest(
	t *testing.T,
) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, "invalid json"); err != nil {
		t.Errorf("could not parse request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.JSONRPC != jsonrpc {
		t.Errorf(`jsonrpc must be "2.0": got %v`, msg.JSONRPC)
	}
}

func TestDefaultDispatcher_DispatchShouldNotReturnAnErrorWithAValidRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, validRequest); err != nil {
		t.Errorf("could not write request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.Error != nil {
		t.Errorf("response should not contain an error: %v", msg)
	}
}

func TestDefaultDispatcher_DispatchShouldReturnAnIdWithAValidRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, validRequest); err != nil {
		t.Errorf("could not parse request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.ID == nil {
		t.Errorf("server should return an id")
	}
}

func TestDefaultDispatcher_DispatchShouldReturnAResultWithAValidRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, validRequest); err != nil {
		t.Errorf("could not write request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.Result == nil {
		t.Errorf("server should return a result")
	}
}

func TestDefaultDispatcher_DispatchShouldReturnCorrectJsonRpcVersionWithAValidRequest(
	t *testing.T,
) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if _, err := fmt.Fprintln(client, validRequest); err != nil {
		t.Errorf("could not write request: %v", err)
	}

	var msg jrpc.Message

	if err := json.NewDecoder(client).Decode(&msg); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if msg.JSONRPC != jsonrpc {
		t.Errorf("server must return jsonrpc version equal to 2.0: got %v", msg.JSONRPC)
	}
}

func TestDefaultDispatcher_DispatchShouldHandleValidBatchRequests(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	batch := []jrpc.Message{
		//nolint:exhaustruct
		{ID: new(int), JSONRPC: jsonrpc, Method: "sum", Params: []byte("[2, 3]")},
		//nolint:exhaustruct
		{ID: new(int), JSONRPC: jsonrpc, Method: "div", Params: []byte("[4, 2]")},
		//nolint:exhaustruct
		{ID: new(int), JSONRPC: jsonrpc, Method: "mul", Params: []byte(`{ "a": 2, "b": 5 }`)},
	}

	if err := json.NewEncoder(client).Encode(batch); err != nil {
		t.Errorf("could not send request: %v", err)
	}

	var msgs []jrpc.Message

	if err := json.NewDecoder(client).Decode(&msgs); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if len(msgs) != len(batch) {
		t.Errorf(
			"server should return an response for every request: got %v len %v",
			msgs,
			len(msgs),
		)
	}
}

func TestDefaultDispatcher_DispatchShouldReturnAnEmptyResponseWihtAnAnEmptyBatchRequest(t *testing.T) {
	t.Parallel()

	d, server, client := NewDispatcher(t)

	go d.Dispatch(context.Background(), server)

	if err := json.NewEncoder(client).Encode([]jrpc.Message{}); err != nil {
		t.Errorf("could not write request: %v", err)
	}

	var msgs []jrpc.Message

	if err := json.NewDecoder(client).Decode(&msgs); err != nil {
		t.Errorf("could not parse response: %v", err)
	}

	if len(msgs) != 0 {
		t.Errorf("response must be empty: got %v", msgs)
	}
}
