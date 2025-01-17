package jrpc

import "fmt"

const (
	UnsupportedError = -32001 // Feature not implemented yet.
	UnexpectedError  = -32002
	MethodNotFound   = -32601
	InvalidParams    = -32602
	InternalError    = -32603
	ParseError       = -32700
)

type ProcedureError struct {
	code int
	msg  string
	data any
}

func (pe *ProcedureError) Error() string {
	return fmt.Sprintf("%v: %v", pe.code, pe.msg)
}

func NewError(code int, msg string, data any) *ProcedureError {
	return &ProcedureError{
		code: code,
		msg:  msg,
		data: data,
	}
}
