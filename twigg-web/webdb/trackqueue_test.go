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
