package webdb_test

import (
	"context"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/bugsearch"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"slices"
	"testing"
	"time"
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
	db.SetNower(mockNow{now: time.UnixMilli(0)}, t)
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

func Test_SearchBugs_FiltersByPriority(t *testing.T) {
	db, w := newBugSearchTestDb(t)
	aangId, _ := setUpSearchableBugs(t, db, w)
	for number, p := range map[uint64]bug.Priority{1: bug.Priority_P0, 2: bug.Priority_P1, 4: bug.Priority_P0} {
		_, _, err := db.SetBugPriority(w, searchBugRepoId, number, aangId, p)
		if err != nil {
			t.Fatal(err)
		}
	}

	f := bugsearch.NewFilter(searchBugRepoId)
	f.HasPriority = true
	f.Priority = bug.Priority_P0
	got, _ := searchBugNumbers(t, db, w, f, "", 10)
	if !slices.Equal(got, []uint64{4, 1}) {
		t.Fatalf("got %v, want [4 1]", got)
	}

	f.Priority = bug.Priority_None
	got, _ = searchBugNumbers(t, db, w, f, "", 10)
	if !slices.Equal(got, []uint64{3}) {
		t.Fatalf("got %v, want [3]", got)
	}
}

// Each case creates one event on bug 1 and checks that it moves from last to first.
func Test_SearchBugs_OrdersByLastUpdate(t *testing.T) {
	for _, test := range []struct {
		name        string
		createEvent func(webdb.WebDb, context.Context, int64) error
	}{
		{"title", func(db webdb.WebDb, w context.Context, authorId int64) error {
			_, _, err := db.EditBugTitle(w, searchBugRepoId, 1, authorId, "Updated title")
			return err
		}},
		{"description", func(db webdb.WebDb, w context.Context, authorId int64) error {
			_, _, err := db.EditBugDescription(w, searchBugRepoId, 1, authorId, "Updated body")
			return err
		}},
		{"comment", func(db webdb.WebDb, w context.Context, authorId int64) error {
			_, _, err := db.AddBugComment(w, searchBugRepoId, 1, authorId, "An update")
			return err
		}},
		{"status", func(db webdb.WebDb, w context.Context, authorId int64) error {
			_, _, err := db.SetBugStatus(w, searchBugRepoId, 1, authorId, bug.Status_Closed)
			return err
		}},
		{"assignment", func(db webdb.WebDb, w context.Context, authorId int64) error {
			_, _, err := db.SetBugAssignee(w, searchBugRepoId, 1, authorId, 0)
			return err
		}},
		{"submitted commit", func(db webdb.WebDb, w context.Context, authorId int64) error {
			_, _, err := db.AddBugSubmittedCommit(w, searchBugRepoId, 1, authorId, 42)
			return err
		}},
		{"closing commit", func(db webdb.WebDb, w context.Context, authorId int64) error {
			return db.CloseBugIfNotClosed(w, searchBugRepoId, 1, authorId, 42)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, w := newBugSearchTestDb(t)
			authorId, _ := setUpSearchableBugs(t, db, w)
			f := bugsearch.NewFilter(searchBugRepoId)
			before, _ := searchBugNumbers(t, db, w, f, "", 10)
			if position := slices.Index(before, 1); position < 0 || position != len(before)-1 {
				t.Fatalf("before event: got %v, want bug 1 last", before)
			}

			db.SetNower(mockNow{now: time.UnixMilli(200)}, t)
			if err := test.createEvent(db, w, authorId); err != nil {
				t.Fatal(err)
			}

			after, _ := searchBugNumbers(t, db, w, f, "", 10)
			if slices.Index(after, 1) != 0 {
				t.Fatalf("after event: got %v, want bug 1 first", after)
			}
		})
	}
}

func Test_SearchBugs_PaginatesByLastUpdate(t *testing.T) {
	db, w := newBugSearchTestDb(t)
	authorId, _ := setUpSearchableBugs(t, db, w)
	db.SetNower(mockNow{now: time.UnixMilli(200)}, t)
	_, _, err := db.EditBugTitle(w, searchBugRepoId, 1, authorId, "Updated title")
	if err != nil {
		t.Fatal(err)
	}

	// Bug 1 moves first; the cursor must continue through bugs with tied timestamps.
	f := bugsearch.NewFilter(searchBugRepoId)
	first, cursor := searchBugNumbers(t, db, w, f, "", 2)
	if !slices.Equal(first, []uint64{1, 4}) || cursor == "" {
		t.Fatalf("first page: got %v (cursor %q), want [1 4] and a cursor", first, cursor)
	}
	second, next := searchBugNumbers(t, db, w, f, cursor, 2)
	if !slices.Equal(second, []uint64{3, 2}) || next != "" {
		t.Fatalf("second page: got %v (next %q), want [3 2] and no next page", second, next)
	}
}
