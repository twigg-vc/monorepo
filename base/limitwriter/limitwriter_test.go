package limitwriter

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriteLessThanLimit(t *testing.T) {
	w := bytes.NewBuffer(nil)
	l := New(w, 6)

	n, err := l.Write([]byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("wrote %d", n)
	}
	n, err = l.Write([]byte{4, 5})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("wrote %d", n)
	}
	n, err = l.Write([]byte{6})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("wrote %d", n)
	}
}

func TestWriteMoreThanLimitOnce(t *testing.T) {
	w := bytes.NewBuffer(nil)
	l := New(w, 3)

	n, err := l.Write([]byte{1, 2, 3, 4})
	if !errors.Is(err, ErrNotEnoughQuota) {
		t.Fatal("expected not enough quota")
	}
	if n != 3 {
		t.Fatalf("wrote %d", n)
	}
	n, err = l.Write([]byte{5})
	if !errors.Is(err, ErrNotEnoughQuota) {
		t.Fatal("expected not enough quota")
	}
	if n != 0 {
		t.Fatalf("wrote %d", n)
	}
}

func TestWriteMoreThanLimitInSecondTry(t *testing.T) {
	w := bytes.NewBuffer(nil)
	l := New(w, 3)

	n, err := l.Write([]byte{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("wrote %d", n)
	}

	n, err = l.Write([]byte{3, 4})
	if !errors.Is(err, ErrNotEnoughQuota) {
		t.Fatal("expected not enough quota")
	}
	if n != 1 {
		t.Fatalf("wrote %d", n)
	}
	n, err = l.Write([]byte{5})
	if !errors.Is(err, ErrNotEnoughQuota) {
		t.Fatal("expected not enough quota")
	}
	if n != 0 {
		t.Fatalf("wrote %d", n)
	}
}

func FuzzLimitWriter(f *testing.F) {
	f.Add(int64(10), []byte("hello world")) // seed input

	f.Fuzz(func(t *testing.T, limit int64, data []byte) {
		if limit < 0 {
			t.Skip() // invalid quota
		}

		var buf bytes.Buffer
		lw := New(&buf, limit)

		n, err := lw.Write(data)

		// Case 1: when data fits entirely
		if int64(len(data)) <= limit {
			if err != nil {
				t.Fatalf("unexpected error for len=%d, limit=%d: %v", len(data), limit, err)
			}
			if n != len(data) {
				t.Fatalf("expected n=%d, got %d", len(data), n)
			}
			if buf.Len() != len(data) {
				t.Fatalf("expected buf len=%d, got %d", len(data), buf.Len())
			}
			if buf.String() != string(data) {
				t.Fatalf("unexpected buffer contents: %q vs %q", buf.String(), data)
			}
		} else {
			// Case 2: when data exceeds the quota
			if !errors.Is(err, ErrNotEnoughQuota) {
				t.Fatalf("expected ErrNotEnoughQuota, got %v", err)
			}
			if int64(n) != limit {
				t.Fatalf("expected to write %d bytes, wrote %d", limit, n)
			}
			if int64(buf.Len()) != limit {
				t.Fatalf("expected buf len=%d, got %d", limit, buf.Len())
			}

			// All subsequent writes should fail with ErrNotEnoughQuota
			_, err2 := lw.Write([]byte("x"))
			if limit > 0 && !errors.Is(err2, ErrNotEnoughQuota) {
				t.Fatalf("expected ErrNotEnoughQuota after limit reached, got %v", err2)
			}
		}

	})
}
