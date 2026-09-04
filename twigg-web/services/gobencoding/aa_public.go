package gobencoding

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
)

func Encode(v any) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(v)
	if err != nil {
		panic(fmt.Errorf("failed to encode: %s", err))
	}
	return buf.Bytes()
}
func Decode[T any](data []byte) (val T, err error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(&val)
	return
}

// Wraps x so it can be streamed straight into an io.Writer (e.g. a blob
// store's SetBlob) without first buffering the whole encoding in memory.
func StructWriterTo[T any](x T) io.WriterTo {
	return gobWriterTo[T]{x}
}

// Reads and gob-decodes a value of type T from r. closeR is always called.
func ReadIntoStruct[T any](r io.Reader, closeR func()) (T, error) {
	var x T
	defer closeR()
	dec := gob.NewDecoder(r)
	err := dec.Decode(&x)
	if err != nil {
		return x, fmt.Errorf("failed to decode data: %w", err)
	}
	return x, nil
}

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
