package peeker

import "io"

type Peeker struct {
	p *peeker
}

func (pk Peeker) Read(b []byte) (int, error) {
	return pk.p.Read(b)
}

// Returns the peeked data.
// Never returns nil after IsInit().
func (pk Peeker) Peeked() []byte {
	return pk.p.Peeked()
}

// Forces the data to be peeked. This ensures the validity of the
// DataIs* methods even if Read() is never called.
// Read() automatically calls this function on first read.
func (pk Peeker) Init() error {
	return pk.p.Init()
}

// Returns whether the peeker was initialized, which is equivalent to
// wheter Init() or Read() was ever called.
func (pk Peeker) IsInit() bool {
	return pk.p.IsInit()
}

// Since the peeker only reads data lazily (i.e. after the first Read),
// the DataIs* methods can't really be trusted before Read() has been
// called at least once or Init() was called.

// Panics if n > maxBytesPeeked
func (pk Peeker) DataIsSmallerThan(n int) bool {
	return pk.p.DataIsSmallerThan(n)
}

// Panics if n > maxBytesPeeked
func (pk Peeker) DataIsLargerThan(n int) bool {
	return pk.p.DataIsLargerThan(n)
}

// Uses heuristics to identify data that is probably textual
func (pk Peeker) DataIsProbablyText() bool {
	return pk.p.DataIsProbablyText()
}

// Creates an instance that will analyze the data provided by the reader.
// This peeker can then be passed as an io.Reader and it'll provide all the
// original data. The peeker only peeks at the data lazily: it only analyzes the
// data on once the first Read happens. This is done so that the constructor
// never returns errors.
// Note that up to `maxBytesPeeked` bytes are loaded into memory.
func New(data io.Reader, maxBytesPeeked int) Peeker {
	return Peeker{newPeeker(data, maxBytesPeeked)}
}