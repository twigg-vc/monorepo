package webdb_test

import (
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"testing"
)

func TestInsertAndGetPipeline(t *testing.T) {
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

	p := job.Pipeline{
		RepoId:          1,
		Commit:          2,
		CommitVersion:   3,
		Path:            "file/path",
		Name:            "pipename",
		RunNumber:       4,
		NumberOfStages:  3,
		Status:          job.PipelineStatusRunning,
		CreatedTime:     "2026-09-05T00:00:00Z",
		IsCreatedByUser: true,
		CreatedByUserId: 99,
	}

	exists, err := b.PipelineExists(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no pipeline before the insert")
	}
	_, isNotFoundErr, err := b.GetPipeline(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}

	p.InternalId, err = b.InsertPipeline(w, p)
	if err != nil {
		t.Fatal(err)
	}
	if p.InternalId == 0 {
		t.Fatal("expected an internal pipeline id")
	}

	exists, err = b.PipelineExists(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected the inserted pipeline to exist")
	}

	got, isNotFoundErr, err := b.GetPipeline(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	if got != p {
		t.Fatalf("expected pipeline %+v, got %+v", p, got)
	}

	// Only the exact key matches
	exists, err = b.PipelineExists(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber+1)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected no pipeline for another run number")
	}
}

func TestInsertPipelineWithoutStatusOrCreatedTime(t *testing.T) {
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

	_, err = b.InsertPipeline(w, job.Pipeline{CreatedTime: "2026-09-05T00:00:00Z"})
	if err == nil {
		t.Fatal("expected an error when the status is missing")
	}
	_, err = b.InsertPipeline(w, job.Pipeline{Status: job.PipelineStatusRunning})
	if err == nil {
		t.Fatal("expected an error when the createdTime is missing")
	}
}
