package params

import (
	"fmt"
	"testing"
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
			t.Log(string(data.params))
			got, err := ParseParams[args](data.params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if got != data.expected {
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
			_, err := ParseParams[args](data.params)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
