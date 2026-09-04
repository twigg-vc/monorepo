package deltastream

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"sync"
)

type flateCompressor struct {
	flateW   io.WriteCloser
	isClosed bool
	d        CompressionData
}

const flateCompression = flate.BestSpeed

// Doesnt take the parent as input bc it doesnt do delta compression
func newFlateCompressor(out io.Writer) (*flateCompressor, func() error) {
	z := flateWriterPool.Get().(*flate.Writer)
	z.Reset(out)
	f := flateCompressor{
		flateW: z,
		d: CompressionData{
			Method: CompressionMethodSpeedFlate,
		},
	}
	return &f, f.close
}

func (f flateCompressor) Write(p []byte) (int, error) {
	return f.flateW.Write(p)
}
func (f flateCompressor) Data() CompressionData {
	return f.d
}

func (f *flateCompressor) close() error {
	if f.isClosed {
		return nil
	}
	f.isClosed = true
	err := f.flateW.Close()
	if err != nil {
		return err
	}
	flateWriterPool.Put(f.flateW)
	return nil
}

type flateDecomp struct {
	data     io.ReadCloser
	isClosed bool
}

func newFlateDecompressor(data io.Reader) (*flateDecomp, func() error) {
	flateR := GetFlateReader(data)
	d := &flateDecomp{
		data:     flateR,
		isClosed: false,
	}
	return d, d.close
}

func (d *flateDecomp) Read(p []byte) (int, error) {
	return d.data.Read(p)
}

func (d *flateDecomp) close() error {
	if d.isClosed {
		return nil
	}
	d.isClosed = true
	return d.data.Close()
}

// flateReaderPool pools flate.Reader instances to reduce allocations.
var flateReaderPool = sync.Pool{
	New: func() any {
		// Create a flate.Reader with a dummy reader. We'll reset it before use.
		r := bytes.NewReader(nil)
		fr := flate.NewReader(r)
		return fr
	},
}

// GetFlateReader returns a flate reader wrapping `r`.
// The returned reader must be closed to return it to the pool.
func GetFlateReader(r io.Reader) io.ReadCloser {
	fr := flateReaderPool.Get().(io.ReadCloser)
	// Reset the underlying reader
	if resetter, ok := fr.(flate.Resetter); ok {
		if err := resetter.Reset(r, nil); err != nil {
			// On error, discard and create new
			fr.Close()
			return flate.NewReader(r)
		}
	}
	return &pooledFlateReader{fr}
}

// pooledFlateReader wraps a flate.Reader and returns it to the pool on Close.
type pooledFlateReader struct {
	io.ReadCloser
}

func (p *pooledFlateReader) Close() error {
	// Return the reader to the pool instead of closing underlying
	flateReaderPool.Put(p.ReadCloser)
	return nil
}

var flateWriterPool = sync.Pool{
	New: func() any {
		// Create a flate.Writer with a dummy writer; will Reset before use
		w, err := flate.NewWriter(io.Discard, flateCompression)
		if err != nil {
			panic(fmt.Sprintf("flate.NewWriter: %v", err))
		}
		return w
	},
}