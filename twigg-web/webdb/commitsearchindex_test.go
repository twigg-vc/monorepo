package webdb

import (
	"context"
	"monorepo/twigg-web/review"
	"monorepo/twigg/commit"
	"reflect"
	"testing"
	"time"
)

const indexedRepoId = 7

type indexedCommit struct {
	version            uint64
	authorId           int64
	isSubmitted        bool
	createdOnUnixMilli int64
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
			isWip, isArchived
		FROM twigg_commit_search WHERE repoId = ? AND commitId = ?
	`, indexedRepoId, cId).Scan(&c.version, &c.authorId, &c.isSubmitted,
		&c.createdOnUnixMilli, &c.isWip, &c.isArchived)
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
	if got.version != 2 {
		t.Fatalf("indexed v%d, want v2", got.version)
	}
	text := readIndexedText(t, db, w, 1)
	if !reflect.DeepEqual(text, []string{"older", "newer"}) {
		t.Fatalf("indexed text is %v, want [older newer]", text)
	}
}

func readIndexedText(t *testing.T, db WebDb, w context.Context,
	cId commit.LocalId) []string {
	t.Helper()
	rows, err := db.db.s.Query(w, `
		SELECT message FROM commit_search_text
		WHERE repoId = ? AND commitId = ? ORDER BY commitVersion
	`, indexedRepoId, cId)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	messages := []string{}
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return messages
}

func Test_SetCommit_IndexesTheMessageTextOfEveryVersion(t *testing.T) {
	db, w := newIndexTestDb(t)

	for _, c := range []commit.Commit{
		{L: 1, Version: 0, AuthorUserId: 3, Message: "first"},
		{L: 1, Version: 1, AuthorUserId: 3, Message: "second"},
	} {
		if err := db.SetCommit(w, "owner", indexedRepoId, c); err != nil {
			t.Fatal(err)
		}
	}

	got := readIndexedText(t, db, w, 1)
	if !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatalf("indexed text is %v, want [first second]", got)
	}
}

func Test_SetCommit_IndexesTheMessageTextOfAVersionOnce(t *testing.T) {
	db, w := newIndexTestDb(t)

	for range 2 {
		err := db.SetCommit(w, "owner", indexedRepoId,
			commit.Commit{L: 1, Version: 0, AuthorUserId: 3, Message: "only"})
		if err != nil {
			t.Fatal(err)
		}
	}

	got := readIndexedText(t, db, w, 1)
	if !reflect.DeepEqual(got, []string{"only"}) {
		t.Fatalf("indexed text is %v, want [only]", got)
	}
}

func readIndexedReview(t *testing.T, db WebDb, w context.Context,
	cId commit.LocalId) (reviewStatus uint32, reviewerIds []int64) {
	t.Helper()
	err := db.db.s.QueryRow(w, `
		SELECT reviewStatus FROM reviews WHERE repoId = ? AND commitId = ?
	`, indexedRepoId, cId).Scan(&reviewStatus)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.db.s.Query(w, `
		SELECT userId FROM review_reviewers
		WHERE repoId = ? AND commitId = ? ORDER BY userId
	`, indexedRepoId, cId)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var userId int64
		if err := rows.Scan(&userId); err != nil {
			t.Fatal(err)
		}
		reviewerIds = append(reviewerIds, userId)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return reviewStatus, reviewerIds
}

func Test_SetReviewData_IndexesTheStatusAndTheReviewers(t *testing.T) {
	db, w := newIndexTestDb(t)

	err := db.SetReviewData(w, "owner", indexedRepoId, 1, review.Data{
		ReviewStatus:     review.ReviewStatus_Unresolved,
		ReviewersUserIds: []int64{5, 9},
	})
	if err != nil {
		t.Fatal(err)
	}

	gotStatus, gotReviewers := readIndexedReview(t, db, w, 1)
	if gotStatus != uint32(review.ReviewStatus_Unresolved) {
		t.Fatalf("indexed status %d, want %d", gotStatus,
			review.ReviewStatus_Unresolved)
	}
	if !reflect.DeepEqual(gotReviewers, []int64{5, 9}) {
		t.Fatalf("indexed reviewers %v, want [5 9]", gotReviewers)
	}
}

func Test_SetReviewData_ReplacesTheReviewers(t *testing.T) {
	db, w := newIndexTestDb(t)
	err := db.SetReviewData(w, "owner", indexedRepoId, 1, review.Data{
		ReviewersUserIds: []int64{5, 9},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.SetReviewData(w, "owner", indexedRepoId, 1, review.Data{
		ReviewersUserIds: []int64{9, 12},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, gotReviewers := readIndexedReview(t, db, w, 1)
	if !reflect.DeepEqual(gotReviewers, []int64{9, 12}) {
		t.Fatalf("indexed reviewers %v, want [9 12]", gotReviewers)
	}
}
