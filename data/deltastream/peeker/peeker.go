package peeker

import (
	"bytes"
	"io"
	"monorepo/base/textdetect"
)

func newPeeker(r io.Reader, n int) *peeker {
	if n < minN {
		n = minN
	}
	return &peeker{
		r:                  r,
		n:                  n,
		dataIsProbablyText: true, // Starts out as true if nothing is written
	}
}

func (pr *peeker) Init() error {
	return pr.init()
}
func (pr peeker) IsInit() bool {
	return pr.isInit
}

func (pr peeker) Peeked() []byte {
	return pr.peeked
}

func (pr *peeker) Read(p []byte) (int, error) {
	if !pr.isInit {
		err := pr.init()
		if err != nil {
			return 0, err
		}
	}
	if pr.buffer.Len() > 0 {
		return pr.buffer.Read(p)
	}
	return pr.r.Read(p)
}

func (pr peeker) DataIsSmallerThan(n int) bool {
	if n > pr.n {
		// pr.peeked will be at most of size pr.n
		// Even if it's full, we can't tell if the data is smaller than
		// something larger than its size, bc we won't have the data.
		panic("called DataIsSmallerThan n > maxBytesPeeked")
	}
	return len(pr.peeked) < n
}

func (pr peeker) DataIsLargerThan(n int) bool {
	if n > pr.n {
		// See DataIsSmallerThan
		panic("called DataIsLargerThan n > maxBytesPeeked")
	}
	return len(pr.peeked) > n
}

func (pr peeker) DataIsProbablyText() bool {
	return pr.dataIsProbablyText
}

// This is required for identifying text content
const minN = 512

// peeker wraps an io.Reader, reads some initial bytes for inspection,
// and allows subsequent reads to include the peeked bytes.
type peeker struct {
	r                  io.Reader
	buffer             *bytes.Buffer
	peeked             []byte
	n                  int
	dataIsProbablyText bool
	isInit             bool
}

func (pr *peeker) init() error {
	if pr.isInit {
		return nil
	}
	pr.isInit = true

	buf := make([]byte, pr.n)
	readN, err := io.ReadFull(pr.r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return err
	}
	pr.buffer = bytes.NewBuffer(buf[:readN])
	pr.peeked = buf[:readN]

	w, td := textdetect.Wrap(nil)
	_, err = w.Write(pr.peeked)
	pr.dataIsProbablyText = td.ProbablyWroteText()
	return err
}
