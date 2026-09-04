package deltastream

import (
	"bytes"
	"errors"
	"io"
	"monorepo/data/deltastream/peeker"
)

// This is entry just serves the purpose of receiving some bytes to then
// forward the writes to the best suited delta encoder.
type frontCompressor struct {
	old peeker.Peeker

	destination io.Writer
	buff        *bytes.Buffer

	w      Writer // chosen writer
	closeW func() error
}

const peekSize = 1024 * 1024 // 1 MB

func newCompressor(old io.Reader, destination io.Writer) (Writer, func() error) {
	if old == nil {
		return newFlateCompressor(destination)
	}

	pk := peeker.New(old, peekSize) // will initialize lazilly on first Write
	f := &frontCompressor{
		old:         pk,
		destination: destination,
		buff:        bytes.NewBuffer(nil),
		w:           nil,
	}
	return f, f.close
}

func (f *frontCompressor) Write(p []byte) (n int, err error) {
	if !f.old.IsInit() {
		err = f.old.Init()
		if err != nil {
			return
		}
	}

	if f.hasChosenWriter() {
		return f.w.Write(p)
	}

	// err is always nil for buffer Writes
	n, _ = f.buff.Write(p)

	if f.maybeChooseDuringWrite() {
		// If a writer was chosen, dump all the buffer to it and continue
		_, err = io.Copy(f.w, f.buff)
		if err != nil {
			// Close the chosen writer if that didn't work; we'll try
			// chosing it again next time Write is called
			f.closeW()
			f.w = nil
			// Discard the last n bytes written to the buff as the write
			// didn't succeed.
			f.buff.Truncate(f.buff.Len() - n)
			return 0, err
		}
		return
	}

	return
}

func (f *frontCompressor) Data() CompressionData {
	if !f.hasChosenWriter() {
		// A writer is always chosen either as we write or on close
		panic("tried to get CompressionData before closing")
	}
	return f.w.Data()
}

func (f frontCompressor) hasChosenWriter() bool {
	return f.w != nil
}

// Might choose a writer during the writing (for example if we already wrote
// too much that the data would not fit in memory for some compressors).
func (f *frontCompressor) maybeChooseDuringWrite() (choseSomething bool) {
	// We only choose prematurely if we wrote more than peekSize, as that
	// will imply the written data is big so we can't use compressions that
	// rely on the data fitting in memory
	if f.buff.Len() < peekSize {
		return false
	}
	// In here: f.buff.Len() >= peekSize
	choseSomething = true

	w, cl := newFlateCompressor(f.destination)
	f.w = w
	f.closeW = cl
	return
}

const minSizeForGodelta = 500

// If we reach Close() without a chosen writer, one needs to be chosen here.
func (f *frontCompressor) chooseOnClose() {
	if f.hasChosenWriter() {
		panic("already chosen")
	}

	// If old and new content are small, use godelta for great speed
	// and compression. We can't use it for files too large.
	// It has a header overhead so we should not use it for tiny content either
	if f.old.DataIsSmallerThan(peekSize) && f.buff.Len() < peekSize &&
		f.old.DataIsLargerThan(minSizeForGodelta) &&
		f.buff.Len() > minSizeForGodelta {
		w, cl := newGodeltaCompressor(f.old.Peeked(), f.destination)
		f.w = w
		f.closeW = cl
		return
	}

	// The fallback is always the simple flate compressor
	w, cl := newFlateCompressor(f.destination)
	f.w = w
	f.closeW = cl
}

func (f *frontCompressor) close() error {
	if !f.old.IsInit() {
		err := f.old.Init()
		if err != nil {
			return err
		}
	}
	if f.hasChosenWriter() {
		return f.closeW()
	}

	if !f.hasChosenWriter() {
		f.chooseOnClose()
	}
	_, err := io.Copy(f.w, f.buff)
	return errors.Join(err, f.closeW())
}

func getDecompressor(old, compressed io.Reader, m CompressionMethod) (r io.Reader, close func() error) {
	switch m {
	case CompressionMethodSpeedFlate:
		return newFlateDecompressor(compressed)
	case CompressionMethodGodelta:
		return newGodeltaDecompressor(old, compressed)
	default:
		panic("decompressor not implemented")
	}
}
