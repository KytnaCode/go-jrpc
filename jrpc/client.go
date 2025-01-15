package jrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
)

const (
	ParseError       = -32700
	InternalError    = -32603
	UnsupportedError = -32001 // Feature not implemented yet.
)

// ClientDispatcher handles client connections parsing their requests and dispatching messages
// and batch requests to their correct callback.
type ClientDispatcher interface {
	// Dispatch handles a client connection, parsing its content and dispatching messages
	// and batch requests to their correct callbacks, the connection is closed on finish.
	// Can be called concurrently.
	Dispatch(ctx context.Context, conn net.Conn)
}

// NewClientDispatcher creates a new instance of jrpc.DefaultDispatcher.
func NewClientDispatcher(logger *slog.Logger, handler CallbackHandler) *DefaultDispatcher {
	return &DefaultDispatcher{logger: logger, handler: handler}
}

// DefaultDispatcher is the default implementation of ClientDispatcher.
type DefaultDispatcher struct {
	logger  *slog.Logger
	handler CallbackHandler // Handles messages
}

// Dispatch implements jrpc.ClientDispatcher.Dispatch.
func (cd *DefaultDispatcher) Dispatch(ctx context.Context, conn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.Close()

	// Get the raw message because we don't know if it's a batch request or a single message.
	var raw json.RawMessage

	parseError := errorMessage(ParseError, "could not parse request", nil)

	err := json.NewDecoder(conn).Decode(&raw)
	if cd.handleError(err, conn, parseError) {
		return
	}

	msgs, batch, err := parseMessages(raw)
	if cd.handleError(err, conn, parseError) {
		return
	}

	internalError := errorMessage(InternalError, "could not write response", nil)

	if batch {
		resp := cd.handler.HandleBatch(ctx, msgs)

		err := json.NewEncoder(conn).Encode(resp)
		// Is not necessary to check return value here because there's no extra error handling code.
		cd.handleError(err, conn, internalError)

		return
	}

	resp := cd.handler.HandleMsg(ctx, msgs[0])

	err = json.NewEncoder(conn).Encode(resp)
	cd.handleError(err, conn, internalError) // We don't need extra error handling here either.
}

// handleError logs the error and send an error response.
func (cd *DefaultDispatcher) handleError(err error, conn net.Conn, errMsg Message) bool {
	if err != nil {
		err = json.NewEncoder(conn).Encode(errMsg) // Send error.
		if err != nil {
			cd.logger.Error("error handling request", slog.Any("error", err))
		}

		return true
	}

	return false
}

// parseMessages parses a raw json.RawMessage into a list of messages, and a boolean
// that is true when the request is a batch of messages.
// if raw is a single message object parseMessages will return a list with only one message,
// if the raw is an array of only one message it is still a batch request.
func parseMessages(raw json.RawMessage) ([]Message, bool, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))

	var msgs []Message

	// readBracket check if raw is a batch, if it is then read the array bracket.
	readBracket := func() ([]Message, error) {
		if isBatch(raw) {
			if _, err := dec.Token(); errors.Is(err, io.EOF) {
				return []Message{}, nil // If is an empty list.
			} else if err != nil {
				return nil, fmt.Errorf("could not read batch: %w", err)
			}
		}

		return nil, nil
	}

	if emptyMsgs, err := readBracket(); err != nil {
		return emptyMsgs, true, err
	}

	// Decode messages.
	for dec.More() {
		var msg Message

		if err := dec.Decode(&msg); err != nil {
			return nil, false, fmt.Errorf("could not decode messages: %w", err)
		}

		msgs = append(msgs, msg)
	}

	if emptyMsgs, err := readBracket(); err != nil {
		return emptyMsgs, true, err
	}

	return msgs, isBatch(raw), nil
}

// isBatch check if raw is a json array.
func isBatch(raw json.RawMessage) bool {
	for _, char := range raw {
		// Insignificant whitespace
		// Space || Horizontal tab || New line || Carriage return
		if char == 0x20 || char == 0x09 || char == 0x0a || char == 0x0d {
			continue
		}

		return char == '['
	}

	return false
}
