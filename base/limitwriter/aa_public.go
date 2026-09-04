package limitwriter

import (
	"errors"
	"io"
)

var ErrNotEnoughQuota = errors.New("not enough quota")

// Writer that writes to a destination but stops with
// ErrNotEnoughQuota after n bytes.
type LimitWriter interface {
	io.Writer
}

func New(w io.Writer, n int64) LimitWriter {
	return newLimitWriter(w, n)
}
