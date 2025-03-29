package parse

import "reflect"

// paramsWrapper is a wrapper for the params to be unmarshaled.
// Implements the json.Unmarshaler interface.
type paramsWrapper struct {
	// t is the type of the params to be unmarshaled.
	t reflect.Type
	// value is the value of params after unmarshaling, it will be of type t.
	value any
}
