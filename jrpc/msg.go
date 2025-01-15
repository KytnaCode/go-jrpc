package jrpc

import "encoding/json"

const jsonrpc = "2.0"

type Message struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	Result  any             `json:"result,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func errorMessage(code int, msg string, data any) Message {
	return Message{
		JSONRPC: jsonrpc,
		ID:      nil,
		Result:  nil,
		Method:  "",
		Params:  nil,
		Error: &Error{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	}
}
