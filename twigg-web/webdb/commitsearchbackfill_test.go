package webdb

import (
	"context"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
)

func countIndexedCommits(t *testing.T, db WebDb, w context.Context) int {
	t.Helper()
	var n int
	err := db.db.s.QueryRow(w,
		`SELECT COUNT(*) FROM twigg_commit_search`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func Test_IndexCommitsForSearch_IndexesEveryCommitInBatches(t *testing.T) {
	db, w := newIndexTestDb(t)
	for _, c := range []struct {
		repoId   uint64
		commitId commit.LocalId
	}{
		{7, 1},
		{7, 2},
		{8, 1},
	} {
		err := db.SetCommit(w, "owner", c.repoId,
			commit.Commit{L: c.commitId, AuthorUserId: 3, Message: "msg"})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Makes the commits look like they predate the index.
	_, err := db.db.s.Exec(w, `DELETE FROM twigg_commit_search`)
	if err != nil {
		t.Fatal(err)
	}

	var cursor string
	var done bool
	var batches int
	for {
		cursor, done, err = db.IndexCommitsForSearch(w, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		batches++
	}

	if batches != 2 {
		t.Fatalf("swept in %d batches, want 2", batches)
	}
	if got := countIndexedCommits(t, db, w); got != 3 {
		t.Fatalf("indexed %d commits, want 3", got)
	}
}

func Test_IndexCommitsForSearch_IndexesTheTextOfEveryVersion(t *testing.T) {
	db, w := newIndexTestDb(t)
	for _, c := range []commit.Commit{
		{L: 1, Version: 0, AuthorUserId: 3, Message: "first try"},
		{L: 1, Version: 1, AuthorUserId: 3, Message: "second try"},
	} {
		if err := db.SetCommit(w, "owner", indexedRepoId, c); err != nil {
			t.Fatal(err)
		}
	}
	// Makes the commit look like it predates the index.
	_, err := db.db.s.Exec(w, `DELETE FROM commit_search_text`)
	if err != nil {
		t.Fatal(err)
	}

	var cursor string
	var done bool
	for !done {
		cursor, done, err = db.IndexCommitsForSearch(w, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
	}

	got := readIndexedText(t, db, w, 1)
	if !reflect.DeepEqual(got, []string{"first try", "second try"}) {
		t.Fatalf("indexed text is %v, want [first try, second try]", got)
	}
}

func Test_CommitSearchIndexCursor_IsEmptyUntilOneIsSaved(t *testing.T) {
	db, w := newIndexTestDb(t)

	got, err := db.GetCommitSearchIndexCursor(w)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("the cursor of a database with no sweep is %q, want none", got)
	}
}

func Test_CommitSearchIndexCursor_ReadsBackTheLastSavedOne(t *testing.T) {
	db, w := newIndexTestDb(t)
	for i := 1; i <= 4; i++ {
		err := db.SetCommit(w, "owner", 7,
			commit.Commit{L: uint64(i), AuthorUserId: 3, Message: "msg"})
		if err != nil {
			t.Fatal(err)
		}
	}
	first, _, err := db.IndexCommitsForSearch(w, "", 2)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.SetCommitSearchIndexCursor(w, first); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetCommitSearchIndexCursor(w)
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatalf("the saved cursor is %q, want %q", got, first)
	}
	// A second sweep replaces the saved cursor instead of adding a row.
	second, _, err := db.IndexCommitsForSearch(w, first, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetCommitSearchIndexCursor(w, second); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetCommitSearchIndexCursor(w)
	if err != nil {
		t.Fatal(err)
	}
	if got != second {
		t.Fatalf("the saved cursor is %q, want %q", got, second)
	}
}
