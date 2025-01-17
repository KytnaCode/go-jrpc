package jrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sync"
)

// ParametersParseError indicates an error on parameters' json parsing.
type ParametersParseError struct {
	err error
}

// InvalidParametersError indicates that parameters are a valid json but not a valid type.
type InvalidParametersError struct {
	cause string
}

func (ppe ParametersParseError) Error() string {
	return fmt.Sprintf("could not parse parameters: %v", ppe.err.Error())
}

func (ppe InvalidParametersError) Error() string {
	return fmt.Sprintf("invalid parameters: %v", ppe.cause)
}

func (ppe ParametersParseError) Unwrap() error {
	return ppe.err
}

// CallbackHandler handles procedures call and dispatches them to their method callback.
type CallbackHandler interface {
	// HandleMsg handles a single message. Can return MethodNotFound if the method has not been
	// found in its registry, and ParseError if msg.Params are not a valid JSON, an InvalidParams
	// error if parameters are a valid json but their amount or types does not match, and an
	// InternalError if server stops or an unknown error occurs.
	HandleMsg(ctx context.Context, msg Message) Message
	HandleBatch(ctx context.Context, batch []Message) []Message
}

// DefaultHandler is the default implementation for CallbackHandler.
type DefaultHandler struct {
	logger   *slog.Logger
	registry CallbackRegistry
}

// NewCallbackHandler creates a new DefaultHandler.
func NewCallbackHandler(logger *slog.Logger, registry CallbackRegistry) DefaultHandler {
	return DefaultHandler{logger: logger, registry: registry}
}

func (ch *DefaultHandler) HandleBatch(ctx context.Context, msgs []Message) []Message {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan Message, len(msgs))

	// It's called wg for convention, so ignore warning.
	//nolint:varnamelen
	var wg sync.WaitGroup

	wg.Add(len(msgs))

	go func() {
		wg.Wait()
		close(results)
	}()

	for _, msg := range msgs {
		go func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			defer wg.Done()

			results <- ch.HandleMsg(ctx, msg)
		}()
	}

	out := make([]Message, len(msgs))
	i := 0

	for result := range results {
		out[i] = result
		i++
	}

	return out
}

// HandleMsg handles a single message.
func (ch *DefaultHandler) HandleMsg(ctx context.Context, msg Message) Message {
	callback, argsType, ok := ch.registry.GetByName(msg.Method) // Get the requested method.
	// Check if exists
	if !ok {
		ch.logger.Warn("method could not be found", slog.String("method", msg.Method))

		return errorMessage(MethodNotFound, fmt.Sprintf("%v: not found", msg.Method), nil)
	}

	// Convert msg.Params from json.RawMessage to an array of reflect.Value.
	params, err := parseParams(
		msg.Params,
		argsType,
	)
	if err != nil {
		ch.logger.Error("could not parse params", slog.Any("error", err))

		var parseError ParametersParseError

		var invalidParamsError InvalidParametersError

		if errors.As(err, &parseError) {
			return errorMessage(ParseError, "could not parse parameters", nil)
		} else if errors.As(err, &invalidParamsError) {
			return errorMessage(InvalidParams, "invalid params", invalidParamsError.Error())
		}

		return errorMessage(InternalError, "internal error", nil) // Unknown error.
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan any, 1) // Procedure result
	errCh := make(chan *ProcedureError, 1)

	go func() {
		res, err := callback(params)
		resultCh <- res
		errCh <- err
	}()

	select {
	case <-ctx.Done(): // If context is done, stop procedure.
		return errorMessage(InternalError, "server stopped", nil)
	case res := <-resultCh:
		if err := <-errCh; err != nil {
			return Message{
				JSONRPC: jsonrpc,
				ID:      msg.ID,
				Error: &Error{
					Code:    err.code,
					Message: err.msg,
					Data:    err.data,
				},
				Method: "",
				Params: nil,
				Result: nil,
			}
		}

		resp := Message{
			JSONRPC: jsonrpc,
			ID:      msg.ID,
			Result:  res,
			Method:  "",
			Params:  nil,
			Error:   nil,
		}

		return resp
	}
}

