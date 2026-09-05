package webdb_test

import (
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestInsertJob(t *testing.T) {
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

	j := job.Job{
		RepoId:        1,
		Commit:        2,
		CommitVersion: 3,
		Path:          "file/path",
		Name:          "jobname",
		RunNumber:     4,
		Status:        job.JobStatusQueued,
		CreatedTime:   "2026-09-05T00:00:00Z",
	}

	exists, err := b.JobExists(w, j.RepoId, j.Commit, j.CommitVersion, j.Path, j.Name, j.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no job before the insert")
	}

	internalJobId, err := b.InsertJob(w, j)
	if err != nil {
		t.Fatal(err)
	}
	if internalJobId == 0 {
		t.Fatal("expected an internal job id")
	}

	exists, err = b.JobExists(w, j.RepoId, j.Commit, j.CommitVersion, j.Path, j.Name, j.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected the inserted job to exist")
	}

	// Only the exact key matches
	exists, err = b.JobExists(w, j.RepoId, j.Commit, j.CommitVersion, j.Path, "other", j.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no job for another name")
	}

	// The internal ids are unique
	other := j
	other.RunNumber = 5
	otherInternalJobId, err := b.InsertJob(w, other)
	if err != nil {
		t.Fatal(err)
	}
	if otherInternalJobId == internalJobId {
		t.Fatalf("expected a new internal job id, got %d twice", internalJobId)
	}
}

func TestInsertJobWithoutStatusOrCreatedTime(t *testing.T) {
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

	_, err = b.InsertJob(w, job.Job{CreatedTime: "2026-09-05T00:00:00Z"})
	if err == nil {
		t.Fatal("expected an error when the status is missing")
	}
	_, err = b.InsertJob(w, job.Job{Status: job.JobStatusQueued})
	if err == nil {
		t.Fatal("expected an error when the createdTime is missing")
	}
}
