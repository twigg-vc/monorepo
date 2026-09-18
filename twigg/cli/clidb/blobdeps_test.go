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
	v0, err := m.GrabMetadataVersion(w, "prefix", "id")
	if err != nil {
		t.Fatal(err)
	}
	if v0 != 0 {
		t.Fatalf("got v=%d, expected 0", v0)
	}
	// All fields must roundtrip
	in := blobdb.BlobData{
		IdPrefix:             "prefix",
		Id:                   "id",
		Version:              v0,
		Size:                 100,
		CompressedSize:       42,
		SavedAt:              time.UnixMilli(123456789),
		IsDeleted:            false,
		QuotaOwner:           "owner",
		Datastrip:            "ds0",
		Offset:               7,
		DistanceToNonDelta:   2,
		Encoding:             deltastream.CompressionMethodSpeedFlate,
		HasDeltaEncodingBase: true,
		DeltaEncodingBase:    987,
	}
	err = m.SetMetadataGrabbedVersion(w, in)
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

	// A higher version must take over as the latest, and the older one must
	// stay reachable by version
	v1, err := m.GrabMetadataVersion(w, "prefix", "id")
	if err != nil {
		t.Fatal(err)
	}
	if v1 != 1 {
		t.Fatalf("got v=%d, expected 0", v1)
	}
	newer := got
	newer.Version = 1
	err = m.SetMetadataGrabbedVersion(w, newer)
	if err != nil {
		t.Fatal(err)
	}
	got, isNotFoundErr, err = m.GetLatestMetadata(w, "prefix", "id")
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != newer.Version {
		t.Fatalf("Version=%d, expected %d", got.Version, newer.Version)
	}
	got, isNotFoundErr, err = m.GetMetadataByVersion(w, "prefix", "id", 0)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 0 {
		t.Fatalf("Version=%d, expected 0", got.Version)
	}

	// Cant set version without first grabbing
	newer.Version = 999
	err = m.SetMetadataGrabbedVersion(w, newer)
	if err == nil {
		t.Fatal("no error when setting before grabing")
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

// Successive reservations of a blob must hand out 0, 1, 2... and each blob
// must get its own sequence
func Test_GrabMetadataVersion(t *testing.T) {
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

	for i := range 5 {
		v, err := m.GrabMetadataVersion(w, "prefix", "id")
		if err != nil {
			t.Fatal(err)
		}
		if v != blobdb.Version(i) {
			t.Fatalf("v=%d, expected %d", v, i)
		}
	}

	v, err := m.GrabMetadataVersion(w, "prefix", "id2")
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("v=%d, expected 0", v)
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

// A failed version grab must prevent the enclosing write transaction from
// being committed, just like a failed blob write
func Test_FailedGrabPreventsCommit(t *testing.T) {
	db, closeDb, err := newMemCliDb()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)

	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	// A successful write first, so the tx would be committable otherwise
	err = db.SetRepoNextLocalId(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !db.ShouldCommit(w) {
		t.Fatalf("ShouldCommit=false before the failed grab, expected true")
	}

	_, err = db.db.s.Exec(w, `DROP TABLE sqlarge_blob_versions`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.GrabBlobVersion(w, "prefix", "id")
	if err == nil {
		t.Fatalf("got no error grabbing a version without its table")
	}
	if db.ShouldCommit(w) {
		t.Fatalf("ShouldCommit=true after a failed grab, expected false")
	}
}
