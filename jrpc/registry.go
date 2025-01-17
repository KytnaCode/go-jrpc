package jrpc

import (
	"reflect"
)

// Callback signature. For methods that don't return one or both a value and an error the
// missing return values always will be nil.
type Callback func(params []reflect.Value) (any, *ProcedureError)

// CallbackRegistry allows getting registered callbacks by name.
type CallbackRegistry interface {
	GetByName(name string) (callback Callback, argsType reflect.Type, found bool)
}
