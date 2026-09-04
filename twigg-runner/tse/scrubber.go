package tse

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

type scrubber struct {
	dest io.Writer

	secrets           [][]byte
	maskPlacerHolder  []byte
	buffer            []byte
	maxCensoredValLen int
	needsCleanUp      bool
	missingForCleanup int
}

const buffLen = 4 * 1024

func newTseScrubber(w io.Writer, secrets []string) (s Scrubber, closeScrubber func() error, err error) {

	// Sort secrets by length (longest first) so longer secrets like "abc"
	// are replaced before shorter overlapping ones like "bc", avoiding
	// partial leaks (e.g., "a**").
	// See test: TestWriter_OverlappingSecrets
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})

	var byteSecrets [][]byte
	maxCensoredValLen := 0
	if len(secrets) > 0 {
		maxCensoredValLen = len(secrets[0]) // Longest
	}

	for _, secret := range secrets {
		if secret == "" {
			return Scrubber{}, func() error { return nil }, fmt.Errorf("tse: secrets contains empty string")
		}
		b := []byte(secret)
		byteSecrets = append(byteSecrets, b)
	}

	s = Scrubber{
		s: &scrubber{
			dest:             w,
			secrets:          byteSecrets,
			maskPlacerHolder: MaskPlaceholder,
			// Pre-allocate a reasonable capacity to avoid early re-allocations
			buffer:            make([]byte, 0, buffLen),
			maxCensoredValLen: maxCensoredValLen,
			needsCleanUp:      false,
			missingForCleanup: 0,
		},
	}
	closeScrubber = s.s.closeScrubber
	err = nil
	return
}
func (s *scrubber) write(p []byte) (int, error) {
	if len(s.secrets) == 0 {
		return s.dest.Write(p)
	}

	if s.needsCleanUp {
		err := s.cleanUp()
		if err != nil {
			return 0, fmt.Errorf("cleanup failed: %s", err)
		}
	}

	// Append incoming data safely without string casting
	// From this point on write() will always return len(p), nil, since p is
	// already saved in buffer
	s.buffer = append(s.buffer, p...)
	// Scrub the buffer
	// bytes.Replace is highly optimized and only allocates memory IF a match is found.
	for _, secret := range s.secrets {
		s.buffer = bytes.ReplaceAll(s.buffer, secret, s.maskPlacerHolder)
	}
	// Flush if needed
	if len(s.buffer) >= s.maxCensoredValLen {
		_ = s.flush( /*force=*/ false)
	}
	// Always return len(p) on success to satisfy the io.Writer contract
	return len(p), nil
}

// Writes any remaining from buffer to the destination
func (s *scrubber) closeScrubber() error {
	_, err := s.flushN(-1)
	s.buffer = nil
	return err
}

// if !force, flushes only the portion of the buffer that is guaranteed
// not to contain any partial secret matches. If force, will flush anyway.
// using force just means that secrets split accross two separate writes wont
// be redacted - so it must be used with care.
//
// It keeps the last (maxCensoredValLen - 1) bytes in the buffer because
// a secret may span across chunk boundaries. Flushing beyond this point
// could leak part of a secret that hasn't been fully matched yet.
func (s *scrubber) flush(force bool) error {
	if len(s.buffer) == 0 {
		return nil
	}

	if s.needsCleanUp {
		err := s.cleanUp()
		if err != nil {
			return fmt.Errorf("cleanup failed while flushing: %s", err)
		}
	}

	// Compute the safe boundary
	safeEnd := len(s.buffer) - (s.maxCensoredValLen - 1)
	if force {
		safeEnd = len(s.buffer)
	}
	// If nothing is safe yet, do nothing
	if safeEnd <= 0 {
		return nil
	}
	written, err := s.flushN(safeEnd)
	if err != nil {
		s.needsCleanUp = true
		s.missingForCleanup = safeEnd - written
	}
	return nil
}

// flush writes up to n bytes from the internal buffer to the destination.
// The write range is buffer[:n] (n is exclusive).
//
// After writing, the remaining bytes (buffer[n:]) are shifted to the
// beginning of the buffer so they can be reused without allocating.
// The buffer capacity is preserved. If err not nil then shifts only
// successfully written ones.
//
// Special cases:
//   - n == -1: flush the entire buffer
//   - n > len(buffer): treated as len(buffer)
//
// It returns the number of bytes written to the destination.
func (s *scrubber) flushN(n int) (int, error) {
	if len(s.buffer) == 0 {
		return 0, nil
	}
	if n < 0 || n > len(s.buffer) {
		n = len(s.buffer)
	}

	toWrite := s.buffer[:n]
	written, err := s.dest.Write(toWrite)

	// Shift the buffer to written
	// Using copy() safely handles overlapping slices and reuses the existing
	// memory array, preventing the buffer from growing indefinitely.
	tailLen := len(s.buffer) - written
	copy(s.buffer[:tailLen], s.buffer[written:])
	s.buffer = s.buffer[:tailLen]

	return written, err
}

// Flushes bytes missing for cleanup
func (s *scrubber) cleanUp() error {
	written, err := s.flushN(s.missingForCleanup)
	if err != nil {
		s.needsCleanUp = true
		s.missingForCleanup = s.missingForCleanup - written
		return err
	}
	s.needsCleanUp = false
	s.missingForCleanup = 0
	return nil
}