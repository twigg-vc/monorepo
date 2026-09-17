package webdb

import (
	"context"
	"monorepo/twigg/commit"
	"testing"
	"time"
)

const indexedRepoId = 7

type indexedCommit struct {
	version            uint64
	authorId           int64
	isSubmitted        bool
	createdOnUnixMilli int64
	message            string
	isWip              bool
	isArchived         bool
}

func newIndexTestDb(t *testing.T) (WebDb, context.Context) {
	t.Helper()
	db, closeDb, err := NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	w, closeW, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeW)
	return db, w
}

func readIndexedCommit(t *testing.T, db WebDb, w context.Context,
	cId commit.LocalId) indexedCommit {
	t.Helper()
	var c indexedCommit
	err := db.db.s.QueryRow(w, `
		SELECT commitVersion, authorId, isSubmitted, createdOnUnixMilli,
			message, isWip, isArchived
		FROM twigg_commit_search WHERE repoId = ? AND commitId = ?
	`, indexedRepoId, cId).Scan(&c.version, &c.authorId, &c.isSubmitted,
		&c.createdOnUnixMilli, &c.message, &c.isWip, &c.isArchived)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func Test_SetCommit_IndexesTheSearchableColumns(t *testing.T) {
	db, w := newIndexTestDb(t)
	createdOn := time.UnixMilli(1_700_000_000_000).UTC()

	err := db.SetCommit(w, "owner", indexedRepoId, commit.Commit{
		L: 1, Version: 0, AuthorUserId: 3, IsSubmitted: true,
		CreatedOn: createdOn, Message: "wip: refactor the queue",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := readIndexedCommit(t, db, w, 1)
	want := indexedCommit{
		version: 0, authorId: 3, isSubmitted: true,
		createdOnUnixMilli: createdOn.UnixMilli(),
		message:            "wip: refactor the queue",
		isWip:              true, isArchived: false,
	}
	if got != want {
		t.Fatalf("indexed %+v, want %+v", got, want)
	}
}

func Test_SetCommit_IgnoresAnOlderCommitVersion(t *testing.T) {
	db, w := newIndexTestDb(t)
	newer := commit.Commit{L: 1, Version: 2, AuthorUserId: 3, Message: "newer"}
	older := commit.Commit{L: 1, Version: 1, AuthorUserId: 3, Message: "older"}

	err := db.SetCommit(w, "owner", indexedRepoId, newer)
	if err != nil {
		t.Fatal(err)
	}
	err = db.SetCommit(w, "owner", indexedRepoId, older)
	if err != nil {
		t.Fatal(err)
	}

	got := readIndexedCommit(t, db, w, 1)
	if got.version != 2 || got.message != "newer" {
		t.Fatalf("indexed v%d %q, want v2 \"newer\"", got.version, got.message)
	}
}
