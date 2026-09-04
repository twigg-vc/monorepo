package textdetect

import (
	"bytes"
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	buff := bytes.NewBuffer(nil)
	w, id := Wrap(buff)
	n, err := w.Write([]byte("abcde"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("wrong n")
	}
	n, err = w.Write([]byte("fgh"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("wrong n")
	}
	if !id.ProbablyWroteText() {
		t.Fatal("data written should be text")
	}
}

func TestNotText(t *testing.T) {
	buff := bytes.NewBuffer(nil)
	w, id := Wrap(buff)
	// write some initial text to ensure it doesnt just check the first char
	_, _ = w.Write([]byte("some text"))
	n, err := w.Write([]byte{0x00, 0xFF, 0xA5, 0x1C, 0x7D})
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("wrong n")
	}
	if id.ProbablyWroteText() {
		t.Fatal("data written should not be text")
	}
}

func TestWriteTooLong(t *testing.T) {
	buff := bytes.NewBuffer(nil)
	w, id := Wrap(buff)
	n, err := w.Write([]byte(strings.Repeat("a", maxBytesWritten+1)))
	if err != nil {
		t.Fatal(err)
	}
	if n != maxBytesWritten+1 {
		t.Fatal("wrong n")
	}
	if !id.ProbablyWroteText() {
		t.Fatal("data written should be text")
	}
}