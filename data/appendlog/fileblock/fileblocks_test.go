package fileblock

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(t *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	testDir := filepath.Join(wd, "test")
	os.RemoveAll(testDir)
	t.Run()
	os.RemoveAll(testDir)
}

func TestReadNotFound(t *testing.T) {
	_, close, err := NewFileReadBlock("test/f.txt")
	// Ok to call close many times
	close()
	close()
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestSimpleWriteAndReadAt(t *testing.T) {
	w, closeW, err := NewFileWriteBlock("test/f.txt", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := w.SizeLeft(); s != 2 {
		t.Fatal("expeted 2 size left")
	}
	n, err := w.Write([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("expected to write 1")
	}
	if s, _ := w.SizeLeft(); s != 1 {
		t.Fatal("expeted 1 size left")
	}
	n, err = w.Write([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("expected to write 1")
	}

	if s, _ := w.SizeLeft(); s != 0 {
		t.Fatal("expeted 0 size left")
	}
	_, err = w.Write([]byte("c"))
	if err == nil {
		t.Fatal("expected err bc should be full")
	}

	p := make([]byte, 1)
	n, err = w.ReadAt(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("expected to read 1")
	}
	if p[0] != 'b' {
		t.Fatal("expected to read b")
	}

	// Ok to close many times
	err = closeW()
	if err != nil {
		t.Fatal(err)
	}
	err = closeW()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewWithTrucateDeletes(t *testing.T) {
	w, closeW, _ := NewFileWriteBlock("test/f.txt", 3, true)
	w.Write([]byte("abc"))
	closeW()
	// Open again truncating
	w, closeW, err := NewFileWriteBlock("test/f.txt", 3, true)
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := w.Size(); s != 0 {
		t.Fatal("expected size 0")
	}
}

func TestTrucate(t *testing.T) {
	w, closeW, _ := NewFileWriteBlock("test/f.txt", 3, true)
	defer closeW()
	w.Write([]byte("abc"))
	if s, _ := w.Size(); s != 3 {
		t.Fatalf("got size %d", s)
	}
	err := w.Trucate()
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := w.Size(); s != 0 {
		t.Fatalf("got size %d", s)
	}
	w.Write([]byte("def"))
	if s, _ := w.Size(); s != 3 {
		t.Fatalf("got size %d", s)
	}
	b := make([]byte, 3)
	w.ReadAt(b, 0)
	if !bytes.Equal(b, []byte("def")) {
		t.Fatal("read wrong bytes")
	}
}

func TestWriteNotTruncated(t *testing.T) {
	w, closeW, _ := NewFileWriteBlock("test/f.txt", 3, true)
	w.Write([]byte("abc"))
	closeW()
	// Open again without truncating
	w, closeW, err := NewFileWriteBlock("test/f.txt", 3, false)
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	p := make([]byte, 2)
	w.ReadAt(p, 1)
	expectedP := []byte{'b', 'c'}
	if !bytes.Equal(p, expectedP) {
		t.Fatal("unexpected read at")
	}
}

func TestWriteAndRead(t *testing.T) {
	w, closeW, _ := NewFileWriteBlock("test/f.txt", 5, true)
	w.Write([]byte("abcde"))
	closeW()

	fR, closeR, err := NewFileReadBlock("test/f.txt")
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	// Section starts at zero and has size 2
	sectionR := io.NewSectionReader(fR, 0, 2)

	p := make([]byte, 1)
	n, err := sectionR.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("got %d bytes read", n)
	}
	if p[0] != 'a' {
		t.Fatal("wrong readAt")
	}
	n, err = sectionR.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("got %d bytes read", n)
	}
	if p[0] != 'b' {
		t.Fatal("wrong readAt")
	}

	// Close and re-initialize at a different offset
	closeR()
	fR, closeR, err = NewFileReadBlock("test/f.txt")
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	// Section starts at 4 and has size 1
	sectionR = io.NewSectionReader(fR, 4, 1)
	n, err = sectionR.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("got %d bytes read", n)
	}
	if p[0] != 'e' {
		t.Fatal("wrong readAt")
	}
}

func TestEverythingFailsAfterClose(t *testing.T) {
	// Write a dummy test file
	w, closeW, err := NewFileWriteBlock("test/f.txt", 5, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("abcde"))
	if err != nil {
		t.Fatal(err)
	}
	err = closeW()
	if err != nil {
		t.Fatal(err)
	}
	// All methods should fail after close was called
	_, err = w.ReadAt([]byte{}, 0)
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}
	_, err = w.Size()
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}
	_, err = w.Write([]byte{})
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}
	err = w.Sync()
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}
	_, err = w.SizeLeft()
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}

	// Same applies for the read block
	r, closeE, err := NewFileReadBlock("test/f.txt")
	if err != nil {
		t.Fatal(err)
	}
	closeE()
	_, err = r.ReadAt([]byte{}, 0)
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}
	_, err = r.Size()
	if err != errBlockClosed {
		t.Fatal("expected blockClosedErr")
	}
}