// parseParams parses a json array or object into an array of reflect.Value.
func parseParams(raw json.RawMessage, argsType reflect.Type) ([]reflect.Value, error) {
	if raw == nil { // Allow nil params.
		return []reflect.Value{}, nil
	}

	if isArray(raw) {
		return parsePositionalParams(raw, argsType)
	}

	if isObject(raw) {
		return parseNamedParameters(raw, argsType)
	}

	return nil, InvalidParametersError{cause: "parameters must be an array, an object or nil"}
}

// readBracket reads the next token from dec and returns error if dec.Token return a non EOF error.
func readBracket(dec *json.Decoder) error {
	if _, err := dec.Token(); err != nil && err != io.EOF {
		return ParametersParseError{err: err}
	}

	return nil
}

// decodeParam decodes a single param from an array.
func decodeParam(dec *json.Decoder, argsType reflect.Type, fieldIndex int) (reflect.Value, error) {
	param := reflect.New(argsType.Field(fieldIndex).Type)

	if err := dec.Decode(param.Elem().Addr().Interface()); err != nil {
		var typeError *json.UnmarshalTypeError

		if errors.As(err, &typeError) {
			return reflect.ValueOf(nil), InvalidParametersError{
				cause: fmt.Sprintf("parameter %v: expected %v got %v",
					fieldIndex,
					argsType.Field(fieldIndex).Type.Name(),
					typeError.Value),
			}
		}

		return reflect.ValueOf(nil), ParametersParseError{err: err}
	}

	return param, nil
}

// parsePositionalParams parses parameters as a json array into an array of reflect.Value.
func parsePositionalParams(raw json.RawMessage, argsType reflect.Type) ([]reflect.Value, error) {
	dec := json.NewDecoder(bytes.NewBuffer(raw))

	if err := readBracket(dec); err != nil { // Read '['
		return nil, err
	}

	args := reflect.New(argsType) // Create the arguments struct.

	for fieldIndex := range argsType.NumField() {
		if !argsType.Field(fieldIndex).IsExported() {
			continue
		}

		if !dec.More() { // If there is not more arguments.
			return nil, InvalidParametersError{cause: "not enough parameters to call procedure"}
		}

		param, err := decodeParam(dec, argsType, fieldIndex) // Decode next param.
		if err != nil {
			return nil, err
		}

		// Set value in the arguments struct.
		args.Elem().Field(fieldIndex).Set(param.Elem())
	}

	// Check if there's too many parameters.
	if dec.More() {
		return nil, InvalidParametersError{cause: "too many parameters"}
	}

	if err := readBracket(dec); err != nil { // Reads ']'
		return nil, err
	}

	return []reflect.Value{args.Elem()}, nil
}

// parseNamedParameters parses params as an object.
func parseNamedParameters(raw json.RawMessage, argsType reflect.Type) ([]reflect.Value, error) {
	var params map[string]json.RawMessage

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields() // Return an error if there's unexpected parameters.

	if err := dec.Decode(&params); err != nil {
		return nil, ParametersParseError{err: err}
	}

	// Create args struct.
	args := reflect.New(argsType)

	exportedFields := 0

	// Check if all parameters are present.
	for fieldIndex := range argsType.NumField() {
		if !argsType.Field(fieldIndex).IsExported() {
			continue
		}

		exportedFields++

		value, ok := params[argsType.Field(fieldIndex).Name]
		if !ok {
			return nil, InvalidParametersError{
				cause: fmt.Sprintf("parameter %v is missing", fieldIndex),
			}
		}

		// Create a new pointer with the correct type.
		param := reflect.New(argsType.Field(fieldIndex).Type.Elem())

		// Unmarshal parameter.
		err := json.Unmarshal(value, param.Elem().Addr().Interface())
		if err != nil {
			return nil, ParametersParseError{err: err}
		}

		args.Elem().Field(fieldIndex).Set(param)
	}

	if len(params) != exportedFields {
		return nil, InvalidParametersError{
			cause: fmt.Sprintf("expected exactly %v parameters: got %v", exportedFields, len(params)),
		}
	}

	return []reflect.Value{args.Elem()}, nil
}
