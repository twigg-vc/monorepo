package webdb_test

import (
	"monorepo/base/iterator"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
)

func collectLocalIds(t *testing.T, it iterator.I[commit.Commit]) []uint64 {
	t.Helper()
	ids := []uint64{}
	for it.Next() {
		c, err := it.Get()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.L)
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func Test_Commit(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, commitW, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := cliDb.GetLatestCommitByLocalId(w, 99, 7)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}
	_, isNotFoundErr, err = cliDb.GetCommitVersionByLocalId(w, 99, 7, 0)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	c0 := commit.Commit{L: 7, Version: 0, AuthorUserId: 42}
	err = cliDb.SetCommit(w, "owner", 99, c0)
	if err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err := cliDb.GetLatestCommitByLocalId(w, 99, 7)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 0 {
		t.Fatalf("Version=%d, expected 0", got.Version)
	}

	// Amend: write a new version of the same commit
	c1 := commit.Commit{L: 7, Version: 1, AuthorUserId: 42}
	err = cliDb.SetCommit(w, "owner", 99, c1)
	if err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err = cliDb.GetLatestCommitByLocalId(w, 99, 7)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 1 {
		t.Fatalf("Version=%d, expected 1", got.Version)
	}
	got, isNotFoundErr, err = cliDb.GetCommitVersionByLocalId(w, 99, 7, 0)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 0 {
		t.Fatalf("Version=%d, expected 0", got.Version)
	}

	c2 := commit.Commit{L: 7, Version: 2, AuthorUserId: 42, IsSubmitted: true}
	err = cliDb.SetCommit(w, "owner", 99, c2)
	if err != nil {
		t.Fatal(err)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_CommitByServerId(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, commitW, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	_, isNotFoundErr, err := cliDb.GetLatestCommitByServerId(w, 99, 500)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}
	_, isNotFoundErr, err = cliDb.GetCommitVersionByServerId(w, 99, 500, 0)
	if err == nil || !isNotFoundErr {
		t.Fatalf("got no isNotFoundErr")
	}

	c0 := commit.Commit{L: 7, Version: 0, AuthorUserId: 42, HasServerL: true, ServerL: 500, HasServerV: true, ServerV: 0}
	err = cliDb.SetCommit(w, "owner", 99, c0)
	if err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err := cliDb.GetLatestCommitByServerId(w, 99, 500)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 0 {
		t.Fatalf("Version=%d, expected 0", got.Version)
	}

	// Amend: write a new version, still under the same server commit id but
	// with a bumped server version
	c1 := commit.Commit{L: 7, Version: 1, AuthorUserId: 42, HasServerL: true, ServerL: 500, HasServerV: true, ServerV: 1}
	err = cliDb.SetCommit(w, "owner", 99, c1)
	if err != nil {
		t.Fatal(err)
	}

	got, isNotFoundErr, err = cliDb.GetLatestCommitByServerId(w, 99, 500)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 1 {
		t.Fatalf("Version=%d, expected 1", got.Version)
	}

	got, isNotFoundErr, err = cliDb.GetCommitVersionByServerId(w, 99, 500, 0)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 0 || got.ServerV != 0 {
		t.Fatalf("got=%+v, expected Version=0, ServerV=0", got)
	}

	got, isNotFoundErr, err = cliDb.GetCommitVersionByServerId(w, 99, 500, 1)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got.Version != 1 || got.ServerV != 1 {
		t.Fatalf("got=%+v, expected Version=1, ServerV=1", got)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_CommitChildren(t *testing.T) {
	db := getNewDb(t)

	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	const repoId = 87

	// Build the following tree:
	//
	//  c4v0
	//  |
	//  ~
	//      c3v0
	//      |
	// c1v0 c1v1 c2v0
	// |   /    /
	// root----/
	root := commit.Commit{L: 0, Version: 0}
	c1V0 := commit.Commit{L: 1, Version: 0, ParentL: 0, ParentV: 0}
	c1V1 := commit.Commit{L: 1, Version: 1, ParentL: 0, ParentV: 0}
	c2v0 := commit.Commit{L: 2, Version: 0, ParentL: 0, ParentV: 0}
	c3v0 := commit.Commit{L: 3, Version: 0, ParentL: 1, ParentV: 1}
	c4V0 := commit.Commit{L: 4, Version: 0, IsDetached: true}
	all := []commit.Commit{root, c1V0, c1V1, c2v0, c3v0, c4V0}
	for _, c := range all {
		err = db.SetCommit(w, "owner", repoId, c)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Helper to check children of each commit
	checkChildren := func(id commit.LocalId, v uint64,
		expectedChildrenIds []commit.LocalId, expectedChildrenV []uint64) {
		t.Helper()

		// Fix nil slice for DeepEqual to work ok
		if len(expectedChildrenIds) == 0 {
			expectedChildrenIds = nil
		}
		if len(expectedChildrenV) == 0 {
			expectedChildrenV = nil
		}

		ids, versions, err := db.GetCommitChildren(w, repoId,
			id, v, len(expectedChildrenIds))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, expectedChildrenIds) {
			t.Fatalf("ids=%v, expected %v", ids, expectedChildrenIds)
		}
		if !reflect.DeepEqual(versions, expectedChildrenV) {
			t.Fatalf("versions=%v, expected %v", versions, expectedChildrenV)
		}

	}

	// root
	checkChildren(0, 0, []commit.LocalId{1, 1, 2}, []uint64{0, 1, 0})
	// c1v0
	checkChildren(1, 0, []commit.LocalId{}, []uint64{})
	// c1v1
	checkChildren(1, 1, []commit.LocalId{3}, []uint64{0})
	// c2v0
	checkChildren(2, 0, []commit.LocalId{}, []uint64{})
	// c3v0
	checkChildren(3, 0, []commit.LocalId{}, []uint64{})

	// Another repo has no children at all
	ids, _, err := db.GetCommitChildren(w, repoId+1, 0, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("ids=%v, expected no children", ids)
	}

	// Reading more children than the max leads to an error
	_, _, err = db.GetCommitChildren(w, repoId, 0, 0,
		/*maxRowsToRead*/ 1)
	if err == nil {
		t.Fatal("expected error when reading more children than the max")
	}
}

func Test_PendingCommits(t *testing.T) {
	cliDb := getNewDb(t)

	w, closeW, commitW, err := cliDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	// repo 99: three pending commits and one submitted commit (which must
	// not show up as pending)
	for _, L := range []uint64{1, 2, 3} {
		err = cliDb.SetCommit(w, "owner", 99, commit.Commit{L: L, Version: 0, AuthorUserId: 42})
		if err != nil {
			t.Fatal(err)
		}
	}
	err = cliDb.SetCommit(w, "owner", 99, commit.Commit{L: 4, Version: 0, AuthorUserId: 42, IsSubmitted: true})
	if err != nil {
		t.Fatal(err)
	}

	// A different repo's pending commits must not interfere
	err = cliDb.SetCommit(w, "owner", 100, commit.Commit{L: 1, Version: 0, AuthorUserId: 42})
	if err != nil {
		t.Fatal(err)
	}

	ascIt, err := cliDb.GetPendingCommits(w, true, 99)
	if err != nil {
		t.Fatal(err)
	}
	if got := collectLocalIds(t, ascIt); !reflect.DeepEqual(got, []uint64{1, 2, 3}) {
		t.Fatalf("got=%v, expected [1 2 3]", got)
	}

	descIt, err := cliDb.GetPendingCommits(w, false, 99)
	if err != nil {
		t.Fatal(err)
	}
	if got := collectLocalIds(t, descIt); !reflect.DeepEqual(got, []uint64{3, 2, 1}) {
		t.Fatalf("got=%v, expected [3 2 1]", got)
	}

	afterIt, err := cliDb.GetPendingCommitsAfter(w, 99, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := collectLocalIds(t, afterIt); !reflect.DeepEqual(got, []uint64{2, 1}) {
		t.Fatalf("got=%v, expected [2 1]", got)
	}

	err = commitW()
	if err != nil {
		t.Fatal(err)
	}
}