package appendlog

import (
	"errors"
	"io"
	"monorepo/data/appendlog/tiered"
)

type appendLog struct {
	p tiered.Provider
}

func new(p tiered.Provider) *appendLog {
	return &appendLog{
		p: p,
	}
}

func (d *appendLog) Name() string {
	return d.p.Name()
}

func (d *appendLog) Size() (int64, error) {
	return d.p.Size()
}

func (d *appendLog) Sync() error {
	return d.p.GetWrite().Sync()
}

func (d *appendLog) Write(p []byte) (nTotal int, err error) {
	var n int
	var sizeLeft int64
	for nTotal < len(p) {
		sizeLeft, err = d.p.GetWrite().SizeLeft()
		if err != nil {
			return
		}
		if sizeLeft == 0 {
			err = d.p.Expand()
			if err != nil {
				return
			}
			continue
		}
		writeUntil := int64(nTotal) + sizeLeft
		if writeUntil > int64(len(p)) {
			writeUntil = int64(len(p))
		}
		n, err = d.p.GetWrite().Write(p[nTotal:writeUntil])
		nTotal += n
		if err != nil {
			return
		}
	}
	return
}

func (d *appendLog) ReadAt(p []byte, off int64) (nTotal int, err error) {
	defer func() {
		// Per io.ReaderAt spec, ReadAt should never return io.EOF if n > 0.
		if errors.Is(err, io.EOF) && nTotal > 0 {
			err = io.ErrUnexpectedEOF
		}
	}()

	if off < 0 {
		return 0, errors.New("negative offset")
	}
	totalSize, err := d.p.Size()
	if err != nil {
		return
	}
	if off >= totalSize {
		return 0, io.EOF
	}

	for len(p) > 0 {
		blockIndex := off / d.p.BlockSize()
		offInBlock := off % d.p.BlockSize()
		if blockIndex >= d.p.Blocks() {
			if nTotal == 0 {
				err = io.EOF
				return
			}
			err = io.ErrUnexpectedEOF
			return
		}

		var readBlock tiered.ReadBlock
		var closeReadBlock func()
		readBlock, closeReadBlock, err = d.p.GetRead(blockIndex)
		if err != nil {
			return
		}

		nToReadFromBlock := min(int64(len(p)), d.p.BlockSize()-offInBlock)
		var n int
		n, err = io.ReadFull(
			io.NewSectionReader(readBlock, offInBlock, nToReadFromBlock),
			p[:nToReadFromBlock])
		nTotal += n
		off += int64(n)
		p = p[n:]
		closeReadBlock()
		if err != nil {
			return
		}
	}
	return
}
