package webdb

import (
	"context"
	"monorepo/twigg/commit"
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

	var cursor IndexCommitsForSearchCursor
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
