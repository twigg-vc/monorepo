package webdb_test

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"slices"
	"testing"
)

func TestPutAndArchivePipelineRef(t *testing.T) {
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
		name   = "jobname"
	)
	ref := job.PipelineRef{RepoId: repoId, Path: path, Name: name}

	if got := readRefs(t, b, w, repoId); len(got) != 0 {
		t.Fatalf("expected no refs before the put, got %+v", got)
	}

	err = b.PutPipelineRef(w, repoId, path, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRefs(t, b, w, repoId); !slices.Equal(got, []job.PipelineRef{ref}) {
		t.Fatalf("expected the put ref, got %+v", got)
	}

	// Putting it again is idempotent
	err = b.PutPipelineRef(w, repoId, path, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRefs(t, b, w, repoId); !slices.Equal(got, []job.PipelineRef{ref}) {
		t.Fatalf("expected a single ref after the second put, got %+v", got)
	}

	err = b.ArchivePipelineRef(w, repoId, path, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRefs(t, b, w, repoId); len(got) != 0 {
		t.Fatalf("expected no refs after the archive, got %+v", got)
	}

	// Putting an archived ref un-archives it
	err = b.PutPipelineRef(w, repoId, path, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRefs(t, b, w, repoId); !slices.Equal(got, []job.PipelineRef{ref}) {
		t.Fatalf("expected the ref to be un-archived, got %+v", got)
	}

	// Archiving a ref that does not exist is a no-op
	err = b.ArchivePipelineRef(w, repoId, path, "other")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetRepoPipelineRefs(t *testing.T) {
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
	put := func(rId uint64, path, name string) job.PipelineRef {
		t.Helper()
		err := b.PutPipelineRef(w, rId, path, name)
		if err != nil {
			t.Fatal(err)
		}
		return job.PipelineRef{RepoId: rId, Path: path, Name: name}
	}
	// Put out of order to check the ordering
	bPath := put(repoId, "b/path", "name")
	aName1 := put(repoId, "a/path", "name-1")
	aName0 := put(repoId, "a/path", "name-0")
	put(repoId+1, "other/path", "name")

	// Ordered by path and name, scoped to the repo
	got := readRefs(t, b, w, repoId)
	if !slices.Equal(got, []job.PipelineRef{aName0, aName1, bPath}) {
		t.Fatalf("expected the refs ordered by path and name, got %+v", got)
	}

	// afterPath and afterName start after a ref
	iter, err := b.GetRepoPipelineRefs(w, repoId, "a/path", "name-0")
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(10, iter)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []job.PipelineRef{aName1, bPath}) {
		t.Fatalf("expected the refs after a/path name-0, got %+v", got)
	}
}

func readRefs(t *testing.T, b webdb.WebDb, ctx context.Context, repoId uint64) []job.PipelineRef {
	t.Helper()
	iter, err := b.GetRepoPipelineRefs(ctx, repoId, "", "")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := iterator.GetFirstN(10, iter)
	if err != nil {
		t.Fatal(err)
	}
	return refs
}
