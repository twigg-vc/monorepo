package clidb

import (
	"encoding/gob"
	"fmt"
	"io"
)

type gobWriterTo[T any] struct{ x T }

func (g gobWriterTo[T]) WriteTo(w io.Writer) (int64, error) {
	cw := &countWriter{w: w}
	enc := gob.NewEncoder(cw)
	err := enc.Encode(g.x)
	return cw.n, err
}

type countWriter struct {
	w io.Writer
	n int64
}

func (cw *countWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

func structWriterTo[T any](x T) io.WriterTo { return gobWriterTo[T]{x} }

func readIntoStruct[T any](r io.Reader, closeR func()) (T, error) {
	var x T
	defer closeR()
	dec := gob.NewDecoder(r)
	err := dec.Decode(&x)
	if err != nil {
		return x, fmt.Errorf("failed to decode data: %w", err)
	}
	return x, nil
}
