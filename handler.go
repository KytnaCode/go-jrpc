package jrpc

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

const (
	handlerNumIn       = 2 // Must take arguments and reply.
	handlerNumOut      = 1 // Must return an error.
	handlerParamsIndex = 0
	handlerReplyIndex  = 1
)

// Errors returned by the Registry.
var (
	// ErrInvalidHandlerType is returned when the handler does not satisfy the requirements.
	ErrInvalidHandlerType = errors.New(
		"invalid handler type",
	)

	// Error returned when the method is not found.
	ErrMethodNotFound = errors.New(
		"method not found",
	)
)

// RegisterInto is a type-safe wrapper around Register.Register:
//
//	 err := RegisterInto[ArgsType, ReplyType](r, "method", func(args ArgsType, reply *ReplyType) error { ... })
//	// Is equivalent to:
//	 err := r.Register("method", func(args ArgsType, reply *ReplyType) error { ... })
func RegisterInto[I, O any](r Register, method string, handler func(in I, out *O) error) error {
	if r.Register(method, handler) != nil {
		return fmt.Errorf("failed to register method %q: %w", method, ErrInvalidHandlerType)
	}

	return nil
}

// Register defines the method to register a handler.
type Register interface {
	Register(method string, handler any) error
}

// Caller defines the method to call a handler.
type Caller interface {
	Call(method string, params any) (any, error)
}

// MethodRegister defines the method to register a handler and call it.
type MethodRegister interface {
	Register
	Caller

	// MethodParamsType returns the type of the method's arguments type.
	MethodParamsType(method string) (reflect.Type, error)
}

// Registry registers handlers and calls them. Implements the Register interface, handlers must satisfy the
// following requirements:
//   - Must take two arguments.
//   - The first argument must be the method's arguments type.
//   - The second argument must be a pointer to the method's reply type.
//   - Both arguments and the reply type must be a struct or a pointer to a struct.
//   - The handler must return an error.
//
// The handler must look like this:
//
//	func(args ArgsType, reply *ReplyType) error
//
// Is safe for concurrent use.
type Registry struct {
	handlers sync.Map
}

// NewRegistry creates a new [Registry].
func NewRegistry() *Registry {
	return &Registry{}
}

// Call calls the handler for the method with the params and returns the reply. Params must be a struct or a pointer
// to a struct, for get a valid struct from the raw params use the [parse.Params] or [parse.ParamsType] functions:
//
//	result, err := Call("method", &Params{...})
//	if err != nil {
//	    // handle error
//	}
//
//	fmt.Printf("%T: %v\n", result, result)
func (r *Registry) Call(method string, params any) (any, error) {
	handler, ok := r.handlers.Load(method)
	if !ok {
		return nil, fmt.Errorf("method %q not found: %w", method, ErrMethodNotFound)
	}

	handlerV := reflect.ValueOf(handler)

	var paramsV reflect.Value
	if params != nil {
		paramsV = reflect.ValueOf(params)
	} else {
		paramsV = reflect.Zero(handlerV.Type().In(handlerParamsIndex))
	}

	replyV := reflect.New(handlerV.Type().In(handlerReplyIndex).Elem()) // Pass reply as reference.

	out := handlerV.Call([]reflect.Value{paramsV, replyV})

	outErr, _ := out[0].Interface().(error) // Safe to convert.
	if outErr != nil {
		return nil, outErr
	}

	return replyV.Interface(), nil
}

// Register registers a handler for the method. implements the Register interface. The handler must be a function that
// takes two arguments, the first argument must be a struct or a pointer to a struct, the second argument must be a
// pointer to a struct, and the handler must return an error.
//
// Is safe for concurrent use.
func (r *Registry) Register(method string, handler any) error {
	handlerT := reflect.TypeOf(handler)

	if err := validateHandler(handlerT); err != nil {
		return err
	}

	r.handlers.Store(method, handler)

	return nil
}

// MethodParamsType returns the type of the method's arguments type. If the method is not found, it returns an
// [ErrMethodNotFound] error.
func (r *Registry) MethodParamsType(method string) (reflect.Type, error) {
	handler, ok := r.handlers.Load(method)
	if !ok {
		return nil, fmt.Errorf("method %q not found: %w", method, ErrMethodNotFound)
	}

	return reflect.TypeOf(handler).In(handlerParamsIndex), nil
}

// validateHandler checks handler type.
func validateHandler(handlerT reflect.Type) error {
	if handlerT.Kind() != reflect.Func { // Must be a function.
		return fmt.Errorf("handler must be a function: %w", ErrInvalidHandlerType)
	}

	if handlerT.NumIn() != handlerNumIn { // Must take arguments and reply.
		return fmt.Errorf(
			"handler must take two arguments, got %v: %w",
			handlerNumIn,
			ErrInvalidHandlerType,
		)
	}

	// Check if the first argument is a struct or a pointer to a struct.
	if in := handlerT.In(0); in.Kind() != reflect.Struct && (in.Kind() != reflect.Pointer ||
		in.Elem().Kind() != reflect.Struct) {
		return fmt.Errorf(
			"handler's first argument must be a struct or a pointer to a struct, got %v: %w",
			handlerT.In(0).Kind(),
			ErrInvalidHandlerType,
		)
	}

	if handlerT.In(1).Kind() != reflect.Pointer { // Must be a pointer.
		return fmt.Errorf(
			"handler's second argument must be a pointer, got %v: %w",
			handlerT.In(1).Kind(),
			ErrInvalidHandlerType,
		)
	}

	if handlerT.NumOut() != handlerNumOut { // Must return an error.
		return fmt.Errorf(
			"handler must return %v values, got %v: %w",
			handlerNumOut,
			handlerT.NumOut(),
			ErrInvalidHandlerType,
		)
	}

	if !handlerT.Out(0).AssignableTo(reflect.TypeFor[error]()) { // Must return an error.
		return fmt.Errorf(
			"handler must return an error, got %v: %w",
			handlerT.Out(0),
			ErrInvalidHandlerType,
		)
	}

	return nil
}
