package fileblobstore

import (
	"io"
	"testing"
)

// Stores blobs as files on disk. One file per blob.
type FileBlobStorage struct{ s *fileBlobStorage }

// Returns a blob storage that stores blobs as files on disk
func NewFileBlobStorage(directory string) (FileBlobStorage, error) {
	s, err := newFileBlobStorage(directory)
	if err != nil {
		return FileBlobStorage{}, err
	}
	return FileBlobStorage{s: &s}, nil
}

// Returns a self-cleaning blob storage for tests
func NewTestFileBlobStorage(t *testing.T) FileBlobStorage { return newTestFileBlobStorage("", t) }

// Returns a self-cleaning blob storage for tests
func NewTestBlobStorageWithName(nameSuffix string, t *testing.T) FileBlobStorage {
	return newTestFileBlobStorage(nameSuffix, t)
}

// Saves an entry
func (b FileBlobStorage) Put(keyPrefix, key string, size int64, r io.Reader) error {
	return b.s.put(keyPrefix, key, size, r)
}

// Returns an entry
func (b FileBlobStorage) Get(keyPrefix, key string, offset int64) (r io.Reader, closeR func(), err error) {
	return b.s.get(keyPrefix, key, offset)
}

// Returns the number of entries saved
func (b FileBlobStorage) Count() (int, error) {
	return b.s.count()
}
