package jrpc_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/kytnacode/go-jrpc/jrpc"
)

// MockDispatcher implements ClientDispatcher.
type MockDispatcher struct{}

func (md *MockDispatcher) Dispatch(_ context.Context, _ net.Conn) {}

// MockListener implements a net.Listener that always return an error on Accept method.
type MockListener struct{}

func (ml *MockListener) Accept() (net.Conn, error) {
	return nil, errors.New("my error") //nolint:err113
}

func (ml *MockListener) Close() error {
	return nil
}

func (ml *MockListener) Addr() net.Addr {
	return nil
}

func newLogger(t *testing.T) *slog.Logger {
	t.Helper()

	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func TestServer_ServerListenerShouldReturnAnErrorWhenContextIsDone(t *testing.T) {
	t.Parallel()

	l := newLogger(t)

	server := jrpc.NewServer(l, &MockDispatcher{})

	ctx, cancel := context.WithCancel(context.Background())

	listener, err := net.Listen("tcp", "127.0.0.1:0") // use any available port.
	if err != nil {
		t.Errorf("could not create listener: %v", err)
	}

	cancel()

	err = server.ServeListener(ctx, listener)
	if err == nil {
		t.Error("should return an error once context is done")
	}
}

func TestServer_ServerListenerShouldReturnAnErrorIfListenerFails(t *testing.T) {
	t.Parallel()

	l := newLogger(t)

	server := jrpc.NewServer(l, &MockDispatcher{})

	err := server.ServeListener(context.Background(), &MockListener{})
	if err == nil {
		t.Error("server should return an error if listener fails")
	}
}
