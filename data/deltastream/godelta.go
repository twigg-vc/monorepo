package deltastream

import (
	"bytes"
	"io"

	"monorepo/data/balacode/go-delta"
)

type godeltaCompressor struct {
	old         []byte
	in          *bytes.Buffer
	destination io.Writer
	d           CompressionData
}

func newGodeltaCompressor(old []byte, destination io.Writer) (*godeltaCompressor, func() error) {
	gd := &godeltaCompressor{
		old:         old,
		destination: destination,
		d: CompressionData{
			Method: CompressionMethodGodelta,
		},
		in: bytes.NewBuffer(nil),
	}
	return gd, gd.close
}

func (g godeltaCompressor) Write(p []byte) (int, error) {
	return g.in.Write(p)
}

func (g godeltaCompressor) Data() CompressionData {
	return g.d
}

func (g godeltaCompressor) close() error {
	d := delta.Make(g.old, g.in.Bytes())
	_, err := g.destination.Write(d.Bytes())
	return err
}

// Note: all data will be loaded into memory when Read is called
type godeltaDecompressor struct {
	old          io.Reader
	delta        io.Reader
	uncompressed *bytes.Buffer
	isInit       bool
}

func newGodeltaDecompressor(old io.Reader, delta io.Reader) (
	*godeltaDecompressor, func() error) {
	return &godeltaDecompressor{
		old:          old,
		delta:        delta,
		uncompressed: bytes.NewBuffer(nil),
	}, func() error { return nil }
}

func (r *godeltaDecompressor) Read(p []byte) (int, error) {
	if !r.isInit {
		oldBuff := bytes.NewBuffer(nil)
		_, err := io.Copy(oldBuff, r.old)
		if err != nil {
			return 0, err
		}
		deltaBuff := bytes.NewBuffer(nil)
		_, err = io.Copy(deltaBuff, r.delta)
		if err != nil {
			return 0, err
		}

		d, err := delta.Load(deltaBuff.Bytes())
		if err != nil {
			return 0, err
		}
		new, err := d.Apply(oldBuff.Bytes())
		if err != nil {
			return 0, err
		}
		r.uncompressed = bytes.NewBuffer(new)
		r.isInit = true
	}

	return r.uncompressed.Read(p)
}
