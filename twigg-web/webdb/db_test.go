package webdb_test

import (
	"errors"
	"io"
	"monorepo/data/blobdb"
	"monorepo/data/fileblobstore"
	"monorepo/twigg-web/webdb"
	"testing"
)

// Large enough to not matter for tests that don't exercise blob storage.
const defaultTestBlockSize = 4 * 1024 * 1024 * 1024 // 4GB

type bytesWriterTo []byte

func (b bytesWriterTo) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b)
	return int64(n), err
}

func readAll(t *testing.T, r io.Reader, closeR func()) []byte {
	defer closeR()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func getNewDb(t *testing.T) webdb.WebDb {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	return db
}

// New must set up the sqlite db and the datastrip on disk
func Test_NewOnDisk(t *testing.T) {
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	cliDb, closeDb, err := webdb.New(t.TempDir(), "test.db",
		defaultTestBlockSize, blobs, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()

	w, closeW, commitW, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()
	err = cliDb.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

// A blob bigger than the block size must be split across multiple blocks.
// With a real BlobStorage wired in, older blocks get uploaded and their
// local file truncated to save disk space.
func Test_BlobStorageIsWiredThrough(t *testing.T) {
	const blockSize = 3
	const blobStorageCacheCapacity = 1
	dir := t.TempDir()
	blobs := fileblobstore.NewTestFileBlobStorage(t)
	db, closeDb, err := webdb.New(dir, "test.db",
		blockSize, blobs, blobStorageCacheCapacity, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)

	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	// 16 bytes over a block size of 3 forces AT LEAST 5 full blocks (0-4) plus a
	// latest, partially-filled block (5). There might be more because extra
	// data might be written to represent the encoding.
	const data = "0123456789ABCDEF"
	v, err := db.SetBlob(w, "owner", "prefix", "id", bytesWriterTo(data))
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
	}
	n, err := blobs.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n < 5 {
		t.Fatalf("count=%d, expected >=5", n)
	}

	// Reading it back must still work: evicted blocks (cache capacity 1) are
	// downloaded from the BlobStorage on demand.
	m, r, closeR, err := db.GetBlob(w, "prefix", "id")
	if err != nil {
		closeR()
		t.Fatal(err)
	}
	got := readAll(t, r, closeR)
	if string(got) != data {
		t.Fatalf("got %q, expected %q", got, data)
	}
	if m.Size != int64(len(data)) {
		t.Fatalf("Size=%d, expected %d", m.Size, len(data))
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_RepoNextLocalId(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := db.GetRepoNextLocalId(w, 99)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	err = db.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}

	nextId, isNotFoundErr, err := db.GetRepoNextLocalId(w, 99)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if nextId != 3 {
		t.Fatalf("nextId=%d, expected 3", nextId)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_RepoTopCommit(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := db.GetRepoTopCommit(w, 99)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	err = db.SetRepoTopCommit(w, 99, 7)
	if err != nil {
		t.Fatal(err)
	}

	topId, isNotFoundErr, err := db.GetRepoTopCommit(w, 99)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if topId != 7 {
		t.Fatalf("topId=%d, expected 7", topId)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Blob(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, _, closeR, err := db.GetBlob(w, "prefix", "id")
	closeR()
	if !errors.Is(err, blobdb.ErrNotFound) {
		t.Fatalf("err=%v, expected ErrNotFound", err)
	}
	_, _, closeR, err = db.GetBlobVersion(w, "prefix", "id", 0)
	closeR()
	if !errors.Is(err, blobdb.ErrNotFound) {
		t.Fatalf("err=%v, expected ErrNotFound", err)
	}

	// First write must create version 0
	v, err := db.SetBlob(w, "owner", "prefix", "id", bytesWriterTo("v0-data"))
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
	}

	// Second write must create version 1
	v, err = db.SetBlob(w, "owner", "prefix", "id", bytesWriterTo("v1-data"))
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Fatalf("v=%d, expected 1", v)
	}

	// GetBlob must return the latest version
	m, r, closeR, err := db.GetBlob(w, "prefix", "id")
	if err != nil {
		closeR()
		t.Fatal(err)
	}
	if m.Version != 1 {
		t.Fatalf("m.Version=%d, expected 1", m.Version)
	}
	data := readAll(t, r, closeR)
	if string(data) != "v1-data" {
		t.Fatalf("data=%q, expected %q", data, "v1-data")
	}

	// GetBlobVersion must return the requested version
	m, r, closeR, err = db.GetBlobVersion(w, "prefix", "id", 0)
	if err != nil {
		closeR()
		t.Fatal(err)
	}
	if m.Version != 0 {
		t.Fatalf("m.Version=%d, expected 0", m.Version)
	}
	data = readAll(t, r, closeR)
	if string(data) != "v0-data" {
		t.Fatalf("data=%q, expected %q", data, "v0-data")
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}
