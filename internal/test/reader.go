package test

// NoOpReader is a no-op reader that does nothing when reading. Never return EOF nor other errors.
type NoOpReader struct{}

// Read implements the io.Reader interface, always returning 0 bytes read and nil error.
func (r *NoOpReader) Read(_ []byte) (n int, err error) {
	return 0, nil
}
