package fileblobstore

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestFileBlobStorage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows not supported")
	}
	blobs := NewTestFileBlobStorage(t)

	prefixes := []string{"p0", "p1"}
	keyToValue := map[string]string{
		"key1": "abc",
		"key2": "def",
		"key3": "ghij",
	}
	for i := 1; i < 20; i++ {
		keyToValue[strconv.Itoa(i)] = strings.Repeat("a", i)
	}

	wg := sync.WaitGroup{}
	errCh := make(chan error, 2*len(keyToValue)+1)
	for _, pref := range prefixes {
		for key, val := range keyToValue {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := blobs.Put(pref, key, int64(len(pref+val)),
					bytes.NewBufferString(pref+val))
				if err != nil {
					errCh <- err
				}
			}()
		}

	}

	wg.Wait()
	n, err := blobs.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != len(prefixes)*len(keyToValue) {
		t.Fatalf("count=%d, expected %d", n, len(prefixes)*len(keyToValue))
	}

	for _, pref := range prefixes {
		for key, val := range keyToValue {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, closeR, err := blobs.Get(pref, key, 0)
				if err != nil {
					closeR()
					errCh <- err
				}
				b := bytes.NewBuffer(nil)
				io.Copy(b, r)
				closeR()
				if b.String() != pref+val {
					errCh <- fmt.Errorf("got %s:%s", key, val)
				}
			}()
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}
