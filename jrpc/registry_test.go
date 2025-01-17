package jrpc_test

import (
	"errors"
	"testing"

	"github.com/kytnacode/go-jrpc/jrpc"
)

func TestDefaultRegistry_RegisterShouldReturnANilErrorWithNoOutNoInHandler(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := func() {}

	if err := registry.Register("handler", handler); err != nil {
		t.Errorf("no input and no output handler should work: %v", err)
	}
}

func TestDefaultRegistry_RegisterShouldReturnANilErrorWithStructIn(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := func(_ struct{ MyField int }) {}

	if err := registry.Register("handler", handler); err != nil {
		t.Errorf("struct input should work: %v", err)
	}
}

func TestDefaultRegistry_RegisterShouldReturnANilErrorWithValueOut(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := func() int { return 2 }

	if err := registry.Register("handler", handler); err != nil {
		t.Errorf("one value out should work: %v", err)
	}
}

func TestDefaultRegistry_RegisterShouldReturnANilErrorWithErrorOut(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	//nolint:err113
	handler := func() error { return errors.New("my error") }

	if err := registry.Register("handler", handler); err != nil {
		t.Errorf("error only out should work: %v", err)
	}
}

func TestDefaultRegistry_RegisterSHouldReturnANilErrorWithValueAndErrorOut(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := func() (int, error) { return 2, nil }

	if err := registry.Register("handler", handler); err != nil {
		t.Errorf("value and error out should work: %v", err)
	}
}

func TestDefaultRegistry_RegisterNonStructInputShouldReturnAnError(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := func(_ int) {}

	if err := registry.Register("handler", handler); !errors.Is(err, jrpc.ErrArgumentsType) {
		t.Errorf("non struct in should return an invalid signature error: got %v", err)
	}
}

func TestDefaultRegistry_RegisterTwoValueOutShouldReturnAnError(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := func() (string, string) { return "two", "values" }

	if err := registry.Register("handler", handler); !errors.Is(err, jrpc.ErrInvalidSignature) {
		t.Errorf("Two values out should return an invalid signature error: got %v", err)
	}
}

func TestDefaultRegistry_RegisterShouldReturnAnErrorWithANonFunctionHandler(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	handler := 2

	if err := registry.Register("handler", handler); !errors.Is(err, jrpc.ErrHandlerType) {
		t.Errorf("a non function handler should return ErrHandlerType: got %v", err)
	}
}

func TestDefaultRegistry_ShoulReturnFoundIfHandlerWasRegistered(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	const handlerName = "handler"

	handler := func() {}

	if err := registry.Register(handlerName, handler); err != nil {
		t.Errorf("error on handler register: %v", err)
	}

	if _, _, found := registry.GetByName(handlerName); !found {
		t.Error("registry should have found the method")
	}
}

func TestDefaultRegistry_GetByNameShouldReturnNotFoundIfMethodWasNotRegistered(t *testing.T) {
	t.Parallel()

	registry := jrpc.NewRegistry(newLogger(t))

	if _, _, found := registry.GetByName("handler"); found {
		t.Error("registry should not have found method")
	}
}
