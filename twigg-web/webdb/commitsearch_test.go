package webdb_test

import (
	"context"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
)

const searchRepoId = 7

// Writes the commits the search tests run against:
//
//	c1 pending, author 1, reviewed by 1
//	c2 pending, author 2, reviewed by 1 and 2
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
	reviewers := map[commit.LocalId][]int64{1: {1}, 2: {1, 2}}
	for cId, userIds := range reviewers {
		err := db.SetReviewData(w, "owner", searchRepoId, cId, review.Data{
			ReviewersUserIds: userIds,
		})
		if err != nil {
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
	f commitsearch.Filter, cursor string, limit int) ([]uint64, string) {
	t.Helper()
	commits, next, err := db.SearchCommits(w, f, cursor, limit)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uint64{}
	for _, c := range commits {
		ids = append(ids, c.L)
	}
	return ids, next
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

func Test_SearchCommits_ReturnsTheRepoCommitsNewestFirstAndPaginateCursor(t *testing.T) {
	db, w := newSearchTestDb(t)

	got, next := searchLocalIds(t, db, w, commitsearch.NewFilter(searchRepoId), "", 2)
	if !reflect.DeepEqual(got, []uint64{4, 3}) {
		t.Fatalf("found %v, want [4 3]", got)
	}
	got2, next2 := searchLocalIds(t, db, w, commitsearch.NewFilter(searchRepoId), next, 2)
	if !reflect.DeepEqual(got2, []uint64{2, 1}) {
		t.Fatalf("found %v, want [2 1]", got2)
	}
	got3, _ := searchLocalIds(t, db, w, commitsearch.NewFilter(searchRepoId), next2, 2)
	if len(got3) != 0 {
		t.Fatalf("found %v, want empty slice", got)
	}
}

func Test_SearchCommits_FiltersByState(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.State = commitsearch.StatePending
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{4, 2, 1}) {
		t.Fatalf("pending commits are %v, want [4 2 1]", got)
	}

	f.State = commitsearch.StateSubmitted
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{3}) {
		t.Fatalf("submitted commits are %v, want [3]", got)
	}
}

func Test_SearchCommits_FiltersWipAndArchivedCommits(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.Wip = commitsearch.PresenceRequire
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{4}) {
		t.Fatalf("wip commits are %v, want [4]", got)
	}

	f = commitsearch.NewFilter(searchRepoId)
	f.Wip = commitsearch.PresenceExclude
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{3, 2, 1}) {
		t.Fatalf("not wip commits are %v, want [3 2 1]", got)
	}

	f = commitsearch.NewFilter(searchRepoId)
	f.Archived = commitsearch.PresenceRequire
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{5}) {
		t.Fatalf("archived commits are %v, want [5]", got)
	}
}

func Test_SearchCommits_FiltersByMessageText(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.Message = "user"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{3, 2, 1}) {
		t.Fatalf("commits about the user are %v, want [3 2 1]", got)
	}

	f.Message = "methods USER"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{2}) {
		t.Fatalf("commits about the user methods are %v, want [2]", got)
	}
}

func Test_SearchCommits_PaginatesATextSearchWithTheTextIndexRow(t *testing.T) {
	db, w := newSearchTestDb(t)
	f := commitsearch.NewFilter(searchRepoId)
	f.Message = "user"

	got, next := searchLocalIds(t, db, w, f, "", 2)
	if !reflect.DeepEqual(got, []uint64{3, 2}) {
		t.Fatalf("first page is %v, want [3 2]", got)
	}

	got2, next2 := searchLocalIds(t, db, w, f, next, 2)
	if !reflect.DeepEqual(got2, []uint64{1}) {
		t.Fatalf("second page is %v, want [1]", got2)
	}

	got3, _ := searchLocalIds(t, db, w, f, next2, 2)
	if len(got3) != 0 {
		t.Fatalf("last page is %v, want it empty", got3)
	}
}

func Test_SearchCommits_MatchesTheSearchOperatorsLiterally(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	for _, text := range []string{`AND`, `OR NOT`, `"`, `(`, `user*`, `^`, `   `} {
		f.Message = text
		if _, _, err := db.SearchCommits(w, f, "", 100); err != nil {
			t.Fatalf("searching %q failed: %s", text, err)
		}
	}
}

func Test_SearchCommits_FiltersByAuthor(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.AuthorId = 1
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{3, 1}) {
		t.Fatalf("commits of author 1 are %v, want [3 1]", got)
	}

	f.AuthorId = 2
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{4, 2}) {
		t.Fatalf("commits of author 2 are %v, want [4 2]", got)
	}
}

func Test_SearchCommits_FiltersByReviewer(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.ReviewerId = 1
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{2, 1}) {
		t.Fatalf("commits reviewed by 1 are %v, want [2 1]", got)
	}

	f.ReviewerId = 2
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{2}) {
		t.Fatalf("commits reviewed by 2 are %v, want [2]", got)
	}

	f.ReviewerId = 3
	if got, _ := searchLocalIds(t, db, w, f, "", 100); len(got) != 0 {
		t.Fatalf("commits reviewed by 3 are %v, want none", got)
	}
}
