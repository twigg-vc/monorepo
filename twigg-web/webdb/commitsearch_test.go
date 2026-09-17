package webdb_test

import (
	"context"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
)

const searchRepoId = 7

// Writes the commits the search tests run against:
//
//	c1 pending, author 1
//	c2 pending, author 2
//	c3 submitted, author 1
//	c4 pending WIP, author 2
//	c5 pending archived, author 1
func setUpSearchableCommits(t *testing.T, db webdb.WebDb, w context.Context) {
	t.Helper()
	commits := []commit.Commit{
		{L: 1, AuthorUserId: 1, Message: "Define user entity"},
		{L: 2, AuthorUserId: 2, Message: "Implement user methods"},
		{L: 3, AuthorUserId: 1, Message: "Add test to user handler",
			IsSubmitted: true},
		{L: 4, AuthorUserId: 2, Message: "wip: refactor the queue"},
		{L: 5, AuthorUserId: 1, Message: "#ARCHIVED old attempt"},
	}
	for _, c := range commits {
		if err := db.SetCommit(w, "owner", searchRepoId, c); err != nil {
			t.Fatal(err)
		}
	}
	// A commit of another repo must never show up
	err := db.SetCommit(w, "owner", searchRepoId+1,
		commit.Commit{L: 1, AuthorUserId: 1, Message: "Define user entity"})
	if err != nil {
		t.Fatal(err)
	}
}

func searchLocalIds(t *testing.T, db webdb.WebDb, w context.Context,
	f commitsearch.Filter) []uint64 {
	t.Helper()
	it, err := db.SearchCommits(w, f)
	if err != nil {
		t.Fatal(err)
	}
	return collectLocalIds(t, it)
}

func newSearchTestDb(t *testing.T) (webdb.WebDb, context.Context) {
	t.Helper()
	db := getNewDb(t)
	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW)
	setUpSearchableCommits(t, db, w)
	return db, w
}

func Test_SearchCommits_ReturnsTheRepoCommitsNewestFirst(t *testing.T) {
	db, w := newSearchTestDb(t)

	got := searchLocalIds(t, db, w, commitsearch.NewFilter(searchRepoId, 10))

	if !reflect.DeepEqual(got, []uint64{4, 3, 2, 1}) {
		t.Fatalf("found %v, want [4 3 2 1]", got)
	}
}

func Test_SearchCommits_PaginatesWithTheLimitAndTheCursor(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId, 2)
	firstPage := searchLocalIds(t, db, w, f)
	if !reflect.DeepEqual(firstPage, []uint64{4, 3}) {
		t.Fatalf("first page is %v, want [4 3]", firstPage)
	}

	f.HasAfterCommitId = true
	f.AfterCommitId = firstPage[len(firstPage)-1]
	secondPage := searchLocalIds(t, db, w, f)
	if !reflect.DeepEqual(secondPage, []uint64{2, 1}) {
		t.Fatalf("second page is %v, want [2 1]", secondPage)
	}
}

func Test_SearchCommits_FiltersByState(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId, 10)
	f.State = commitsearch.StatePending
	if got := searchLocalIds(t, db, w, f); !reflect.DeepEqual(got, []uint64{4, 2, 1}) {
		t.Fatalf("pending commits are %v, want [4 2 1]", got)
	}

	f.State = commitsearch.StateSubmitted
	if got := searchLocalIds(t, db, w, f); !reflect.DeepEqual(got, []uint64{3}) {
		t.Fatalf("submitted commits are %v, want [3]", got)
	}
}

func Test_SearchCommits_FiltersWipAndArchivedCommits(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId, 10)
	f.Wip = commitsearch.PresenceRequire
	if got := searchLocalIds(t, db, w, f); !reflect.DeepEqual(got, []uint64{4}) {
		t.Fatalf("wip commits are %v, want [4]", got)
	}

	f = commitsearch.NewFilter(searchRepoId, 10)
	f.Wip = commitsearch.PresenceExclude
	if got := searchLocalIds(t, db, w, f); !reflect.DeepEqual(got, []uint64{3, 2, 1}) {
		t.Fatalf("not wip commits are %v, want [3 2 1]", got)
	}

	f = commitsearch.NewFilter(searchRepoId, 10)
	f.Archived = commitsearch.PresenceRequire
	if got := searchLocalIds(t, db, w, f); !reflect.DeepEqual(got, []uint64{5}) {
		t.Fatalf("archived commits are %v, want [5]", got)
	}
}
