package webdb_test

import (
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestInsertAndCheckCiCdRun(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const (
		repoId        = uint64(1)
		commitId      = uint64(2)
		commitVersion = uint64(3)
		runNumber     = int64(4)
	)

	exists, err := b.CiCdRunExists(w, repoId, commitId, commitVersion, runNumber)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no run before the insert")
	}

	err = b.InsertCiCdRun(w, repoId, commitId, commitVersion, runNumber, "nonce-0")
	if err != nil {
		t.Fatal(err)
	}

	exists, err = b.CiCdRunExists(w, repoId, commitId, commitVersion, runNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected the inserted run to exist")
	}

	// Only the exact key matches
	exists, err = b.CiCdRunExists(w, repoId, commitId, commitVersion, runNumber+1)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no run for another run number")
	}
	exists, err = b.CiCdRunExists(w, repoId, commitId, commitVersion+1, runNumber)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no run for another commit version")
	}
}

func TestInsertCiCdRunWithoutNonce(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	err = b.InsertCiCdRun(w, 1, 2, 3, 4, "")
	if err == nil {
		t.Fatal("expected an error when the nonce is missing")
	}
}
