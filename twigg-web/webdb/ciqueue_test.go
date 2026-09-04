package webdb_test

import (
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestInsertAndGetCiCdQueueRun(t *testing.T) {
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
	)

	// Empty before any insert
	_, isNotFoundErr, err := b.GetCiCdQueueLastRunNumber(w, repoId, commitId, commitVersion)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	_, isNotFoundErr, err = b.GetCiCdQueueLatestRunStatus(w, repoId, commitId, commitVersion)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	_, _, isNotFoundErr, err = b.GetCiCdQueueRunTriggerAndStatus(w, repoId, commitId, commitVersion, 0, "nonce-0")
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}

	err = b.InsertCiCdQueueRun(w, repoId, commitId, commitVersion, 0, "on-push", "nonce-0", "prepared")
	if err != nil {
		t.Fatal(err)
	}

	runNumber, isNotFoundErr, err := b.GetCiCdQueueLastRunNumber(w, repoId, commitId, commitVersion)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	if runNumber != 0 {
		t.Fatalf("expected run number 0, got %d", runNumber)
	}

	trigger, status, isNotFoundErr, err := b.GetCiCdQueueRunTriggerAndStatus(
		w, repoId, commitId, commitVersion, 0, "nonce-0")
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	if trigger != "on-push" {
		t.Fatalf("got trigger %q", trigger)
	}
	if status != "prepared" {
		t.Fatalf("got status %q", status)
	}

	// Wrong nonce should not match
	_, _, isNotFoundErr, err = b.GetCiCdQueueRunTriggerAndStatus(
		w, repoId, commitId, commitVersion, 0, "wrong-nonce")
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
}

func TestCiCdQueueLastRunNumberAndLatestStatus(t *testing.T) {
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
	)
	err = b.InsertCiCdQueueRun(w, repoId, commitId, commitVersion, 0, "on-push", "nonce-0", "started")
	if err != nil {
		t.Fatal(err)
	}
	err = b.InsertCiCdQueueRun(w, repoId, commitId, commitVersion, 1, "on-push", "nonce-1", "prepared")
	if err != nil {
		t.Fatal(err)
	}

	runNumber, _, err := b.GetCiCdQueueLastRunNumber(w, repoId, commitId, commitVersion)
	if err != nil {
		t.Fatal(err)
	}
	if runNumber != 1 {
		t.Fatalf("expected run number 1, got %d", runNumber)
	}

	// Latest status is the one of the highest run number
	status, _, err := b.GetCiCdQueueLatestRunStatus(w, repoId, commitId, commitVersion)
	if err != nil {
		t.Fatal(err)
	}
	if status != "prepared" {
		t.Fatalf("got status %q", status)
	}

	// Other commit versions are unaffected
	_, isNotFoundErr, err := b.GetCiCdQueueLastRunNumber(w, repoId, commitId, commitVersion+1)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
}

func TestSetCiCdQueueRunStatus(t *testing.T) {
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
	)
	err = b.InsertCiCdQueueRun(w, repoId, commitId, commitVersion, 0, "on-push", "nonce-0", "prepared")
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetCiCdQueueRunStatus(w, repoId, commitId, commitVersion, 0, "nonce-0", "started")
	if err != nil {
		t.Fatal(err)
	}
	_, status, _, err := b.GetCiCdQueueRunTriggerAndStatus(w, repoId, commitId, commitVersion, 0, "nonce-0")
	if err != nil {
		t.Fatal(err)
	}
	if status != "started" {
		t.Fatalf("got status %q", status)
	}

	// Setting with a wrong nonce is a no-op
	err = b.SetCiCdQueueRunStatus(w, repoId, commitId, commitVersion, 0, "wrong-nonce", "prepared")
	if err != nil {
		t.Fatal(err)
	}
	_, status, _, err = b.GetCiCdQueueRunTriggerAndStatus(w, repoId, commitId, commitVersion, 0, "nonce-0")
	if err != nil {
		t.Fatal(err)
	}
	if status != "started" {
		t.Fatalf("got status %q", status)
	}
}

func TestInsertCiCdQueueRunMissingFields(t *testing.T) {
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

	err = b.InsertCiCdQueueRun(w, 1, 2, 3, 0, "", "nonce-0", "prepared")
	if err == nil {
		t.Fatal("expected error inserting with empty trigger")
	}
	err = b.InsertCiCdQueueRun(w, 1, 2, 3, 0, "on-push", "", "prepared")
	if err == nil {
		t.Fatal("expected error inserting with empty nonce")
	}
	err = b.InsertCiCdQueueRun(w, 1, 2, 3, 0, "on-push", "nonce-0", "")
	if err == nil {
		t.Fatal("expected error inserting with empty status")
	}
}
