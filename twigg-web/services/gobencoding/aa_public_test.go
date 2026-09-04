package gobencoding_test

import (
	"bytes"
	"monorepo/twigg-web/services/gobencoding"
	"testing"
)

type point struct {
	X, Y int
}

func Test_EncodeDecode(t *testing.T) {
	in := point{X: 1, Y: 2}
	out, err := gobencoding.Decode[point](gobencoding.Encode(in))
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("got %+v, expected %+v", out, in)
	}
}

func Test_StructWriterToReadIntoStruct(t *testing.T) {
	in := point{X: 3, Y: 4}
	var buf bytes.Buffer
	n, err := gobencoding.StructWriterTo(in).WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("reported n=%d, expected %d", n, buf.Len())
	}

	closed := false
	out, err := gobencoding.ReadIntoStruct[point](&buf, func() { closed = true })
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("got %+v, expected %+v", out, in)
	}
	if !closed {
		t.Fatal("closeR was not called")
	}
}
