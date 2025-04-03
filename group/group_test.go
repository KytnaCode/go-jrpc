package group_test

import (
	"errors"
	"testing"

	"github.com/kytnacode/go-jrpc/group"
)

type mockRegister struct {
	handlers map[string]any
	invalid  map[string]error
}

func (r *mockRegister) Register(method string, handler any) error {
	if r.invalid == nil {
		r.invalid = make(map[string]error)
	}

	if err, ok := r.invalid[method]; ok {
		return err
	}

	r.handlers[method] = handler

	return nil
}

func (r *mockRegister) setErr(method string, err error) {
	if r.invalid == nil {
		r.invalid = make(map[string]error)
	}

	r.invalid[method] = err
}

func TestGroup_RegisterShouldNotReturnAnErrorNever(t *testing.T) {
	t.Parallel()

	type data struct {
		handler any
	}

	testData := map[string]data{
		// Invalid handlers, Group.Register should not return an error even if the handler is invalid.
		"without arguments":      {handler: func() error { return nil }},
		"without error":          {handler: func(args, reply *struct{}) {}},
		"without args and error": {handler: func() {}},
		"non-pointer reply":      {handler: func(args, reply struct{}) error { return nil }},
		"non-function":           {handler: struct{}{}},

		// Valid handlers
		"valid": {handler: func(args, reply *struct{}) error { return nil }},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var g group.Group

			err := g.Register("method", data.handler)
			if err != nil {
				t.Errorf("Group.Register() should never return an error, got: %v", err)
			}
		})
	}
}

func TestGroup_AddMethodShouldNotPanicHandler(t *testing.T) {
	t.Parallel()

	type data struct {
		handler any
	}

	testData := map[string]data{
		// Invalid handlers, Group.AddMethod should not validate the handler.
		"without arguments":      {handler: func() error { return nil }},
		"without error":          {handler: func(args, reply *struct{}) {}},
		"without args and error": {handler: func() {}},
		"non-pointer reply":      {handler: func(args, reply struct{}) error { return nil }},
		"non-function":           {handler: struct{}{}},

		// Valid handlers
		"valid": {handler: func(args, reply *struct{}) error { return nil }},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var g group.Group

			// Add method should not panic if the handler is invalid.
			g.AddMethod("method", data.handler)
		})
	}
}

func TestGroup_UseShouldSetDefaultSeparator(t *testing.T) {
	t.Parallel()

	const (
		sep            = "_"
		prefix         = "math"
		method         = "add"
		expectedMethod = prefix + sep + method
	)

	var g group.Group

	g.SetSeparator(sep)

	g.Use(prefix, func(g *group.Group) {
		g.AddMethod(method, func(args, reply *struct{}) error { return nil })
	})

	r := &mockRegister{handlers: make(map[string]any)}

	g.RegisterTo(r)

	if _, ok := r.handlers[expectedMethod]; !ok {
		t.Errorf("Group.Use() should set the default separator, expected method %q, got: %v", expectedMethod, r.handlers)
	}
}

func TestGroup_UseSubgroupsSeparatorShouldOverwriteParents(t *testing.T) {
	t.Parallel()

	const (
		sep            = "_"
		subSep         = "-"
		prefix         = "math"
		method         = "add"
		expectedMethod = prefix + subSep + method
	)

	var g group.Group

	g.SetSeparator(sep)

	g.Use(prefix, func(g *group.Group) {
		g.SetSeparator(subSep)
		g.AddMethod(method, func(args, reply *struct{}) error { return nil })
	})

	r := &mockRegister{handlers: make(map[string]any)}

	g.RegisterTo(r)

	if _, ok := r.handlers[expectedMethod]; !ok {
		t.Errorf("Group.Use() should overwrite the separator, expected method %q, got: %v", expectedMethod, r.handlers)
	}
}

func TestGroup_UseSubgroupShouldRegisterTheirSubgroups(t *testing.T) {
	t.Parallel()

	const expectedMethod = "math.arith.add"

	var g group.Group

	g.Use("math", func(g *group.Group) {
		g.Use("arith", func(g *group.Group) {
			g.AddMethod("add", func(args, reply *struct{}) error { return nil })
		})
	})

	r := &mockRegister{handlers: make(map[string]any)}

	errs := g.RegisterTo(r)
	if len(errs) > 0 {
		t.Errorf("Group.Use() should not return errors, got: %v", errs)
	}

	if _, ok := r.handlers[expectedMethod]; !ok {
		t.Errorf("Group.Use() should register subgroups, expected method %q, got: %v", expectedMethod, r.handlers)
	}
}

func TestGroup_UseShouldIgnoreSeparatorWhenCalledWithAnEmptyPrefix(t *testing.T) {
	t.Parallel()

	const (
		prefix         = ""
		method         = "add"
		expectedMethod = method
	)

	var g group.Group

	g.Use(prefix, func(g *group.Group) {
		g.AddMethod(method, func(args, reply *struct{}) error { return nil })
	})

	r := &mockRegister{handlers: make(map[string]any)}

	g.RegisterTo(r)

	if _, ok := r.handlers[expectedMethod]; !ok {
		t.Errorf("Group.Use() should ignore the separator, expected method %q, got: %v", expectedMethod, r.handlers)
	}
}

func TestGroup_RegisterToShouldReturnErrors(t *testing.T) {
	t.Parallel()

	const (
		expectedErrors = 1
		validMethod    = "method1"
		invalidMethod  = "method2"
	)

	var g group.Group

	invalidHandler := func(args, reply *struct{}) {} // No error return.
	validHandler := func(args, reply *struct{}) error { return nil }

	g.AddMethod(validMethod, validHandler)
	g.AddMethod(invalidMethod, invalidHandler)

	r := &mockRegister{handlers: make(map[string]any)}
	r.setErr(invalidMethod, errors.New("invalid handler"))

	errs := g.RegisterTo(r)

	if len(errs) != expectedErrors {
		t.Errorf("Group.RegisterTo() should return %v error(s), got: %v: %v", expectedErrors, len(errs), errs)
	}
}
