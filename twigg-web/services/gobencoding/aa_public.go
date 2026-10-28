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
