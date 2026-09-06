package clidb

import (
	"monorepo/data/blobdb"
	"monorepo/data/deltastream"
	"monorepo/data/sqlitehelper"
	"testing"
	"time"
)

func Test_BlobMetadataDb(t *testing.T) {
	s, err := sqlitehelper.NewSqliteHelper(sqlitehelper.InMemoryPathToDir, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	err = s.Init(embeddedMigrations)
	if err != nil {
		t.Fatal(err)
	}
	m := blobMetadataDb{s}

	w, closeW, commitW, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := m.GetLatestMetadata(w, "prefix", "id")
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}
	_, isNotFoundErr, err = m.GetMetadataByVersion(w, "prefix", "id", 0)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	// All fields must roundtrip
	in := blobdb.BlobData{
		IdPrefix:             "prefix",
		Id:                   "id",
		Version:              0,
		Size:                 100,
		CompressedSize:       42,
		SavedAt:              time.UnixMilli(123456789),
		IsDeleted:            false,
		QuotaOwner:           "owner",
		IsLatest:             true,
		Datastrip:            "ds0",
		Offset:               7,
		DistanceToNonDelta:   2,
		Encoding:             deltastream.CompressionMethodSpeedFlate,
		HasDeltaEncodingBase: true,
		DeltaEncodingBase:    987,
	}
	err = m.InsertMetadata(w, in)
	if err != nil {
		t.Fatal(err)
	}
	got, isNotFoundErr, err := m.GetLatestMetadata(w, "prefix", "id")
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if !got.SavedAt.Equal(in.SavedAt) {
		t.Fatalf("SavedAt=%v, expected %v", got.SavedAt, in.SavedAt)
	}
	got.SavedAt = in.SavedAt
	if got != in {
		t.Fatalf("got %+v, expected %+v", got, in)
	}

	got, isNotFoundErr, err = m.GetMetadataByVersion(w, "prefix", "id", 0)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 0 {
		t.Fatalf("Version=%d, expected 0", got.Version)
	}

	// Clearing isLatest must hide the row from GetLatestMetadata but keep it
	// reachable by version
	err = m.SetMetadata(w, "prefix", "id", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	_, isNotFoundErr, err = m.GetLatestMetadata(w, "prefix", "id")
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}
	got, isNotFoundErr, err = m.GetMetadataByVersion(w, "prefix", "id", 0)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.IsLatest {
		t.Fatalf("IsLatest=true, expected false")
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

// commitTx must sync the blob log before committing the metadata
// transaction, so metadata never points at unflushed bytes.
func Test_CommitSyncsBlobLog(t *testing.T) {
	cliDb, closeDb, err := newMemCliDb()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)

	log := cliDb.db.log.(*memAppendLog)

	w, closeW, commitW, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	if log.synced {
		t.Fatalf("synced=true before commit, expected false")
	}
	err = cliDb.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
	if !log.synced {
		t.Fatalf("synced=false after commit, expected true")
	}
}
