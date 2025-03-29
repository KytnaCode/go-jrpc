package params

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/kytnacode/go-jrpc/jsonutil"
)

var (
	// ErrInvalidParams is returned when an error occurs while parsing the params.
	ErrInvalidParams = errors.New("invalid params")
)

// ParseParams parses the params and returns the value of type T.
// The params can be an array or an object.
// If the params are an array, the fields of T must be in the same order as the elements of the array.
// If the params are an object, the fields of T must have the same name as the keys of the object or have a json tag with the same name.
// Only exported fields are considered.
func ParseParams[T any](params []byte) (T, error) {
	t := reflect.TypeFor[T]()

	v, err := ParseParamsType(t, params)
	if err != nil {
		zero, _ := v.(T) // Safe to convert.

		return zero, err
	}

	value, _ := v.(T) // Safe to convert.

	return value, nil
}

// Custom unmarshaler for the params.
func (pw *paramsWrapper) UnmarshalJSON(b []byte) error {
	// Trim left whitespace.
	trimmed := jsonutil.TrimLeftWhitespace(b)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty JSON: %w", ErrInvalidParams)
	}

	// Check if the params are an array or an object.
	if trimmed[0] == '[' { // array
		return pw.parsePositional(b)
	} else if trimmed[0] == '{' { // object
		return pw.parseNamed(b)
	} else { // invalid
		return fmt.Errorf("parameters must be an array or an object: %w", ErrInvalidParams)
	}
}

func ParseParamsType(t reflect.Type, params []byte) (any, error) {
	if t.Kind() != reflect.Struct {
		return reflect.Zero(t).Interface(), fmt.Errorf("params must be a struct: %w", ErrInvalidParams)
	}

	// Custom unmarshaler for the params.
	pw := paramsWrapper{
		t: t,
	}

	if err := json.Unmarshal(params, &pw); err != nil {
		return reflect.Zero(t).Interface(), err
	}

	return pw.value, nil
}

// parsePositional parses the params when they are an array.
func (pw *paramsWrapper) parsePositional(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))

	// Start array.
	_, err := dec.Token()
	if err != nil {
		return err
	}

	params := reflect.New(pw.t) // Output params.

	for i := range pw.t.NumField() {
		// Unmarshal the field.
		field := reflect.New(pw.t.Field(i).Type)

		if err := dec.Decode(field.Interface()); err != nil {
			return err
		}

		// Set the field.
		params.Elem().Field(i).Set(field.Elem())
	}

	if dec.More() {
		return fmt.Errorf("extra parameters in array: %w", ErrInvalidParams)
	}

	pw.value = params.Elem().Interface()

	// End array.
	_, err = dec.Token()
	if err != nil {
		return err
	}

	return nil
}

// parseNamed parses params when they are an object.
func (pw *paramsWrapper) parseNamed(b []byte) error {
	params := reflect.New(pw.t) // Output params.

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()

	// Unmarshal the object.
	if err := dec.Decode(params.Interface()); err != nil {
		return err
	}

	pw.value = params.Elem().Interface()

	return nil
}
