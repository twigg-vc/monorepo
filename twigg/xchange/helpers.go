package xchange

import (
	"errors"
	"io"
)

// Converts an io.Reader into an io.WriterTo
type readerToWriterTo struct {
	r io.Reader
}

func (rtwt readerToWriterTo) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, rtwt.r)
}

// Reader that returns an error on read
type unavailableReader struct{}

func (u unavailableReader) Read(p []byte) (int, error) {
	return 0, errors.New("file not available")
}
