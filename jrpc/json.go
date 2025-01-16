package jrpc

import "encoding/json"

// isArray check if raw is a json array.
func isArray(raw json.RawMessage) bool {
	for _, char := range raw {
		if isWhitespace(char) {
			continue
		}

		return char == '['
	}

	return false
}

// isWhitespace check if a character is an insignificant whitespace (RFC 8259).
func isWhitespace(char byte) bool {
	// Space || Horizontal tab || New line || Carriage return
	return char == 0x20 || char == 0x09 || char == 0x0a || char == 0x0d
}
