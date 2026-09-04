package clidb_test

import (
	"errors"
	"io"
	"monorepo/data/blobdb"
	"monorepo/twigg/cli/clidb"
	"monorepo/twigg/clistate"
	"reflect"
	"testing"
)

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

func getNewDb(t *testing.T) clidb.CliDb {
	db, closeDb, err := clidb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	return db
}

// New must set up the sqlite db and the datastrip on disk
func Test_NewOnDisk(t *testing.T) {
	cliDb, closeDb, err := clidb.New(t.TempDir(), "test.db")
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

func Test_CliState(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := db.GetCliState(w)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	st := clistate.State{
		ServerUrl:           "https://example.com",
		ApiKey:              "some-api-key",
		EnableUnsafeDevMode: true,
	}
	st.Current.L = 7
	st.Current.Version = 2

	err = db.SetCliState(w, st)
	if err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err := db.GetCliState(w)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, st) {
		t.Fatalf("got=%+v, expected %+v", got, st)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_WorkdirCache(t *testing.T) {
	db := getNewDb(t)
	w, closeW, commitW, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, _, _, _, isNotFoundErr, err := db.GetWorkdirCache(w, "a/b.txt")
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	var hash [32]byte
	for i := range hash {
		hash[i] = byte(i)
	}
	err = db.SetWorkdirCache(w, "a/b.txt", 123, 456, hash, true)
	if err != nil {
		t.Fatal(err)
	}

	size, modTime, gotHash, isText, isNotFoundErr, err := db.GetWorkdirCache(w, "a/b.txt")
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if size != 123 {
		t.Fatalf("size=%d, expected 123", size)
	}
	if modTime != 456 {
		t.Fatalf("modTime=%d, expected 456", modTime)
	}
	if gotHash != hash {
		t.Fatalf("hash=%v, expected %v", gotHash, hash)
	}
	if !isText {
		t.Fatalf("isText=false, expected true")
	}

	// Overwriting the same path must update the row
	err = db.SetWorkdirCache(w, "a/b.txt", 321, 654, hash, false)
	if err != nil {
		t.Fatal(err)
	}
	size, modTime, _, isText, isNotFoundErr, err = db.GetWorkdirCache(w, "a/b.txt")
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if size != 321 {
		t.Fatalf("size=%d, expected 321", size)
	}
	if modTime != 654 {
		t.Fatalf("modTime=%d, expected 654", modTime)
	}
	if isText {
		t.Fatalf("isText=true, expected false")
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}
