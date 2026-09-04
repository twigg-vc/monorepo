// Text Sanitization Engine
package tse

import (
	"io"
)

type Scrubber struct {
	s *scrubber
}

func NewTseScrubber(w io.Writer, secrets []string) (s Scrubber, closeScrubber func() error, err error) {
	return newTseScrubber(w, secrets)
}

func (s *Scrubber) Write(p []byte) (n int, err error) {
	return s.s.write(p)
}

// flushes only the portion of the buffer that is guaranteed
// not to contain any partial secret matches.
func (s *Scrubber) Flush() error {
	return s.s.flush(false)
}

// flushes the underlying buffer. Unlike `Flush`, it doesn't flush only the
// portion that is guaranteed to not contain partial secrets. I.e. if a caller
// calls Write("SE"), UnsafeFlush(), Write("CRET"), UnsafeFlush(); the output
// will show "SECRET".
func (s *Scrubber) UnsafeFlush() error {
	return s.s.flush(true)
}

var MaskPlaceholder = []byte("*******")