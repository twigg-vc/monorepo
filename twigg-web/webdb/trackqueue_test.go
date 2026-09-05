package webdb_test

import (
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestInsertAndCountTrackQueueJobs(t *testing.T) {
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

	count, err := b.CountTrackQueueJobs(w)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected an empty queue, got %d jobs", count)
	}

	if err := b.InsertTrackQueueJobIfNotExists(w, "job-1", 7, []byte("payload"), "queued", 1234); err != nil {
		t.Fatal(err)
	}
	if err := b.InsertTrackQueueJobIfNotExists(w, "job-2", 7, []byte("payload"), "queued", 5678); err != nil {
		t.Fatal(err)
	}

	count, err = b.CountTrackQueueJobs(w)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 jobs, got %d", count)
	}

	// Putting a job is idempotent.
	if err := b.InsertTrackQueueJobIfNotExists(w, "job-1", 8, []byte("other"), "queued", 9999); err != nil {
		t.Fatal(err)
	}
	count, err = b.CountTrackQueueJobs(w)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("the conflicting insert added a job: got %d", count)
	}
}

func TestInsertTrackOwnerUsageIfNotExists(t *testing.T) {
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

	_, _, isNotFoundErr, err := b.GetTrackOwnerLimits(w, 7)
	if !isNotFoundErr {
		t.Fatalf("expected the owner to not be tracked yet, got err %v", err)
	}

	if err := b.InsertZeroTrackOwnerUsageIfNotExists(w, 7); err != nil {
		t.Fatal(err)
	}

	// A new owner gets the limits the table defaults to.
	maxJobs, maxTimeoutMs, isNotFoundErr, err := b.GetTrackOwnerLimits(w, 7)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("the tracked owner was not found")
	}
	if maxJobs != 1 {
		t.Fatalf("got maxJobs %d, want 1", maxJobs)
	}
	if maxTimeoutMs != 60_000 {
		t.Fatalf("got maxTimeoutMs %d, want 60000", maxTimeoutMs)
	}
}
