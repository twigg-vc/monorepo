package webdb_test

import (
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"slices"
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

func TestSetPipelineStatus(t *testing.T) {
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
		RepoId:         1,
		Commit:         2,
		CommitVersion:  3,
		Path:           "file/path",
		Name:           "pipename",
		RunNumber:      4,
		NumberOfStages: 1,
		Status:         job.PipelineStatusRunning,
		CreatedTime:    "2026-09-05T00:00:00Z",
	}
	p.InternalId, err = b.InsertPipeline(w, p)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetPipelineStatus(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name,
		p.RunNumber, job.PipelineStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := b.GetPipeline(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != job.PipelineStatusSuccess {
		t.Fatalf("expected status %q, got %q", job.PipelineStatusSuccess, got.Status)
	}

	// Setting the status of a pipeline that does not exist is a no-op
	err = b.SetPipelineStatus(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name,
		p.RunNumber+1, job.PipelineStatusFail)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetPipelineStatus(w, p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name,
		p.RunNumber, "")
	if err == nil {
		t.Fatal("expected an error when the status is missing")
	}
}

func TestGetRepoPipelinesByRef(t *testing.T) {
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
		repoId = uint64(1)
		path   = "file/path"
		name   = "pipename"
	)
	insert := func(pPath, pName string, runNumber int64) job.Pipeline {
		t.Helper()
		p := job.Pipeline{
			RepoId:         repoId,
			Commit:         2,
			CommitVersion:  3,
			Path:           pPath,
			Name:           pName,
			RunNumber:      runNumber,
			NumberOfStages: 1,
			Status:         job.PipelineStatusRunning,
			CreatedTime:    "2026-09-05T00:00:00Z",
		}
		p.InternalId, err = b.InsertPipeline(w, p)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	run0 := insert(path, name, 0)
	run1 := insert(path, name, 1)
	insert(path, "other-name", 0)

	const readAll = 10

	// Newest first, scoped to the ref
	iter, err := b.GetRepoPipelinesByRef(w, repoId, path, name, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []job.Pipeline{run1, run0}) {
		t.Fatalf("expected the pipelines of the ref newest first, got %+v", got)
	}

	// afterInternalPipelineId reads the ones after a previously read pipeline
	iter, err = b.GetRepoPipelinesByRef(w, repoId, path, name, run1.InternalId)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []job.Pipeline{run0}) {
		t.Fatalf("expected only the pipelines after %d, got %+v", run1.InternalId, got)
	}

	// A ref with no pipelines has none
	iter, err = b.GetRepoPipelinesByRef(w, repoId, path, "unknown", 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no pipelines of an unknown ref, got %+v", got)
	}
}