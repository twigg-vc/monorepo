package tiered

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"monorepo/base/cache"
	"monorepo/data/appendlog/fileblock"
	"monorepo/data/fileblobstore"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"database/sql"

	_ "modernc.org/sqlite" // register the driver
)

type provider struct {
	absDirPath       string
	name             string
	buff             WriteBlock
	buffRead         ReadBlock
	closeBuff        func() error
	latestBlockIndex int64
	blockSize        int64
	bs               BlobStorage
	bsBlocksOnDisk   cache.LRU[string, bool]
	db               *sql.DB
	lock             *sync.Mutex
	isClosed         bool
}

func newProvider(
	directory string, name string, blockSize int64, bs BlobStorage,
	blobStorageCacheCapacity int) (*provider, func() error, error) {
	if bs == nil {
		blobStorageCacheCapacity = 1
	}
	if blobStorageCacheCapacity <= 0 {
		panic(fmt.Sprintf("blobStorageCacheCapacity %d is invalid",
			blobStorageCacheCapacity))
	}
	noOpClose := func() error { return nil }
	if name == "" {
		return nil, noOpClose, errors.New("name cant be empty")
	}
	if !isSafeFilename(name) {
		return nil, noOpClose, fmt.Errorf("%s is a bad name", name)
	}
	absDirPath := filepath.Clean(directory)
	if !filepath.IsAbs(directory) {
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, noOpClose, fmt.Errorf("failed to get wd: %s", err)
		}
		absDirPath = filepath.Join(
			currentDir,
			directory,
		)
	}
	dbFileName := fmt.Sprintf("%s-blocks.db", name)
	absPathToDbFile := filepath.Join(absDirPath, dbFileName)
	err := os.MkdirAll(filepath.Dir(absPathToDbFile), os.ModePerm)
	if err != nil {
		return nil, noOpClose, fmt.Errorf(
			"failed to mkdir to db file: %s", err)
	}
	db, err := sql.Open(
		"sqlite",
		fmt.Sprintf("file:%s?_pragma=journal_mode=WAL&_pragma=synchronous=FULL",
			absPathToDbFile))
	if err != nil {
		return nil, noOpClose, fmt.Errorf("failed to initialize db: %s", err)
	}

	const dbSetupQuery = `
	BEGIN;
	CREATE TABLE IF NOT EXISTS blocks (
		BlockIndex INTEGER NOT NULL,
		BlockSize  INTEGER NOT NULL,
		IsUploaded BOOLEAN NOT NULL,
		IsLatest   BOOLEAN NOT NULL
	);
	CREATE INDEX IF NOT EXISTS blocks_by_idx
	ON blocks (BlockIndex);
	CREATE INDEX IF NOT EXISTS blocks_by_idx_islatest
	ON blocks (BlockIndex, IsLatest);
	COMMIT;
	`
	_, err = db.Exec(dbSetupQuery)
	if err != nil {
		db.Close()
		return nil, noOpClose,
			fmt.Errorf("failed to initialize db table: %s", err)
	}
	const getLatestBlockQuery = `
	SELECT
		BlockIndex,
		BlockSize
	FROM blocks
	WHERE IsLatest = TRUE
	`
	var latestBlockIndex int64
	var latestBlockSize int64
	err = db.QueryRow(getLatestBlockQuery).Scan(
		&latestBlockIndex,
		&latestBlockSize,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		db.Close()
		return nil, noOpClose,
			fmt.Errorf("failed to query latest db block: %s", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = db.Exec(`
		INSERT INTO blocks
			(BlockIndex, BlockSize, IsUploaded, IsLatest)
		VALUES
			(0, ?, FALSE, TRUE);
		`, blockSize)
		if err != nil {
			db.Close()
			return nil, noOpClose,
				fmt.Errorf("failed to insert first block: %s", err)
		}
		latestBlockIndex = 0
		latestBlockSize = blockSize
	}
	if latestBlockSize != blockSize {
		db.Close()
		return nil, noOpClose,
			fmt.Errorf("cant change blocksize from %d to %d",
				latestBlockSize, blockSize)
	}
	bufferBlock, closeBufferBlock, err := fileblock.NewFileWriteBlock(
		blockFileAbsPath(absDirPath, name, latestBlockIndex), blockSize,
		/*truncate=*/ false)
	if err != nil {
		db.Close()
		closeBufferBlock()
		return nil, noOpClose,
			fmt.Errorf("failed to get write block: %s", err)
	}
	bufferBlockR, closeBufferBlockR, err := fileblock.NewFileReadBlock(
		blockFileAbsPath(absDirPath, name, latestBlockIndex))
	if err != nil {
		db.Close()
		closeBufferBlock()
		closeBufferBlockR()
		return nil, noOpClose,
			fmt.Errorf("failed to get write-r block: %s", err)
	}
	closeBuff := func() error {
		closeBufferBlockR()
		return closeBufferBlock()
	}
	p := &provider{
		absDirPath:       absDirPath,
		name:             name,
		buff:             bufferBlock,
		buffRead:         bufferBlockR,
		closeBuff:        closeBuff,
		latestBlockIndex: latestBlockIndex,
		blockSize:        latestBlockSize,
		bs:               bs,
		db:               db,
		lock:             &sync.Mutex{},
		bsBlocksOnDisk:   cache.New[string, bool](blobStorageCacheCapacity),
	}
	return p, p.close, nil
}

