package webdb_test

import (
	"context"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
)

const searchRepoId = 7

// Writes the commits the search tests run against:
//
//	c1 pending, author aang, reviewed by aang, ready
//	c2 pending, author katara, reviewed by aang and katara, unresolved
//	c3 submitted, author aang
//	c4 pending WIP, author katara
//	c5 pending archived, author aang
func setUpSearchableUsers(t *testing.T, db webdb.WebDb, w context.Context) {
	t.Helper()
	for _, username := range []string{"aang", "katara"} {
		_, err := db.CreateUser(w, username+"@twigg.vc", user.UserState_NoSubscription,
			/*isOrganization=*/ false, username, "hash",
			user.Subscription_Solo, 1)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func setUpSearchableCommits(t *testing.T, db webdb.WebDb, w context.Context) {
	t.Helper()
	setUpSearchableUsers(t, db, w)
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
	reviews := map[commit.LocalId]review.Data{
		1: {
			ReviewStatus:     review.ReviewStatus_Ready,
			ReviewersUserIds: []int64{1},
		},
		2: {
			ReviewStatus:     review.ReviewStatus_Unresolved,
			ReviewersUserIds: []int64{1, 2},
		},
	}
	for cId, d := range reviews {
		if err := db.SetReviewData(w, "owner", searchRepoId, cId, d); err != nil {
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
	// The last page says it is the last, so nothing asks for another one
	if next2 != "" {
		t.Fatalf("the last page points at %q, want it to point at nothing", next2)
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
	// The last page says it is the last, so nothing asks for another one
	if next2 != "" {
		t.Fatalf("the last page points at %q, want it to point at nothing", next2)
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
	f.AuthorUsername = "aang"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{3, 1}) {
		t.Fatalf("commits of aang are %v, want [3 1]", got)
	}

	f.AuthorUsername = "katara"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{4, 2}) {
		t.Fatalf("commits of katara are %v, want [4 2]", got)
	}
}

func Test_SearchCommits_FiltersByReviewer(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.ReviewerUsername = "aang"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{2, 1}) {
		t.Fatalf("commits reviewed by aang are %v, want [2 1]", got)
	}

	f.ReviewerUsername = "katara"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{2}) {
		t.Fatalf("commits reviewed by katara are %v, want [2]", got)
	}

	f.ReviewerUsername = "nobody"
	if got, _ := searchLocalIds(t, db, w, f, "", 100); len(got) != 0 {
		t.Fatalf("commits reviewed by nobody are %v, want none", got)
	}
}

func Test_SearchCommits_FiltersByReviewStatus(t *testing.T) {
	db, w := newSearchTestDb(t)

	f := commitsearch.NewFilter(searchRepoId)
	f.HasReviewStatus = true
	f.ReviewStatus = review.ReviewStatus_Ready
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{1}) {
		t.Fatalf("ready commits are %v, want [1]", got)
	}

	f.ReviewStatus = review.ReviewStatus_Unresolved
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{2}) {
		t.Fatalf("unresolved commits are %v, want [2]", got)
	}

	// c4 has no reviews row at all, so it is missing a LGTM.
	f.ReviewStatus = review.ReviewStatus_MissingLgtm
	if got, _ := searchLocalIds(t, db, w, f, "", 100); !reflect.DeepEqual(got, []uint64{4}) {
		t.Fatalf("commits missing a lgtm are %v, want [4]", got)
	}
}

// A status that needs a reviews row is searched through the reviews, so its
// pages are cut by them instead of by the commits.
func Test_SearchCommits_PaginatesAReviewStatusSearch(t *testing.T) {
	db := getNewDb(t)
	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW)
	setUpSearchableUsers(t, db, w)
	for i := 1; i <= 3; i++ {
		c := commit.Commit{L: uint64(i), AuthorUserId: 1, Message: "a commit"}
		if err := db.SetCommit(w, "owner", searchRepoId, c); err != nil {
			t.Fatal(err)
		}
		err := db.SetReviewData(w, "owner", searchRepoId, commit.LocalId(i),
			review.Data{ReviewStatus: review.ReviewStatus_Unresolved})
		if err != nil {
			t.Fatal(err)
		}
	}
	f := commitsearch.NewFilter(searchRepoId)
	f.HasReviewStatus = true
	f.ReviewStatus = review.ReviewStatus_Unresolved

	got, next := searchLocalIds(t, db, w, f, "", 2)
	if !reflect.DeepEqual(got, []uint64{3, 2}) {
		t.Fatalf("the first page is %v, want [3 2]", got)
	}
	got2, next2 := searchLocalIds(t, db, w, f, next, 2)
	if !reflect.DeepEqual(got2, []uint64{1}) {
		t.Fatalf("the second page is %v, want [1]", got2)
	}
	if next2 != "" {
		t.Fatalf("the last page points at %q, want it to point at nothing", next2)
	}
}