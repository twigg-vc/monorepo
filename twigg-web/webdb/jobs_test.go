package webdb_test

import (
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"slices"
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

func TestGetAndSetJobStatus(t *testing.T) {
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

	_, isNotFoundErr, err := b.GetJob(w, j.RepoId, j.Commit, j.CommitVersion, j.Path, j.Name, j.RunNumber)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}
	isNotFoundErr, err = b.SetJobStatus(w, j.RepoId, j.Commit, j.CommitVersion, j.Path,
		j.Name, j.RunNumber, job.JobStatusRunning)
	if !isNotFoundErr || err == nil {
		t.Fatal("expected is not found err")
	}

	j.InternalId, err = b.InsertJob(w, j)
	if err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err := b.GetJob(w, j.RepoId, j.Commit, j.CommitVersion, j.Path, j.Name, j.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}
	if got != j {
		t.Fatalf("expected job %+v, got %+v", j, got)
	}

	isNotFoundErr, err = b.SetJobStatus(w, j.RepoId, j.Commit, j.CommitVersion, j.Path,
		j.Name, j.RunNumber, job.JobStatusSuccess)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("got is not found err")
	}

	got, _, err = b.GetJob(w, j.RepoId, j.Commit, j.CommitVersion, j.Path, j.Name, j.RunNumber)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != job.JobStatusSuccess {
		t.Fatalf("expected status %q, got %q", job.JobStatusSuccess, got.Status)
	}
}

func TestSetJobStatusWithoutStatus(t *testing.T) {
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

	_, err = b.SetJobStatus(w, 1, 2, 3, "file/path", "jobname", 4, "")
	if err == nil {
		t.Fatal("expected an error when the status is missing")
	}
}

func TestGetCommitAndRepoJobs(t *testing.T) {
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

	const repoId = uint64(1)
	insert := func(commitId uint64, name string) job.Job {
		t.Helper()
		j := job.Job{
			RepoId:        repoId,
			Commit:        commitId,
			CommitVersion: 3,
			Path:          "file/path",
			Name:          name,
			RunNumber:     4,
			Status:        job.JobStatusQueued,
			CreatedTime:   "2026-09-05T00:00:00Z",
		}
		j.InternalId, err = b.InsertJob(w, j)
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	first := insert(2, "first")
	second := insert(2, "second")
	otherCommit := insert(7, "other-commit")

	const readAll = 10

	// The commit jobs come newest first, and exclude other commits
	iter, err := b.GetCommitJobs(w, repoId, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []job.Job{second, first}) {
		t.Fatalf("expected the commit jobs newest first, got %+v", got)
	}

	// afterInternalJobId reads the ones after a previously read job
	iter, err = b.GetCommitJobs(w, repoId, 2, second.InternalId)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []job.Job{first}) {
		t.Fatalf("expected only the jobs after %d, got %+v", second.InternalId, got)
	}

	// The repo jobs include every commit
	iter, err = b.GetRepoJobs(w, repoId, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []job.Job{otherCommit, second, first}) {
		t.Fatalf("expected every commit of the repo, got %+v", got)
	}

	// Another repo has none
	iter, err = b.GetRepoJobs(w, repoId+1, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(readAll, iter)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no jobs of another repo, got %+v", got)
	}
}