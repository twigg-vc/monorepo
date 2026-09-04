package textdetect

import "io"

// MUST BE CONSTRUCTED WITH Wrap
type Detector struct {
	w *wrap
}

// Returns true if the data written to the writter was probably textual
func (d Detector) ProbablyWroteText() bool {
	return d.w.ProbablyWroteText()
}

// Wraps a writter so that everything written to it is also written to the
// detector, so we can later analyze if text was written.
// Passsing in `nil` is ok.
// Note that no more than 512 will ever be written, so it's ok to write
// as much as you want and no OOO should occur.
func Wrap(w io.Writer) (io.Writer, Detector) {
	return newWrapper(w)
}
