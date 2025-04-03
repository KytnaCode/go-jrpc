package jsonutil_test

import (
	"testing"

	"github.com/kytnacode/go-jrpc/internal/jsonutil"
)

func TestTrimLeftWhitespace(t *testing.T) {
	t.Parallel()

	type data struct {
		input    []byte
		expected []byte
	}

	testData := map[string]data{
		"space": {
			input:    []byte("  32"),
			expected: []byte("32"),
		},
		"tab": {
			input:    []byte("\t\t{ \"hello\": \"world\" }"),
			expected: []byte("{ \"hello\": \"world\" }"),
		},
		"newline": {
			input:    []byte("\n\n[1, 2, 3]"),
			expected: []byte("[1, 2, 3]"),
		},
		"carriage return": {
			input:    []byte("\r\r{ \"hello\": \"world\" }"),
			expected: []byte("{ \"hello\": \"world\" }"),
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			got := jsonutil.TrimLeftWhitespace(data.input)

			if string(got) != string(data.expected) {
				t.Errorf("expected %s, got %s", data.expected, got)
			}
		})
	}
}
