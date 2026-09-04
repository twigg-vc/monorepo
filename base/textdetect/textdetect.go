package textdetect

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

type wrap struct {
	w               io.Writer
	b               *bytes.Buffer
	isDoneBuffering bool
	n               int
}

// http.DetectContentType reads at most 512 bytes, so no point in writing
// more than that.
const maxBytesWritten = 512

func newWrapper(w io.Writer) (io.Writer, Detector) {
	wr := &wrap{
		w:               w,
		b:               bytes.NewBuffer(nil),
		isDoneBuffering: false,
		n:               0,
	}
	return wr, Detector{w: wr}
}

func (t *wrap) Write(p []byte) (int, error) {
	if t.isDoneBuffering {
		if t.w == nil {
			return 0, nil
		}
		return t.w.Write(p)
	}
	var n int
	if len(p) > maxBytesWritten {
		// err is always nil
		n, _ = t.b.Write(p[:maxBytesWritten])
	} else {
		n, _ = t.b.Write(p)
	}
	t.n += n
	if t.n >= maxBytesWritten {
		t.isDoneBuffering = true
	}

	if t.w == nil {
		return 0, nil
	}
	return t.w.Write(p)
}

func (t wrap) ProbablyWroteText() bool {
	buffBytes := t.b.Bytes()
	if strings.Contains(http.DetectContentType(buffBytes), "text") ||
		len(buffBytes) == 0 {
		return true
	}
	return false
}