package jrpc

import "encoding/json"

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string           `json:"jsonrpc"` // Must be "2.0".
	Method  string           `json:"method"`  // The method to be invoked.
	Params  *json.RawMessage `json:"params"`  // The parameters to use, may be nil, must be an array or object.

	// The request identifier, must match the response identifier. If nil, the server must not send a response.
	ID *json.Number `json:"id"`
}
