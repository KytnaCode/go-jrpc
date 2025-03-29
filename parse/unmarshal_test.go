package parse_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kytnacode/go-jrpc/parse"
)

type args struct {
	Name   string `json:"name"`
	Last   string `json:"last"`
	Age    int    `json:"age"`
	Nested nested `json:"nested"`
}

type nested struct {
	Email string `json:"email"`
}

func TestParseParams_ValidParams(t *testing.T) {
	t.Parallel()

	type data struct {
		params   []byte
		expected args
	}

	const (
		name  = "john"
		last  = "doe"
		age   = 30
		email = "john.doe@example.com"
	)

	testData := map[string]data{
		"array": {
			params:   fmt.Appendf(nil, `["%v", "%v", %v, { "email": "%v" }]`, name, last, age, email),
			expected: args{Name: name, Last: last, Age: age, Nested: nested{Email: email}},
		},
		"object": {
			params:   fmt.Appendf(nil, `{"name": "%v", "last": "%v", "age": %v, "nested": { "email": "%v" }}`, name, last, age, email),
			expected: args{Name: name, Last: last, Age: age, Nested: nested{Email: email}},
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			got, err := parse.Params[args](data.params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if got != data.expected {
				t.Errorf("expected %v, got %v", data.expected, got)
			}
		})
	}
}

func TestParseParams_ValidPointerParams(t *testing.T) {
	t.Parallel()

	type data struct {
		params   []byte
		expected args
	}

	const (
		name  = "john"
		last  = "doe"
		age   = 30
		email = "john.doe@example.com"
	)

	testData := map[string]data{
		"array": {
			params:   fmt.Appendf(nil, `["%v", "%v", %v, { "email": "%v" }]`, name, last, age, email),
			expected: args{Name: name, Last: last, Age: age, Nested: nested{Email: email}},
		},
		"object": {
			params:   fmt.Appendf(nil, `{"name": "%v", "last": "%v", "age": %v, "nested": { "email": "%v" }}`, name, last, age, email),
			expected: args{Name: name, Last: last, Age: age, Nested: nested{Email: email}},
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			got, err := parse.Params[*args](data.params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if *got != data.expected {
				t.Errorf("expected %v, got %v", data.expected, got)
			}
		})
	}
}

func TestParseParams_InvalidParams(t *testing.T) {
	t.Parallel()

	type data struct {
		params []byte
	}

	const (
		name  = "john"
		last  = "doe"
		age   = 30
		email = "john.doe@example.com"
	)

	testData := map[string]data{
		"empty": {
			params: []byte{},
		},
		"invalid": {
			params: []byte(`"invalid"`),
		},
		"extra parameters array": {
			params: fmt.Appendf(nil, `["%v", "%v", %v, { "email": "%v" }, "extra"]`, name, last, age, email),
		},
		"extra parameters object": {
			params: fmt.Appendf(nil, `{"name": "%v", "last": "%v", "age": %v, "nested": { "email": "%v" }, "extra": "extra"}`, name, last, age, email),
		},
		"unknown field": {
			// Replace `name` field to not trigger extra parameters error, we want to test the unknown field error.
			params: fmt.Appendf(nil, `{"unknown": "unknown", "last": "%v", "age": %v, "nested": { "email": "%v" }}`, last, age, email),
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			_, err := parse.Params[args](data.params)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseParamsType_InvalidType(t *testing.T) {
	t.Parallel()

	in := []byte(`{ "name": "john", "last": "doe", "age": 30, "nested": { "email": "john.doe@example.com" } }`)

	type data struct {
		t reflect.Type
	}

	testData := map[string]data{
		"array": {
			t: reflect.TypeFor[[]any](),
		},
		"channel": {
			t: reflect.TypeFor[chan any](),
		},
		"func": {
			t: reflect.TypeFor[func() any](),
		},
		"int": {
			t: reflect.TypeFor[int](),
		},
		"map": {
			t: reflect.TypeFor[map[string]any](),
		},
		"string": {
			t: reflect.TypeFor[string](),
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			_, err := parse.ParamsType(data.t, in)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseParamsType_ValidParams(t *testing.T) {
	t.Parallel()

	type data struct {
		params   []byte
		t        reflect.Type
		expected args
	}

	const (
		name  = "john"
		last  = "doe"
		age   = 30
		email = "john.doe@example.com"
	)

	testData := map[string]data{
		"array": {
			params:   fmt.Appendf(nil, `["%v", "%v", %v, { "email": "%v" }]`, name, last, age, email),
			expected: args{Name: name, Last: last, Age: age, Nested: nested{Email: email}},
			t:        reflect.TypeFor[args](),
		},
		"object": {
			params:   fmt.Appendf(nil, `{"name": "%v", "last": "%v", "age": %v, "nested": { "email": "%v" }}`, name, last, age, email),
			expected: args{Name: name, Last: last, Age: age, Nested: nested{Email: email}},
			t:        reflect.TypeFor[args](),
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			got, err := parse.ParamsType(data.t, data.params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if got != data.expected {
				t.Errorf("expected %v, got %v", data.expected, got)
			}
		})
	}
}

func TestParseParamsType_InvalidParams(t *testing.T) {
	t.Parallel()

	type data struct {
		params []byte
		t      reflect.Type
	}

	const (
		name  = "john"
		last  = "doe"
		age   = 30
		email = "john.doe@example.com"
	)

	testData := map[string]data{
		"empty": {
			params: []byte{},
			t:      reflect.TypeFor[args](),
		},
		"invalid": {
			params: []byte(`"invalid"`),
			t:      reflect.TypeFor[args](),
		},
		"extra parameters array": {
			params: fmt.Appendf(nil, `["%v", "%v", %v, { "email": "%v" }, "extra"]`, name, last, age, email),
			t:      reflect.TypeFor[args](),
		},
		"extra parameters object": {
			params: fmt.Appendf(nil, `{"name": "%v", "last": "%v", "age": %v, "nested": { "email": "%v" }, "extra": "extra"}`, name, last, age, email),
			t:      reflect.TypeFor[args](),
		},
		"unknown field": {
			// Replace `name` field to not trigger extra parameters error, we want to test the unknown field error.
			params: fmt.Appendf(nil, `{"unknown": "unknown", "last": "%v", "age": %v, "nested": { "email": "%v" }}`, last, age, email),
			t:      reflect.TypeFor[args](),
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			_, err := parse.ParamsType(data.t, data.params)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
