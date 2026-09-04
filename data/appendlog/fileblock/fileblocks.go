package fileblock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type readBlock struct {
	f        *os.File
	isClosed bool
}

func newRead(filePath string) (*readBlock, func(), error) {
	absPath := filepath.Clean(filePath)
	if !filepath.IsAbs(absPath) {
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, func() {}, fmt.Errorf("failed to get wd: %s", err)
		}
		absPath = filepath.Join(
			currentDir,
			filePath,
		)
	}
	f, err := os.OpenFile(absPath, os.O_RDONLY, 0644)
	if errors.Is(err, os.ErrNotExist) {
		return nil, func() {}, fmt.Errorf("%s not found", filePath)
	}
	if err != nil {
		return nil, func() {}, fmt.Errorf("failed to open %s: %w", filePath, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, func() {}, fmt.Errorf("failed to stat %s: %w", filePath, err)
	}
	if info.IsDir() {
		_ = f.Close()
		return nil, func() {}, fmt.Errorf("%s is a directory", filePath)
	}
	b := &readBlock{
		f: f,
	}
	return b, b.close, err
}
func (b *readBlock) close() {
	if b.isClosed {
		return
	}
	b.f.Close()
	b.isClosed = true
}
func (b *readBlock) ReadAt(p []byte, off int64) (int, error) {
	if b.isClosed {
		return 0, errBlockClosed
	}
	return b.f.ReadAt(p, off)
}
func (b *readBlock) Size() (int64, error) {
	if b.isClosed {
		return 0, errBlockClosed
	}
	info, err := b.f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

type writeBlock struct {
	absPath string
	mode    int
	f       *os.File // either nil or in good shape to be used
	fSize   int64    // always set correctly if f!=nil

	maxSize  int64
	lock     *sync.Mutex
	isClosed bool
	closeErr error
}

func newWrite(filePath string, maxFileSize int64, truncate bool) (w *writeBlock, close func() error, err error) {
	noOpClose := func() error { return nil }
	if maxFileSize < 0 {
		return nil, close, errors.New("maxFileSize must be >=0")
	}

	absPath := filepath.Clean(filePath)
	if !filepath.IsAbs(absPath) {
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, noOpClose, fmt.Errorf("failed to get wd: %s", err)
		}
		absPath = filepath.Join(
			currentDir,
			filePath,
		)
	}

	// Make sure the directories exist
	err = os.MkdirAll(filepath.Dir(absPath), os.ModePerm)
	if err != nil {
		return nil, noOpClose, fmt.Errorf("failed to mkdir: %s", err)
	}

	mode := os.O_RDWR | os.O_CREATE | os.O_APPEND
	if truncate {
		mode = os.O_RDWR | os.O_CREATE | os.O_APPEND | os.O_TRUNC
	}
	b := &writeBlock{
		mode:    mode,
		absPath: absPath,
		maxSize: maxFileSize,
		lock:    &sync.Mutex{},
	}
	return b, b.Close, b.selfHeal()
}

// Close the file to require healing
func (b *writeBlock) selfPoison() {
	_ = b.f.Close()
	b.f = nil
}

// Ensures the write block is ready to be used.
// Doesnt lock the mutex (lock it before calling this).
// Should be called before ALL methods except Close.
func (b *writeBlock) selfHeal() error {
	// Once closed, can't be reopened
	if b.isClosed {
		return nil
	}
	// Whenever the file is non nil, we know it's healthy
	if b.f != nil {
		return nil
	}

	f, err := os.OpenFile(b.absPath, b.mode, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", b.absPath, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to stat %s: %w", b.absPath, err)
	}
	if info.IsDir() {
		_ = f.Close()
		return fmt.Errorf(
			"%s is a directory and cant be used as a file",
			b.absPath)
	}
	if info.Size() > b.maxSize {
		_ = f.Close()
		return fmt.Errorf(
			"%d maxFileSize is too small: file %s has %d size",
			b.maxSize, b.absPath, info.Size())
	}
	b.f = f
	b.fSize = info.Size()
	return nil
}

func (b *writeBlock) Close() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.isClosed {
		return b.closeErr
	}
	b.isClosed = true
	if b.f == nil {
		b.closeErr = nil
	} else {
		b.closeErr = b.f.Close()
		b.f = nil
	}
	return b.closeErr
}

func (b *writeBlock) Trucate() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	err := b.selfHeal()
	if err != nil {
		return err
	}
	err = b.f.Truncate(0)
	if err != nil {
		b.selfPoison()
		return err
	}
	_, err = b.f.Seek(0, 0)
	if err != nil {
		b.selfPoison()
		return err
	}
	b.fSize = 0
	return nil
}

func (b *writeBlock) ReadAt(p []byte, off int64) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	err := b.selfHeal()
	if err != nil {
		return 0, err
	}
	if b.isClosed {
		return 0, errBlockClosed
	}
	return b.f.ReadAt(p, off)
}
func (b *writeBlock) Size() (int64, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	err := b.selfHeal()
	if err != nil {
		return 0, err
	}
	if b.isClosed {
		return 0, errBlockClosed
	}
	return b.fSize, nil
}
func (b *writeBlock) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	err := b.selfHeal()
	if err != nil {
		return 0, err
	}
	if b.isClosed {
		return 0, errBlockClosed
	}
	if b.fSize+int64(len(p)) > b.maxSize {
		return 0, errors.New("no space left")
	}
	n, err := b.f.Write(p)
	b.fSize += int64(n)
	if err != nil {
		// If a write fails, the file should be reopened
		b.selfPoison()
	}
	return n, err
}
func (b *writeBlock) SizeLeft() (int64, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	err := b.selfHeal()
	if err != nil {
		return 0, err
	}
	if b.isClosed {
		return 0, errBlockClosed
	}
	return b.maxSize - b.fSize, nil
}
func (b *writeBlock) Sync() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	err := b.selfHeal()
	if err != nil {
		return err
	}
	if b.isClosed {
		return errBlockClosed
	}
	err = b.f.Sync()
	if err != nil {
		// If sync fails, the file should be reopened
		b.selfPoison()
	}
	return err
}

var errBlockClosed = errors.New("block already closed")