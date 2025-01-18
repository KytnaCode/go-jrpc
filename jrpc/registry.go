package jrpc

import (
	"errors"
	"log/slog"
	"reflect"
)

var (
	ErrHandlerType       = errors.New("handler must be a function")
	ErrTooManyArguments  = errors.New("handler must have only one argument in form of a struct")
	ErrArgumentsType     = errors.New("handler arguments must be a struct (non-pointer)")
	ErrInvalidSignature  = errors.New("invalid method signature")
	ErrOnlyPointerFields = errors.New(
		"all exported fields of handler's argument struct must be pointers",
	)
)

// Callback signature. For methods that don't return one or both a value and an error the
// missing return values always will be nil.
type Callback func(params []reflect.Value) (any, *ProcedureError)

// CallbackRegistry allows getting registered callbacks by name.
type CallbackRegistry interface {
	GetByName(name string) (callback Callback, argsType reflect.Type, found bool)
}

// CallbackRegister register new callbacks.
type CallbackRegister interface {
	/*
		Register registers handler as callback for method. Handler must have either no arguments or
		only one as a struct (non-pointer) and can have zero return values, one return value, either
		a value or an error; or two return values, and value and an error.
	*/
	Register(method string, handler any) error
}

// callbackContainer is a container for storing callbacks.
type callbackContainer struct {
	run      Callback
	argsType reflect.Type
}

// DefaultRegistry is the default implementation of both CallbackRegistry and CallbackRegister.
type DefaultRegistry struct {
	logger    *slog.Logger
	callbacks map[string]callbackContainer
}

// NewRegistry returns a new DefaultRegistry.
func NewRegistry(logger *slog.Logger) *DefaultRegistry {
	return &DefaultRegistry{
		logger:    logger,
		callbacks: make(map[string]callbackContainer),
	}
}

// GetByName gets a method's callback by its name.
func (cr *DefaultRegistry) GetByName(name string) (Callback, reflect.Type, bool) {
	container, ok := cr.callbacks[name]
	if !ok {
		return nil, nil, ok
	}

	return container.run, container.argsType, ok
}

// Register registers a callback for a given method.
func (cr *DefaultRegistry) Register(method string, handler any) error {
	handlerValue := reflect.ValueOf(handler)

	if handlerValue.Kind() != reflect.Func {
		return ErrHandlerType
	}

	var argsTypes reflect.Type

	switch handlerValue.Type().NumIn() {
	case 0:
		argsTypes = reflect.TypeFor[any]() // Callback without arguments.
	case 1:
		if handlerValue.Type().In(0).Kind() != reflect.Struct {
			return ErrArgumentsType
		}

		argsTypes = handlerValue.Type().In(0)

		for i := range argsTypes.NumField() {
			if !argsTypes.Field(i).IsExported() {
				continue
			}

			if argsTypes.Field(i).Type.Kind() != reflect.Pointer {
				return ErrOnlyPointerFields
			}
		}
	default:
		return ErrTooManyArguments
	}

	callback, err := CreateCallback(handlerValue)
	if err != nil {
		return err
	}

	cr.callbacks[method] = callbackContainer{run: callback, argsType: argsTypes}
	cr.logger.Info("registered new method", slog.String("method", method))

	return nil
}

const (
	voidOut        = 0 // Returns no values
	valueOrErrOut  = 1 // Returns a value or an error
	valueAndErrOut = 2 // Return both a value and an error
)

func CreateCallback(method reflect.Value) (Callback, error) {
	switch method.Type().NumOut() {
	case voidOut:
		return func(params []reflect.Value) (any, *ProcedureError) {
			method.Call(params)

			return nil, nil
		}, nil
	case valueOrErrOut:
		if method.Type().Out(0).Implements(reflect.TypeFor[error]()) {
			return func(params []reflect.Value) (any, *ProcedureError) {
				out := method.Call(params)
				err := convertError(out[0])

				return nil, err
			}, nil
		}

		return func(params []reflect.Value) (any, *ProcedureError) {
			out := method.Call(params)

			return out[0].Interface(), nil
		}, nil
	case valueAndErrOut:
		if !method.Type().Out(1).Implements(reflect.TypeFor[error]()) {
			return nil, ErrInvalidSignature
		}

		return func(params []reflect.Value) (any, *ProcedureError) {
			out := method.Call(params)
			err := convertError(out[1])

			return out[0].Interface(), err
		}, nil
	}

	return nil, ErrInvalidSignature
}

// convertError check if a value is an ProcedureError, if not return a new ProcedureError.
func convertError(errValue reflect.Value) *ProcedureError {
	if errValue.IsNil() {
		return nil
	}

	err, ok := errValue.Interface().(error)
	if !ok {
		return NewError(InternalError, "internal error", nil)
	}

	var procedureErr *ProcedureError
	if errors.As(err, &procedureErr) {
		return procedureErr
	}

	return NewError(UnexpectedError, err.Error(), nil)
}
