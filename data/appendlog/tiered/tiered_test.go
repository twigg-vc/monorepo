package tiered

import (
	"bytes"
	"monorepo/data/fileblobstore"
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

func TestNewProvider(t *testing.T) {
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	const blobsOnDisk = 1
	p, closeP, err := NewProvider(
		filepath.Join("test", t.Name()), "test", 5, blobs, blobsOnDisk)
	defer closeP()
	if err != nil {
		t.Fatal(err)
	}
	if p.BlockSize() != 5 {
		t.Fatal("unexpected block size")
	}
	if p.Blocks() != 1 {
		t.Fatal("unexpected number of blocks")
	}

	w := p.GetWrite()
	n, err := w.Write([]byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("expected to write 3")
	}

	// Close and reopen to verify durability
	closeP()
	p, closeP, err = NewProvider(
		filepath.Join("test", t.Name()), "test", 5, blobs, blobsOnDisk)
	defer closeP()
	if err != nil {
		t.Fatal(err)
	}
	w = p.GetWrite()
	b := make([]byte, 2)
	w.ReadAt(b, 1)
	if !bytes.Equal(b, []byte("bc")) {
		t.Fatalf("invalid read: %s", b)
	}
}

func TestSize(t *testing.T) {
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	blobCacheCapacity := 1
	p, closeP, _ := NewProvider(
		filepath.Join("test", t.Name()), "test", 5, blobs, blobCacheCapacity)
	defer closeP()
	s, err := p.Size()
	if err != nil {
		t.Fatal(err)
	}
	if s != 0 {
		t.Fatalf("wrong size: %d", s)
	}

	w := p.GetWrite()
	w.Write([]byte("abc"))
	s, err = p.Size()
	if err != nil {
		t.Fatal(err)
	}
	if s != 3 {
		t.Fatalf("wrong size: %d", s)
	}

	// Close and reopen to verify durability
	closeP()
	p, closeP, _ = NewProvider(
		filepath.Join("test", t.Name()), "test", 5, blobs, blobCacheCapacity)
	defer closeP()
	s, err = p.Size()
	if err != nil {
		t.Fatal(err)
	}
	if s != 3 {
		t.Fatalf("wrong size: %d", s)
	}
}

func TestCantChangeBlockSize(t *testing.T) {
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	blobCacheCapacity := 1
	_, closeP, _ := NewProvider(
		filepath.Join("test", t.Name()), "test", 5, blobs, blobCacheCapacity)
	closeP()

	_, _, err := NewProvider(
		filepath.Join("test", t.Name()), "test", 6, blobs, blobCacheCapacity)
	if err == nil {
		t.Fatal("expected err when changing block size")
	}
}

func TestSimpleGetRead(t *testing.T) {
	p := NewTestProvider(5, "test", t)

	w := p.GetWrite()
	w.Write([]byte("01234"))

	r, closeR, err := p.GetRead(0)
	if err != nil {
		t.Fatal(err)
	}
	defer closeR()

	b := make([]byte, 3)
	r.ReadAt(b, 2)
	if !bytes.Equal(b, []byte("234")) {
		t.Fatal("unexpected read")
	}

	closeR()
	_, _, err = p.GetRead(1)
	if err == nil {
		t.Fatal("expected err for too large index")
	}
	_, _, err = p.GetRead(-1)
	if err == nil {
		t.Fatal("expected err for too small index")
	}
}

func TestExpand(t *testing.T) {
	p := NewTestProviderWithBlobStorage(5, "test", t)

	w := p.GetWrite()
	w.Write([]byte("01234"))

	err := p.Expand()
	if err != nil {
		t.Fatal(err)
	}

	w = p.GetWrite()
	_, err = w.Write([]byte("56789"))
	if err != nil {
		t.Fatal(err)
	}

	s, err := p.Size()
	if err != nil {
		t.Fatal(err)
	}
	if s != 10 {
		t.Fatalf("got size %d", s)
	}

	// Check the read blocks
	r, closeR, err := p.GetRead(0)
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 5)
	r.ReadAt(b, 0)
	if !bytes.Equal(b, []byte("01234")) {
		t.Fatal("wrong read 0")
	}
	closeR()
	r, closeR, err = p.GetRead(1)
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	r.ReadAt(b, 0)
	if !bytes.Equal(b, []byte("56789")) {
		t.Fatal("wrong read 1")
	}
}

func TestInstancesWithDifferentName(t *testing.T) {
	p1 := NewTestProviderWithBlobStorage(5, "test1", t)
	p2 := NewTestProviderWithBlobStorage(5, "test2", t)

	p1.GetWrite().Write([]byte("01234"))
	s1, err := p1.Size()
	if err != nil {
		t.Fatal(err)
	}
	if s1 == 0 {
		t.Fatal("got zero size")
	}

	s2, err := p2.Size()
	if err != nil {
		t.Fatal(err)
	}
	if s2 != 0 {
		t.Fatalf("got %d", s2)
	}
}

func TestSmallBlobCacheCapacity(t *testing.T) {
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	const blobCacheCapacity = 1
	p, closeP, err := NewProvider(
		filepath.Join("test", t.Name()), "test", 3, blobs, blobCacheCapacity)
	defer closeP()
	if err != nil {
		t.Fatal(err)
	}

	// Write many blocks to verify that downloading the blobs works
	_, err = p.GetWrite().Write([]byte("123"))
	if err != nil {
		t.Fatal(err)
	}
	err = p.Expand()
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.GetWrite().Write([]byte("456"))
	if err != nil {
		t.Fatal(err)
	}
	err = p.Expand()
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.GetWrite().Write([]byte("789"))
	if err != nil {
		t.Fatal(err)
	}

	// Read blocks one by one
	b := make([]byte, 3)
	r, closeR, err := p.GetRead(0)
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	r.ReadAt(b, 0)
	if !bytes.Equal(b, []byte("123")) {
		t.Fatal("wrong read 0")
	}
	closeR()

	r, closeR, err = p.GetRead(1)
	if err != nil {
		t.Fatal(err)
	}
	r.ReadAt(b, 0)
	closeR()
	if !bytes.Equal(b, []byte("456")) {
		t.Fatal("wrong read 1")
	}

	r, closeR, err = p.GetRead(2)
	if err != nil {
		t.Fatal(err)
	}
	r.ReadAt(b, 0)
	closeR()
	if !bytes.Equal(b, []byte("789")) {
		t.Fatal("wrong read 2")
	}
}

func TestMultipleReaders(t *testing.T) {
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	const blobCacheCapacity = 1
	p, closeP, err := NewProvider(
		filepath.Join("test", t.Name()), "test", 3, blobs, blobCacheCapacity)
	defer closeP()
	if err != nil {
		t.Fatal(err)
	}

	// Write data once and try reading from many readers
	_, err = p.GetWrite().Write([]byte("123"))
	if err != nil {
		t.Fatal(err)
	}
	r1, closeR1, err := p.GetRead(0)
	defer closeR1()
	if err != nil {
		t.Fatal(err)
	}
	r2, closeR2, err := p.GetRead(0)
	defer closeR2()
	if err != nil {
		t.Fatal(err)
	}

	b1 := make([]byte, 3)
	r1.ReadAt(b1, 0)
	if !bytes.Equal(b1, []byte("123")) {
		t.Fatal("wrong read r1")
	}
	b2 := make([]byte, 3)
	r2.ReadAt(b2, 0)
	if !bytes.Equal(b2, []byte("123")) {
		t.Fatal("wrong read r2")
	}

}
