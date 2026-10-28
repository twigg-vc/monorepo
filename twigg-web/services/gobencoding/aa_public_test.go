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

func Test_ReadIntoStruct(t *testing.T) {
	in := point{X: 3, Y: 4}
	closed := false
	out, err := gobencoding.ReadIntoStruct[point](
		bytes.NewReader(gobencoding.Encode(in)), func() { closed = true })
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
