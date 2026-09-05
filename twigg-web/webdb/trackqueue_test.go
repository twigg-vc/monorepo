package webdb_test

import (
	"bytes"
	"errors"
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

func TestInsertZeroTrackOwnerUsageIfNotExists(t *testing.T) {
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

func TestSetTrackOwnerLimits(t *testing.T) {
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

	// The owner does not have to be tracked yet.
	if err := b.SetTrackOwnerLimits(w, 7, 3, 90_000); err != nil {
		t.Fatal(err)
	}
	maxJobs, maxTimeoutMs, isNotFoundErr, err := b.GetTrackOwnerLimits(w, 7)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("the owner was not tracked")
	}
	if maxJobs != 3 || maxTimeoutMs != 90_000 {
		t.Fatalf("got limits %d/%d, want 3/90000", maxJobs, maxTimeoutMs)
	}

	// Queueing a job for the owner must not reset the limits.
	if err := b.InsertZeroTrackOwnerUsageIfNotExists(w, 7); err != nil {
		t.Fatal(err)
	}
	maxJobs, maxTimeoutMs, _, err = b.GetTrackOwnerLimits(w, 7)
	if err != nil {
		t.Fatal(err)
	}
	if maxJobs != 3 || maxTimeoutMs != 90_000 {
		t.Fatalf("the limits were reset to %d/%d", maxJobs, maxTimeoutMs)
	}

	// Setting them again overwrites them.
	if err := b.SetTrackOwnerLimits(w, 7, 5, 120_000); err != nil {
		t.Fatal(err)
	}
	maxJobs, maxTimeoutMs, _, err = b.GetTrackOwnerLimits(w, 7)
	if err != nil {
		t.Fatal(err)
	}
	if maxJobs != 5 || maxTimeoutMs != 120_000 {
		t.Fatalf("got limits %d/%d, want 5/120000", maxJobs, maxTimeoutMs)
	}
}

func TestGetAndDeleteTrackQueueJob(t *testing.T) {
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

	payload := []byte(`{"Name":"a-job"}`)
	if err := b.InsertTrackQueueJobIfNotExists(w, "job-1", 7, payload, "queued", 1234); err != nil {
		t.Fatal(err)
	}

	ownerId, gotPayload, isNotFoundErr, err := b.GetTrackQueueJobOwnerAndPayload(w, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("the queued job was not found")
	}
	if ownerId != 7 {
		t.Fatalf("got ownerId %d, want 7", ownerId)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("got payload %q, want %q", gotPayload, payload)
	}

	if err := b.DeleteTrackQueueJob(w, "job-1"); err != nil {
		t.Fatal(err)
	}
	count, err := b.CountTrackQueueJobs(w)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("the job was not deleted: %d jobs left", count)
	}

	// A finished job that is no longer queued is not an error.
	_, _, isNotFoundErr, err = b.GetTrackQueueJobOwnerAndPayload(w, "job-1")
	if !isNotFoundErr {
		t.Fatalf("expected isNotFoundErr, got err %v", err)
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetOldestTrackQueueJobWithinOwnerLimits(t *testing.T) {
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

	// The owner takes the default limits: one job at a time.
	if err := b.InsertZeroTrackOwnerUsageIfNotExists(w, 7); err != nil {
		t.Fatal(err)
	}
	if err := b.InsertTrackQueueJobIfNotExists(w, "newer", 7, []byte("b"), "queued", 2000); err != nil {
		t.Fatal(err)
	}
	if err := b.InsertTrackQueueJobIfNotExists(w, "older", 7, []byte("a"), "queued", 1000); err != nil {
		t.Fatal(err)
	}

	// The oldest queued job comes first, not the one inserted first.
	jobId, ownerId, payload, isNotFoundErr, err := b.GetOldestTrackQueueJobWithinOwnerLimits(w, "queued")
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("no job was within the limits")
	}
	if jobId != "older" || ownerId != 7 || !bytes.Equal(payload, []byte("a")) {
		t.Fatalf("got job %q owner %d payload %q", jobId, ownerId, payload)
	}

	// Publishing it takes the owner to its limit of one running job, so the
	// job left is not within the limits any more.
	if err := b.SetTrackQueueJobStatus(w, "older", "published"); err != nil {
		t.Fatal(err)
	}
	if err := b.AddTrackOwnerUsage(w, 7, 1, 0); err != nil {
		t.Fatal(err)
	}
	_, _, _, isNotFoundErr, err = b.GetOldestTrackQueueJobWithinOwnerLimits(w, "queued")
	if !isNotFoundErr {
		t.Fatalf("expected the owner to be at its limit, got err %v", err)
	}

	// Once the published job finishes, the one left is picked.
	if err := b.AddTrackOwnerUsage(w, 7, -1, 0); err != nil {
		t.Fatal(err)
	}
	jobId, _, _, _, err = b.GetOldestTrackQueueJobWithinOwnerLimits(w, "queued")
	if err != nil {
		t.Fatal(err)
	}
	if jobId != "newer" {
		t.Fatalf("got job %q, want %q", jobId, "newer")
	}
}
