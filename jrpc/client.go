package jrpc

import (
	"context"
	"net"
)

// ClientDispatcher dispatches client connections.
type ClientDispatcher interface {
	// Dispatch handles a connection in a new goroutine.
	Dispatch(ctx context.Context, conn net.Conn)
}
