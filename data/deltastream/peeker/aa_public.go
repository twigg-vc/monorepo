package peeker

import "io"

type Peeker interface {
	io.Reader

	// Returns the peeked data.
	// Never returns nil after IsInit().
	Peeked() []byte

	// Forces the data to be peeked. This ensures the validity of the
	// DataIs* methods even if Read() is never called.
	// Read() automatically calls this function on first read.
	Init() error
	// Returns whether the peeker was initialized, which is equivalent to
	// wheter Init() or Read() was ever called.
	IsInit() bool

	// Since the peeker only reads data lazily (i.e. after the first Read),
	// the DataIs* methods can't really be trusted before Read() has been
	// called at least once or Init() was called.

	// Panics if n > maxBytesPeeked
	DataIsSmallerThan(n int) bool

	// Panics if n > maxBytesPeeked
	DataIsLargerThan(n int) bool

	// Uses heuristics to identify data that is probably textual
	DataIsProbablyText() bool
}

// Creates an instance that will analyze the data provided by the reader.
// This peeker can then be passed as an io.Reader and it'll provide all the
// original data. The peeker only peeks at the data lazily: it only analyzes the
// data on once the first Read happens. This is done so that the constructor
// never returns errors.
// Note that up to `maxBytesPeeked` bytes are loaded into memory.
func New(data io.Reader, maxBytesPeeked int) Peeker {
	return newPeeker(data, maxBytesPeeked)
}
