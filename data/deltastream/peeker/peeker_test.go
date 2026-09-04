package peeker

import (
	"bytes"
	"testing"
)

func TestAll(t *testing.T) {

	data := bytes.NewBufferString("abcde")
	p := New(data, 100)

	if p.IsInit() {
		t.Fatal("peeker was not yet initialized via Read not Init")
	}
	if p.Peeked() != nil {
		t.Fatal("peeked is still nil")
	}

	b := make([]byte, 5)
	n, err := p.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("expected to read 5 bytes")
	}
	if string(b) != "abcde" {
		t.Fatal("read wrong content")
	}
	if !p.IsInit() {
		t.Fatal("peeker was initialized via Read")
	}

	if !p.DataIsProbablyText() {
		t.Fatal("data should be text")
	}
	if !p.DataIsSmallerThan(20) {
		t.Fatal("data should < 20")
	}
	if p.DataIsSmallerThan(3) {
		t.Fatal("data should not be < 3")
	}
	if !p.DataIsLargerThan(3) {
		t.Fatal("data should be > 3")
	}
	if p.DataIsLargerThan(5) {
		t.Fatal("data should not be > 5")
	}

}

func TestBinary(t *testing.T) {

	// Non textual data of size 5
	data := bytes.NewBuffer([]byte{0x00, 0xFF, 0xA5, 0x1C, 0x7D})
	p := New(data, 10)

	b := make([]byte, 5)
	n, err := p.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatal("expected to read 5 bytes")
	}
	if !bytes.Equal(b, []byte{0x00, 0xFF, 0xA5, 0x1C, 0x7D}) {
		t.Fatal("read wrong data")
	}

	if p.DataIsProbablyText() {
		t.Fatal("data should not be text")
	}

}

func TestWrongUsageBeforeInit(t *testing.T) {

	data := bytes.NewBufferString("content larger than 5")
	p := New(data, 100)

	// Note that before Init() or Read() is called, DataIsSmallerThan
	// will return `true` as it has seen no data yet.
	if !p.DataIsSmallerThan(5) {
		t.Fatal("should not yet know that the data is smaller")
	}

	err := p.Init()
	if err != nil {
		t.Fatal(err)
	}

	// After init, we'll know correctly that the content is larger than 5
	if p.DataIsSmallerThan(5) {
		t.Fatal("should know that the data is >5")
	}

}
