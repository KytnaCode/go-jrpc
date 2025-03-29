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

var (
	ErrInvalidHandlerType = errors.New("Invalid handler type") // Error returned when the handler is invalid.
	ErrMethodNotFound     = errors.New("Method not found")     // Error returned when the method is not found.
)

// Register is a type-safe wrapper around Register.Register.
func RegisterInto[I, O any](r Register, method string, handler func(in I, out *O) error) error {
	return r.Register(method, handler)
}

// Register defines the method to register a handler.
type Register interface {
	Register(method string, handler any) error
}

// Registry registers handlers and calls them. Implements the Register interface.
// Is safe for concurrent use.
type Registry struct {
	handlers sync.Map
}

// NewRegistry creates a new Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Call calls the handler for the method with the params and returns the reply.
// Params must be a struct or a pointer to a struct, for get a valid
// struct from the raw params use the parse.Params or parse.ParamsType functions.
func (r *Registry) Call(method string, params any) (any, error) {
	handler, ok := r.handlers.Load(method)
	if !ok {
		return nil, fmt.Errorf("method %q not found: %w", method, ErrMethodNotFound)
	}

	handlerV := reflect.ValueOf(handler)

	paramsV := reflect.ValueOf(params)

	replyV := reflect.New(handlerV.Type().In(handlerReplyIndex).Elem()) // Pass reply as reference.

	out := handlerV.Call([]reflect.Value{paramsV, replyV})

	outErr, _ := out[0].Interface().(error) // Safe to convert.
	if outErr != nil {
		return nil, outErr
	}

	return replyV.Interface(), nil
}

// Register registers a handler for the method. implements the Register interface.
// The handler must be a function that takes two arguments, the first argument must be a struct or a pointer to a struct,
// the second argument must be a pointer to a struct, and the handler must return an error.
// Is safe for concurrent use.
func (r *Registry) Register(method string, handler any) error {
	handlerT := reflect.TypeOf(handler)

	if err := validateHandler(handlerT); err != nil {
		return err
	}

	r.handlers.Store(method, handler)

	return nil
}

// validateHandler checks handler type.
func validateHandler(handlerT reflect.Type) error {
	if handlerT.Kind() != reflect.Func { // Must be a function.
		return fmt.Errorf("handler must be a function: %w", ErrInvalidHandlerType)
	}

	if handlerT.NumIn() != handlerNumIn { // Must take arguments and reply.
		return fmt.Errorf("handler must take two arguments, got %v: %w", handlerNumIn, ErrInvalidHandlerType)
	}

	// Check if the first argument is a struct or a pointer to a struct.
	if handlerT.In(0).Kind() != reflect.Struct && !(handlerT.In(0).Kind() == reflect.Pointer && handlerT.In(0).Elem().Kind() == reflect.Struct) {
		return fmt.Errorf("handler's first argument must be a struct or a pointer to a struct, got %v: %w", handlerT.In(0).Kind(), ErrInvalidHandlerType)
	}

	if handlerT.In(1).Kind() != reflect.Pointer { // Must be a pointer.
		return fmt.Errorf("handler's second argument must be a pointer, got %v: %w", handlerT.In(1).Kind(), ErrInvalidHandlerType)
	}

	if handlerT.NumOut() != handlerNumOut { // Must return an error.
		return fmt.Errorf("handler must return %v values, got %v: %w", handlerNumOut, handlerT.NumOut(), ErrInvalidHandlerType)
	}

	if !handlerT.Out(0).AssignableTo(reflect.TypeFor[error]()) { // Must return an error.
		return fmt.Errorf("handler must return an error, got %v: %w", handlerT.Out(0), ErrInvalidHandlerType)
	}

	return nil
}
