package webdb_test

import (
	"context"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/bugsearch"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"testing"
)

const searchBugRepoId = 11

// Writes the bugs the search tests run against:
//
//	b/1 open,   assignee aang
//	b/2 open,   assignee unassigned
//	b/3 closed, assignee katara
//	b/4 open,   assignee aang
func setUpSearchableBugs(t *testing.T, db webdb.WebDb, w context.Context) (aangId, kataraId int64) {
	t.Helper()
	ids := map[string]int64{}
	for _, username := range []string{"aang", "katara"} {
		id, err := db.CreateUser(w, username+"@twigg.vc", user.UserState_NoSubscription,
			/*isOrganization=*/ false, username, "hash",
			user.Subscription_Solo, 1)
		if err != nil {
			t.Fatal(err)
		}
		ids[username] = id
	}
	aangId, kataraId = ids["aang"], ids["katara"]

	b1, err := db.CreateBug(w, searchBugRepoId, aangId, "Login is broken", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.SetBugAssignee(w, searchBugRepoId, b1.Number, aangId, aangId)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.CreateBug(w, searchBugRepoId, kataraId, "Signup fails", "")
	if err != nil {
		t.Fatal(err)
	}

	b3, err := db.CreateBug(w, searchBugRepoId, aangId, "Slow queue", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.SetBugAssignee(w, searchBugRepoId, b3.Number, aangId, kataraId)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.SetBugStatus(w, searchBugRepoId, b3.Number, aangId, bug.Status_Closed)
	if err != nil {
		t.Fatal(err)
	}

	b4, err := db.CreateBug(w, searchBugRepoId, kataraId, "Crash on save", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.SetBugAssignee(w, searchBugRepoId, b4.Number, kataraId, aangId)
	if err != nil {
		t.Fatal(err)
	}

	// A bug of another repo must never show up
	_, err = db.CreateBug(w, searchBugRepoId+1, aangId, "Slow queue", "")
	if err != nil {
		t.Fatal(err)
	}
	return aangId, kataraId
}

func searchBugNumbers(t *testing.T, db webdb.WebDb, w context.Context,
	f bugsearch.Filter, cursor string, limit int) ([]uint64, string) {
	t.Helper()
	bugs, next, err := db.SearchBugs(w, f, cursor, limit)
	if err != nil {
		t.Fatal(err)
	}
	numbers := []uint64{}
	for _, b := range bugs {
		numbers = append(numbers, b.Number)
	}
	return numbers, next
}

func newBugSearchTestDb(t *testing.T) (webdb.WebDb, context.Context) {
	t.Helper()
	db := getNewDb(t)
	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW)
	return db, w
}

func Test_SearchBugs_MatchesEveryBugWithNoFilter(t *testing.T) {
	db, w := newBugSearchTestDb(t)
	setUpSearchableBugs(t, db, w)

	got, next := searchBugNumbers(t, db, w, bugsearch.NewFilter(searchBugRepoId), "", 10)

	if len(got) != 4 || next != "" {
		t.Fatalf("got %v (next %q), want 4 bugs and no next page", got, next)
	}
}

func Test_SearchBugs_FiltersByStatus(t *testing.T) {
	db, w := newBugSearchTestDb(t)
	setUpSearchableBugs(t, db, w)

	f := bugsearch.NewFilter(searchBugRepoId)
	f.Status = bug.Status_Closed
	got, _ := searchBugNumbers(t, db, w, f, "", 10)

	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("got %v, want [3]", got)
	}
}

func Test_SearchBugs_FiltersByAssignee(t *testing.T) {
	db, w := newBugSearchTestDb(t)
	setUpSearchableBugs(t, db, w)

	f := bugsearch.NewFilter(searchBugRepoId)
	f.AssigneeUsername = "aang"
	got, _ := searchBugNumbers(t, db, w, f, "", 10)

	if len(got) != 2 || got[0] != 4 || got[1] != 1 {
		t.Fatalf("got %v, want [4 1]", got)
	}
}

func Test_SearchBugs_FiltersByUnassigned(t *testing.T) {
	db, w := newBugSearchTestDb(t)
	setUpSearchableBugs(t, db, w)

	f := bugsearch.NewFilter(searchBugRepoId)
	f.AssigneeUnassigned = true
	got, _ := searchBugNumbers(t, db, w, f, "", 10)

	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("got %v, want [2]", got)
	}
}
