package jrpc

import "encoding/json"

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string      `json:"jsonrpc,omitempty"` // Must be "2.0".
	Result  any         `json:"result,omitempty"`  // The result of the call, must not be nil on success.
	Error   *Error      `json:"error,omitempty"`   // The error of the call, must not be nil on failure.
	ID      json.Number `json:"id"`                // The request identifier, must match the response identifier.
}

// Error represents a JSON-RPC error.
type Error struct {
	// A number indicating the error type that occurred.
	// This MUST be an integer.
	// The error codes from and including -32768 to -32000 are reserved for pre-defined errors.
	Code    int    `json:"code"`
	Message string `json:"message"`        // A string providing a short description of the error.
	Data    any    `json:"data,omitempty"` // A Primitive or Structured value that contains additional information about the error.
}
