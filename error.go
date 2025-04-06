package jrpc

import "errors"

const (
	ParseError     = -32700 // Parse error. Invalid JSON was received by the server.
	InvalidRequest = -32600 // Invalid Request. The JSON sent is not a valid Request object.
	MethodNotFound = -32601 // Method not found. The method does not exist / is not available.
	InvalidParams  = -32602 // Invalid params. Invalid method parameter(s).
	InternalError  = -32603 // Internal error. Internal JSON-RPC error.
)

var (
	ErrParse          = errors.New("failed to parse JSON-RPC message") // Parse error
	ErrInvalidRequest = errors.New("invalid JSON-RPC request")         // Invalid request
	ErrEmptyRequest   = errors.New("empty JSON-RPC request")           // Empty request
	ErrInternalError  = errors.New("internal error")                   // Internal error
)
