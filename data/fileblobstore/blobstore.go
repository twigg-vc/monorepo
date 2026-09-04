package fileblobstore

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type fileBlobStorage struct {
	absDirPath string
	lock       *sync.RWMutex
}

func newFileBlobStorage(directory string) (fileBlobStorage, error) {
	if runtime.GOOS == "windows" {
		// Windows is a piece of shit and doesn't guarantee "Rename" atomicity,
		// which this package assumes.
		panic("tiered storage not fully supported on windows")
	}
	absDirPath := filepath.Clean(directory)
	if !filepath.IsAbs(directory) {
		currentDir, err := os.Getwd()
		if err != nil {
			return fileBlobStorage{}, fmt.Errorf("failed to get wd: %s", err)
		}
		absDirPath = filepath.Join(
			currentDir,
			directory,
		)
	}
	err := os.MkdirAll(absDirPath, os.ModePerm)
	if err != nil {
		return fileBlobStorage{}, fmt.Errorf("failed to mkdir: %s", err)
	}

	return fileBlobStorage{
		absDirPath: absDirPath,
		lock:       &sync.RWMutex{},
	}, nil
}

func (b *fileBlobStorage) put(keyPrefix, key string, size int64, r io.Reader) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	// Create a temp file and write to it
	fileAbsPath := filepath.Join(b.absDirPath, encodeKey(keyPrefix, key))
	tmpFile, err := os.CreateTemp(b.absDirPath, "*_tmp")
	if err != nil {
		return fmt.Errorf("failed to create tmp file: %w", err)
	}
	closeTmpFileFunc := tmpFile.Close
	defer func() {
		closeTmpFileFunc()
		os.Remove(tmpFile.Name())
	}()
	_, err = io.Copy(tmpFile, io.LimitReader(r, size))
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}
	err = tmpFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync tmp file: %w", err)
	}
	err = tmpFile.Close()
	closeTmpFileFunc = func() error { return nil } // to avoid calling again
	if err != nil {
		return fmt.Errorf("failed to close tmp file: %w", err)
	}
	// Create a new one or replace the old one atomically.
	// Note that this assumes Rename to be atomic.
	err = os.Rename(tmpFile.Name(), fileAbsPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	return nil
}
func (b *fileBlobStorage) get(keyPrefix, key string, offset int64) (r io.Reader, closeR func(), err error) {
	b.lock.RLock()
	f, err := os.Open(filepath.Join(b.absDirPath, encodeKey(keyPrefix, key)))
	if err != nil {
		b.lock.RUnlock()
		r = nil
		closeR = func() {}
		return
	}
	_, err = f.Seek(offset, 0)
	if err != nil {
		b.lock.RUnlock()
		r = nil
		closeR = func() {}
		return
	}
	r = f
	closeOnce := &closeOnce{
		isClosed: false,
		closeFunc: func() {
			f.Close()
			b.lock.RUnlock()
		},
	}
	closeR = func() {
		closeOnce.close()
	}
	err = nil
	return
}

func (b *fileBlobStorage) count() (int, error) {
	b.lock.RLock()
	defer b.lock.RLock()
	entries, err := os.ReadDir(b.absDirPath)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), encodedKeyPrefix) {
			n += 1
		}
	}
	return n, nil
}

const encodedKeyPrefix = "blob_"

func encodeKey(keyPrefix, key string) string {
	return encodedKeyPrefix + base64.RawURLEncoding.EncodeToString([]byte(keyPrefix+key))
}

func newTestFileBlobStorage(nameSuffix string, t *testing.T) FileBlobStorage {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}
	dirAbsPath := filepath.Join(wd, encodeKey("", t.Name()+"_blobs"+nameSuffix))
	os.RemoveAll(dirAbsPath)
	t.Cleanup(func() {
		os.RemoveAll(dirAbsPath)
	})
	bs, err := NewFileBlobStorage(dirAbsPath)
	if err != nil {
		t.Fatalf("failed to create blob storage: %s", err)
	}
	return bs
}

// Simple struct to allow closes to be called multiple times without any effect
type closeOnce struct {
	isClosed  bool
	closeFunc func()
}

func (c *closeOnce) close() {
	if c.isClosed {
		return
	}
	c.isClosed = true
	c.closeFunc()
}