func (p *provider) close() error {
	if p.isClosed {
		return nil
	}
	p.isClosed = true
	return errors.Join(p.closeBuff(), p.db.Close())
}
func (p provider) Name() string {
	return p.name
}
func (p provider) GetWrite() WriteBlock {
	return p.buff
}
func (p provider) BlockSize() int64 {
	return p.blockSize
}
func (p provider) Blocks() int64 {
	return p.latestBlockIndex + 1
}
func (p provider) Size() (int64, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	writeBlockSize, err := p.buff.Size()
	if err != nil {
		return 0, fmt.Errorf("failed to read write block size: %w", err)
	}
	nReadBlocks := p.latestBlockIndex
	return nReadBlocks*p.blockSize + writeBlockSize, nil
}
func (p provider) GetRead(i int64) (br ReadBlock, closeBr func(), err error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if i < 0 || i > p.latestBlockIndex {
		br = nil
		closeBr = func() {}
		err = fmt.Errorf("block %d not found", i)
		return
	}

	// latest block read is always open
	if i == p.latestBlockIndex {
		br = p.buffRead
		closeBr = func() {}
		return
	}
	// If no blockstorage is used or we want the latest block,
	// just read the file from disk.
	if p.bs == nil {
		br, closeBr, err = fileblock.NewFileReadBlock(
			blockFileAbsPath(p.absDirPath, p.name, i))
		return
	}
	// Else, we must read it from disk. If its not on disk yet, download first
	blockCachePath := blockCacheFileAbsPath(p.absDirPath, p.name, i)
	_, found := p.bsBlocksOnDisk.Get(blockCachePath)
	if !found {
		var blobR io.Reader
		var closeBlobR func()
		blobR, closeBlobR, err = p.bs.Get(p.name, blockFileName(p.name, i), 0)
		if closeBlobR == nil {
			closeBlobR = func() {}
		}
		if err != nil {
			closeBlobR()
			br = nil
			closeBr = func() {}
			return
		}

		var fW WriteBlock
		var closeFw func() error
		fW, closeFw, err = fileblock.NewFileWriteBlock(
			blockCachePath, p.blockSize, true)
		if closeFw == nil {
			closeFw = func() error { return nil }
		}
		if err != nil {
			closeBlobR()
			closeFw()
			br = nil
			closeBr = func() {}
			return
		}
		_, err = io.Copy(fW, blobR)
		closeBlobR()
		if err != nil {
			_ = closeFw()
			br = nil
			closeBr = func() {}
			return
		}
		err = fW.Sync()
		if err != nil {
			_ = closeFw()
			br = nil
			closeBr = func() {}
			return
		}
		err = closeFw()
		if err != nil {
			br = nil
			closeBr = func() {}
			return
		}
		p.bsBlocksOnDisk.Put(blockCachePath, true)
		p.bsBlocksOnDisk.AddOnRemoveCallback(blockCachePath, func(path string) {
			if !strings.Contains(path, "_cache_block_") {
				panic(fmt.Sprintf("tried to delete non cache block %s", path))
			}
			_ = os.Remove(path)
		})
	}

	br, closeBr, err = fileblock.NewFileReadBlock(blockCachePath)
	if err != nil {
		closeBr()
		br = nil
		closeBr = func() {}
		return
	}
	return
}

