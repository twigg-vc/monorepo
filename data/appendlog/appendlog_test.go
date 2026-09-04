package appendlog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"monorepo/data/appendlog/tiered"
	"sync"
	"testing"
	"time"
)

func getTestAppendLog(t *testing.T) AppendLog {
	bp := tiered.NewTestProvider(3, "ds0", t)
	return New(bp)
}

func TestWriteLessThanFileSize(t *testing.T) {
	ds := getTestAppendLog(t)

	n, err := ds.Write([]byte("12"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatal("should write 2 bytes")
	}
	n, err = ds.Write([]byte("3"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("should write 1 bytes")
	}
	if s, _ := ds.Size(); s != 3 {
		t.Fatal("wrong size")
	}
}

func TestWriteFileSize(t *testing.T) {
	ds := getTestAppendLog(t)

	n, err := ds.Write([]byte("123"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("should write 3 bytes")
	}
}

func TestWriteMoreThanFileSizeInTotal(t *testing.T) {
	ds := getTestAppendLog(t)

	n, err := ds.Write([]byte("123"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("should write 3 bytes")
	}
	n, err = ds.Write([]byte("4"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("should write 1 bytes")
	}
}

func TestWriteMoreThanFileSizeAtOnce(t *testing.T) {
	ds := getTestAppendLog(t)

	n, err := ds.Write([]byte("1234"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatal("should write 4 bytes")
	}
}

func TestWriteMoreThanFileSizeTwice(t *testing.T) {
	ds := getTestAppendLog(t)

	n, err := ds.Write([]byte("1234"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatal("should write 4 bytes")
	}
	n, err = ds.Write([]byte("5678"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatal("should write 4 bytes")
	}
}

func TestEOF(t *testing.T) {
	ds := getTestAppendLog(t)

	b := make([]byte, 1)
	n, err := ds.ReadAt(b, 0)
	if n != 0 {
		t.Fatal("should read 0")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatal("should EOF because zero bytes were read")
	}
}

func TestUnexpectedEOF(t *testing.T) {
	ds := getTestAppendLog(t)

	ds.Write([]byte("123"))
	b := make([]byte, 4)
	n, err := ds.ReadAt(b, 0)
	if n != 3 {
		t.Fatal("should read 3")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("should unexpected EOF bc some bytes were read before EOF")
	}
}

func TestReadTwoChunks(t *testing.T) {
	ds := getTestAppendLog(t)

	ds.Write([]byte("012"))
	ds.Write([]byte("3"))
	ds.Write([]byte("4"))
	ds.Write([]byte("56"))
	b := make([]byte, 5)
	n, err := ds.ReadAt(b, 0)
	if n != 5 {
		t.Fatal("should read 5")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte("01234")) {
		t.Fatal("wrong bytes read")
	}
	b = make([]byte, 2)
	n, err = ds.ReadAt(b, 4)
	if n != 2 {
		t.Fatal("should read 2")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte("45")) {
		t.Fatal("wrong bytes read")
	}
}

func TestManyOneByOne(t *testing.T) {
	ds := getTestAppendLog(t)

	wrote := []byte("01234567891234")
	ds.Write(wrote)
	b := make([]byte, 1)
	for i := 0; i < len(wrote); i++ {
		ds.ReadAt(b, int64(i))
		if b[0] != wrote[i] {
			t.Fatal("wrong read")
		}
	}
}

func TestReadConcurency(t *testing.T) {
	ds := getTestAppendLog(t)

	wrote := []byte("01234567891234")
	ds.Write(wrote)

	testReading := func(wg *sync.WaitGroup) {
		defer wg.Done()
		for i := 0; i < len(wrote); i++ {
			b := make([]byte, 1)
			ds.ReadAt(b, int64(i))
			if b[0] != wrote[i] {
				panic(
					fmt.Sprintf("wrong read. Expected %d got %d",
						b[0], wrote[i]),
				)
			}
		}
	}

	var wg sync.WaitGroup
	// Start many reader in parallel
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go testReading(&wg)
	}
	wg.Wait()
}

func FuzzWriteAndRead(f *testing.F) {

	// FileSize = 5
	// Write 2 times, each time 3 bytes: [0,1,2], [3,4,5]
	// Read with buffers of size 2: [0,1], [2,3], [4,5]
	f.Add(int64(5), 2, int64(3), int64(2))

	// FileSize = 6
	// Write 2 times, each time 3 bytes: [0,1,2], [3,4,5]
	// Read with buffers of size 4: [0,1,2,3], [4,5, , ]
	f.Add(int64(6), 2, int64(3), int64(4))

	f.Fuzz(func(t *testing.T, maxFileSize int64, nWrites int, writeSize, readSize int64) {
		if maxFileSize <= 0 || nWrites < 0 || writeSize < 0 || readSize < 0 {
			t.Skip()
			return
		}
		// Keep maxFileSize smaller than 1kB
		if maxFileSize > 1_000 || writeSize > 1_000 || readSize > 1_000 {
			t.Skip()
			return
		}
		// Keep maxFileSize > 0
		if maxFileSize <= 0 {
			t.Skip()
			return
		}
		// Note: we need to take extra care to no open the same file twice.
		// Ideally we'd implement some form of file locking with the OS but
		// that's not straightforward. This means that other processes can
		// access the file at the same time and that could cause prolems,
		// that's why we should check writes and reads hashes to make sure
		// they succeeded. This problem is not really a big problem in
		// this DB bc it's append only, which means even simultaneous access
		// can't corrupt past data from the DB :)
		bp := tiered.NewTestProvider(
			maxFileSize,
			fmt.Sprintf("test_%d_%d_%d_%d_%d",
				maxFileSize, nWrites, writeSize, readSize,
				time.Now().Nanosecond()),
			t)
		ds := New(bp)

		// This buffer will be our source of truth for what's expected.
		// After-all, we want the Datastrip to behave exactly like a buffer,
		// but backed by files.
		buff := bytes.NewBuffer(nil)

		// Write 0,1,2,3, ... in chunks of size writeSize
		writtenToBuff := int64(0)
		writtenToDs := int64(0)
		valToWrite := 0
		for i := 0; i < nWrites; i++ {
			w := make([]byte, writeSize)
			for j := 0; j < int(writeSize); j++ {
				w[j] = byte(j)
				valToWrite++
			}
			n, err := buff.Write(w)
			if err != nil {
				t.Fatal(err)
			}
			writtenToBuff += int64(n)
			n, err = ds.Write(w)
			if err != nil {
				t.Fatal(err)
			}
			writtenToDs += int64(n)
		}
		if writtenToBuff != writtenToDs {
			t.Fatal("mismatch between n written to buff and to Datastrip")
		}

		if writtenToBuff != int64(nWrites)*writeSize {
			t.Fatalf("wrong total amount written. got:%v\nexpected:%v", writtenToBuff, int64(nWrites)*writeSize)
		}
		s, err := ds.Size()
		if err != nil {
			t.Fatalf("unexpected error geting size: %s", err)
		}
		if s != writtenToBuff {
			t.Fatalf("wrong size. got: %d\nexpected: %d", s, writtenToBuff)
		}

		// Read all the data chunk by chunk
		offset := 0
		buffReader := bytes.NewReader(buff.Bytes())
		for offset <= int(writtenToBuff) {
			buffR := make([]byte, readSize)
			buffN, err := buffReader.ReadAt(buffR, int64(offset))
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatal(err)
			}
			dsR := make([]byte, readSize)
			dsN, err := ds.ReadAt(dsR, int64(offset))
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatal(err)
			}
			if buffN != dsN {
				t.Fatalf("mismatch between n read from buff and Datastrip. got: %v\nexpected: %v", dsN, buffN)
			}
			if !bytes.Equal(dsR, buffR) {
				t.Fatalf("mismatch between bytes read from buff and Datastrip. got: %v\nexpected: %v", dsR, buffR)
			}
			offset += dsN

			if dsN == 0 {
				if !errors.Is(err, io.EOF) && readSize > 0 {
					t.Fatal("read zero but got no EOF")
				}
				break
			}
		}

	})
}
