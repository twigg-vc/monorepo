package limitwriter

import (
	"errors"
	"io"
)

var ErrNotEnoughQuota = errors.New("not enough quota")

// Writer that writes to a destination but stops with
// ErrNotEnoughQuota after n bytes.
type LimitWriter struct {
	w *limitWriter
}

func (l LimitWriter) Write(p []byte) (int, error) {
	return l.w.Write(p)
}

func New(w io.Writer, n int64) LimitWriter {
	return LimitWriter{newLimitWriter(w, n)}
}
