package tiered

import (
	"io"
	"testing"
)

// Provides blocks using a tiered storage:
// Writes are added to a buffer file on disk.
// When it's full, its uploaded to an object store.
type Provider struct{ p *provider }

// Returns a new Storage instance. The BlobStorage is optional. If used,
// Data will spill to it to reduce disk usage. blockSize determines the size of
// blocks saved on disk or on BlobStorage (the same value is used in both).
// The entries from the blobstorage are always first read to disk.
// blobStorageCacheCapacity determines the number of entries that are downloaded
// (use any value if bs=nil).
func NewProvider(directory string, name string, blockSize int64, bs BlobStorage,
	blobStorageCacheCapacity int) (Provider, func() error, error) {
	p, closeF, err := newProvider(directory, name, blockSize, bs, blobStorageCacheCapacity)
	if err != nil {
		return Provider{}, closeF, err
	}
	return Provider{p}, closeF, nil
}

// Returns a self-cleaning Storage
func NewTestProvider(blockSize int64, name string, t *testing.T) Provider {
	return newTestProvider(blockSize, name, false, t)
}

// Returns a self-cleaning Storage
func NewTestProviderWithBlobStorage(blockSize int64, name string, t *testing.T) Provider {
	return newTestProvider(blockSize, name, true, t)
}

func (s Provider) Name() string                                           { return s.p.Name() }
func (s Provider) GetWrite() WriteBlock                                   { return s.p.GetWrite() }
func (s Provider) BlockSize() int64                                       { return s.p.BlockSize() }
func (s Provider) Size() (int64, error)                                   { return s.p.Size() }
func (s Provider) GetRead(i int64) (r ReadBlock, close func(), err error) { return s.p.GetRead(i) }
func (s Provider) Blocks() int64                                          { return s.p.Blocks() }
func (s Provider) Expand() error                                          { return s.p.Expand() }

type WriteBlock interface {
	io.Writer
	io.ReaderAt
	Sync() error
	Size() (int64, error)
	SizeLeft() (int64, error)
	Trucate() error
}

type ReadBlock interface {
	io.ReaderAt
	Size() (int64, error)
}

type BlobStorage interface {
	Put(keyPrefix, key string, size int64, r io.Reader) error
	Get(keyPrefix, key string, offset int64) (r io.Reader, closeR func(), err error)
}