func (p *provider) Expand() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	w := p.GetWrite()
	sizeLeft, err := w.SizeLeft()
	if err != nil {
		return fmt.Errorf("failed to get size left: %s", err)
	}
	if sizeLeft != 0 {
		return fmt.Errorf("current write block is not yet full")
	}
	err = w.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync write block: %s", err)
	}

	// Get the new buffer
	newBuffer, closeNewBuffer, err := fileblock.NewFileWriteBlock(
		blockFileAbsPath(p.absDirPath, p.name, p.latestBlockIndex+1), p.blockSize,
		/*truncate=*/ false)
	if err != nil {
		closeNewBuffer()
		return fmt.Errorf("failed to get new buffer: %s", err)
	}
	newBufferR, closeNewBufferR, err := fileblock.NewFileReadBlock(
		blockFileAbsPath(p.absDirPath, p.name, p.latestBlockIndex+1))
	if err != nil {
		closeNewBufferR()
		closeNewBuffer()
		return fmt.Errorf("failed to get new buffer: %s", err)
	}

	// Upload old buffer to blockstorage if available
	if p.bs != nil {
		r := io.NewSectionReader(w, 0, p.blockSize)
		err = p.bs.Put(p.name, blockFileName(p.name, p.latestBlockIndex),
			p.blockSize, r)
		if err != nil {
			return fmt.Errorf("failed to put block in blockstore: %s", err)
		}
	}
	// Update the db
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %s", err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
		UPDATE blocks
		SET IsLatest = FALSE, IsUploaded = ?
		WHERE IsLatest = TRUE;
		`, p.bs != nil)
	if err != nil {
		return fmt.Errorf("failed to update latest block: %s", err)
	}
	_, err = tx.Exec(`
		INSERT INTO blocks
			(BlockIndex, BlockSize, IsUploaded, IsLatest)
		VALUES
			(?, ?, FALSE, TRUE);
		`, p.latestBlockIndex+1, p.blockSize)
	if err != nil {
		return fmt.Errorf("failed to insert new block: %s", err)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit block update: %s", err)
	}

	// Close the current buffer ignoring errors
	if p.bs == nil {
		_ = p.closeBuff()
	} else {
		// If BlobStore is used, we can truncate the file to save disk space
		_ = p.buff.Trucate()
		_ = p.closeBuff()
	}
	// Populate the new buffers
	p.buff = newBuffer
	p.buffRead = newBufferR
	p.closeBuff = func() error {
		closeNewBufferR()
		return closeNewBuffer()
	}
	p.latestBlockIndex += 1
	return nil
}

func blockFileAbsPath(absDir string, name string, i int64) string {
	return filepath.Join(absDir, blockFileName(name, i))
}
func blockFileName(name string, i int64) string {
	return fmt.Sprintf("%s_block_%d", name, i)
}
func blockCacheFileAbsPath(absDir string, name string, i int64) string {
	// If you change this, make sure to change a panic a couple lines above
	return filepath.Join(absDir, fmt.Sprintf("%s_cache_block_%d", name, i))
}

func newTestProvider(blockSize int64, name string,
	useBlobStorage bool, t *testing.T) Provider {
	var bs BlobStorage
	if useBlobStorage {
		bs = fileblobstore.NewTestFileBlobStorage(t)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %s", err)
	}
	dirAbsPath := filepath.Join(wd, encodeKey(t.Name()+"_tiered"+name))
	os.RemoveAll(dirAbsPath)
	t.Cleanup(func() {
		os.RemoveAll(dirAbsPath)
	})
	p, closeP, err := NewProvider(dirAbsPath, name, blockSize, bs, 1)
	t.Cleanup(func() {
		closeP()
	})
	if err != nil {
		t.Fatalf("failed to create provider: %s", err)
	}
	return p
}

func encodeKey(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func isSafeFilename(s string) bool {
	return safeName.MatchString(s) && len(s) < 1_000
}
