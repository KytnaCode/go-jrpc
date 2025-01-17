package jrpc_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/kytnacode/go-jrpc/jrpc"
)

type MockRegistry struct{}

type ArgsType struct {
	MyFirstField  *int
	MySecondField *int
}

const (
	failMethod       = "fail"
	userDefinedError = 1
)

func (mr *MockRegistry) GetByName(method string) (jrpc.Callback, reflect.Type, bool) {
	if method == failMethod {
		return func(params []reflect.Value) (any, *jrpc.ProcedureError) {
			return nil, jrpc.NewError(userDefinedError, "user defined error", nil)
		}, reflect.TypeFor[ArgsType](), true
	}

	return func(params []reflect.Value) (any, *jrpc.ProcedureError) {
		args, ok := params[0].Interface().(ArgsType)
		if !ok {
			return nil, jrpc.NewError(2, "could not get args", nil)
		}

		if args.MyFirstField == nil || args.MySecondField == nil {
			return 0, nil
		}

		return *args.MyFirstField - *args.MySecondField, nil
	}, reflect.TypeFor[ArgsType](), true
}

func NewHandler(t *testing.T) *jrpc.DefaultHandler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	return jrpc.NewCallbackHandler(logger, &MockRegistry{})
}

func TestDefaultHandler_HandleMsgShouldReturnTheCorrectResultWithPositionalParams(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	params := []int{2, 3}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Errorf("could not marshal parameters: %v", err)
	}

	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      new(int),
		Method:  "method",
		Params:  paramsJSON,
		Error:   nil,
		Result:  nil,
	})

	if actual, expected := resp.Result, params[0]-params[1]; actual != expected {
		t.Logf("resp.Error: %v", resp.Error)
		t.Errorf("wrong result: expected %v got %v", expected, actual)
	}
}

func TestDefaulthandler_HandleMsgShouldReturnAnErrorWithInvalidJSONPositionalParams(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte(`[this, is, not a valid, json array ]`),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.ParseError {
		t.Errorf("server should return a parse error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithInvalidJSONNonStructuredParameters(
	t *testing.T,
) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte("invalid json"),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalid parameters error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithValidJSONNonStructuredParameters(
	t *testing.T,
) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte("27"),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalid parameters error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorIfThereIsNoEnoughPositionalParameters(
	t *testing.T,
) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte("[3]"),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalid parameters error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithTooManyPositionalParameters(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte("[3, 6, 2]"),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalid parameters error: got %v", resp.Error)
	}
}

func TestDefaulthandler_HandleMsgShouldReturnAnErrorWithIncorrectTypePositionalParameters(
	t *testing.T,
) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte("[2.4, 9.6]"),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalidad params error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnTheCorrectResultWithNamedParams(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	identifier := 2

	params := struct{ MyFirstField, MySecondField int }{MyFirstField: 4, MySecondField: 2}

	paramsJSON, err := json.Marshal(params) //nolint:musttag
	if err != nil {
		t.Errorf("could not marshal parameters: %v", err)
	}

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &identifier,
		Method:  "method",
		Params:  paramsJSON,
	})

	if actual, expected := resp.Result, params.MyFirstField-params.MySecondField; actual != expected {
		t.Log(resp.Error)
		t.Errorf("wrong result: expected %v got %v", expected, actual)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithInvalidTypeParameters(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	params := struct{ MyFirstField, MySecondField float64 }{MyFirstField: 2.4, MySecondField: 3}

	paramsJSON, err := json.Marshal(params) //nolint:musttag
	if err != nil {
		t.Errorf("could not marshal parameters: %v", err)
	}

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  paramsJSON,
	})

	if resp.Error == nil || resp.Error.Code != jrpc.ParseError {
		t.Errorf("server should return a parsse error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithInvalidJSONNamedParameters(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte(`{ this: "is not", a: "valid json" }`),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.ParseError {
		t.Errorf("server should return a parse error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithTooManyNamedParameters(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	params := struct{ MyFirstField, MySecondField, MyExtraField int }{
		MyFirstField:  4,
		MySecondField: 2,
		MyExtraField:  3,
	}

	paramsJSON, err := json.Marshal(params) //nolint:musttag
	if err != nil {
		t.Errorf("could not marshal parameters: %v", err)
	}

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  paramsJSON,
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalid parameters error: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorWithTooFewNamedParameters(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	params := struct{ MyFirstField *int }{MyFirstField: new(int)}

	paramsJSON, err := json.Marshal(params) //nolint:musttag
	if err != nil {
		t.Errorf("could not marshal parameters: %v", err)
	}

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  paramsJSON,
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InvalidParams {
		t.Errorf("server should return an invalid params error: got %v", err)
	}
}

func TestDefaultHandler_HandleMsgShouldNotReturnAnErrorWhenSendingANullValue(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	params := struct{ MyFirstField, MySecondField *int }{MyFirstField: new(int), MySecondField: nil}

	paramsJSON, err := json.Marshal(params) //nolint:musttag
	if err != nil {
		t.Errorf("could not marshal parameters: %v", err)
	}

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  paramsJSON,
	})

	if resp.Error != nil {
		t.Errorf("null values should be valid")
	}
}

func TestDefaultHandler_HandleMsgShouldReturnAnErrorIfServerStops(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	id := 2

	//nolint:exhaustruct
	resp := handler.HandleMsg(ctx, jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  "method",
		Params:  []byte("[1, 2]"),
	})

	if resp.Error == nil || resp.Error.Code != jrpc.InternalError {
		t.Errorf("server should return an internal error on stop: got %v", resp.Error)
	}
}

func TestDefaultHandler_HandleMsgShouldReturnUserDefinedErrors(t *testing.T) {
	t.Parallel()

	handler := NewHandler(t)

	id := 2

	resp := handler.HandleMsg(context.Background(), jrpc.Message{
		JSONRPC: jsonrpc,
		ID:      &id,
		Method:  failMethod,
		Params:  []byte("[2, 3]"),
	})

	if resp.Error == nil || resp.Error.Code != userDefinedError {
		t.Errorf("handler should return user defined error: got %v", resp.Error)
	}
}
