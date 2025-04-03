package jsonutil

import "bytes"

func TrimLeftWhitespace(b []byte) []byte {
	return bytes.TrimLeft(b, " \t\n\r") // Trim space, tab, newline, and carriage return, see RFC 8259.
}
