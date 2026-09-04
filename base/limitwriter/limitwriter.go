package limitwriter

import "io"

type limitWriter struct {
	w         io.Writer
	remaining int64 // bytes remaining
	wrote     int64 // bytes written
	hit       bool  // whether quota was exceeded
}

func newLimitWriter(w io.Writer, n int64) *limitWriter {
	return &limitWriter{w: w, remaining: n}
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.hit {
		return 0, ErrNotEnoughQuota
	}

	if int64(len(p)) > l.remaining {
		// Only part of p fits in the quota
		n, err := l.w.Write(p[:l.remaining])
		l.wrote += int64(n)
		l.remaining -= int64(n)
		l.hit = true
		if err != nil {
			return n, err
		}
		return n, ErrNotEnoughQuota
	}

	n, err := l.w.Write(p)
	l.wrote += int64(n)
	l.remaining -= int64(n)
	return n, err
}
