package clidb

import (
	"encoding/gob"
	"fmt"
	"io"
)

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
