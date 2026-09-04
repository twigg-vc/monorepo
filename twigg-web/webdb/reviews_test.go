package webdb_test

import (
	"errors"
	"monorepo/base/iterator"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/webdb"
	"reflect"
	"slices"
	"testing"
	"time"
)

// webdb doesn't own the thread type definitions; it treats them as opaque
// uint32 values, so the tests just pick some.
const (
	commentThreadType    = uint32(1)
	addLgtmThreadType    = uint32(3)
	removeLgtmThreadType = uint32(4)
)

func TestHasReviewAndCreateReviewIfNotExists(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(1)
	const commitId = uint64(7)

	has, err := b.HasReview(w, repoId, commitId)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("should not have review before creation")
	}

	if err := b.CreateReviewIfNotExists(w, repoId, commitId); err != nil {
		t.Fatal(err)
	}
	has, err = b.HasReview(w, repoId, commitId)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("should have review after creation")
	}

	// Creating again is a no-op
	if err := b.CreateReviewIfNotExists(w, repoId, commitId); err != nil {
		t.Fatal(err)
	}

	// Other repo/commit are unaffected
	has, err = b.HasReview(w, repoId, commitId+1)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("other commit should not have review")
	}
	has, err = b.HasReview(w, repoId+1, commitId)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("other repo should not have review")
	}
}

func TestCreateReviewThreadAndGetReviewThreadIds(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(1)
	const commitId = uint64(7)
	const authorId = int64(42)

	// Ids start at 1 and increase by 1, even across repos/commits
	th1, err := b.CreateReviewThread(w, repoId, commitId, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	if th1 != 1 {
		t.Fatalf("expected first thread id 1, got %d", th1)
	}
	th2, err := b.CreateReviewThread(w, repoId, commitId, authorId, addLgtmThreadType)
	if err != nil {
		t.Fatal(err)
	}
	if th2 != 2 {
		t.Fatalf("expected second thread id 2, got %d", th2)
	}
	th3, err := b.CreateReviewThread(w, repoId, commitId+1, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	if th3 != 3 {
		t.Fatalf("expected third thread id 3, got %d", th3)
	}
	th4, err := b.CreateReviewThread(w, repoId+1, commitId, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	if th4 != 4 {
		t.Fatalf("expected fourth thread id 4, got %d", th4)
	}

	// GetReviewThreadIds only returns the threads of the repo/commit
	it, err := b.GetReviewThreadIds(w, repoId, commitId)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int64{th1, th2}) {
		t.Fatalf("wrong thread ids\n got: %v\nexpected: %v", got, []int64{th1, th2})
	}

	// No threads -> empty result
	it, err = b.GetReviewThreadIds(w, repoId+99, commitId)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no thread ids, got %v", got)
	}
}

func TestGetReviewUserLastLgtmThreadId(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(1)
	const commitId = uint64(7)
	const userId = int64(42)

	// Not found before any LGTM thread
	_, isNotFoundErr, err := b.GetReviewUserLastLgtmThreadId(w, repoId, commitId,
		userId, addLgtmThreadType, removeLgtmThreadType)
	if !isNotFoundErr {
		t.Fatal("expected isNotFoundErr")
	}
	if !errors.Is(err, webdb.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Comment threads of the user don't count as LGTM threads
	_, err = b.CreateReviewThread(w, repoId, commitId, userId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	_, isNotFoundErr, err = b.GetReviewUserLastLgtmThreadId(w, repoId, commitId,
		userId, addLgtmThreadType, removeLgtmThreadType)
	if !isNotFoundErr {
		t.Fatal("comment threads should not count as LGTM threads")
	}

	// Add and remove LGTM threads; the latest one wins
	addId, err := b.CreateReviewThread(w, repoId, commitId, userId, addLgtmThreadType)
	if err != nil {
		t.Fatal(err)
	}
	got, isNotFoundErr, err := b.GetReviewUserLastLgtmThreadId(w, repoId, commitId,
		userId, addLgtmThreadType, removeLgtmThreadType)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got != addId {
		t.Fatalf("expected last LGTM thread %d, got %d", addId, got)
	}

	removeId, err := b.CreateReviewThread(w, repoId, commitId, userId, removeLgtmThreadType)
	if err != nil {
		t.Fatal(err)
	}
	got, isNotFoundErr, err = b.GetReviewUserLastLgtmThreadId(w, repoId, commitId,
		userId, addLgtmThreadType, removeLgtmThreadType)
	if err != nil || isNotFoundErr {
		t.Fatal(err)
	}
	if got != removeId {
		t.Fatalf("expected last LGTM thread %d, got %d", removeId, got)
	}

	// Other users' LGTMs are not visible for this user
	_, isNotFoundErr, _ = b.GetReviewUserLastLgtmThreadId(w, repoId, commitId,
		userId+1, addLgtmThreadType, removeLgtmThreadType)
	if !isNotFoundErr {
		t.Fatal("other user should have no LGTM thread")
	}

	// Nor are this user's LGTMs on another commit
	_, isNotFoundErr, _ = b.GetReviewUserLastLgtmThreadId(w, repoId, commitId+1,
		userId, addLgtmThreadType, removeLgtmThreadType)
	if !isNotFoundErr {
		t.Fatal("other commit should have no LGTM thread")
	}
}

func TestGetReviewLgtmAuthorIds(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(1)
	const commitId = uint64(7)

	// user1: add -> odd count -> holds an LGTM
	const user1 = int64(1)
	// user2: add, remove -> even count -> no LGTM
	const user2 = int64(2)
	// user3: add, remove, add -> odd count -> holds an LGTM
	const user3 = int64(3)
	events := []struct {
		userId     int64
		threadType uint32
	}{
		{user1, addLgtmThreadType},
		{user2, addLgtmThreadType},
		{user2, removeLgtmThreadType},
		{user3, addLgtmThreadType},
		{user3, removeLgtmThreadType},
		{user3, addLgtmThreadType},
	}
	for _, e := range events {
		_, err := b.CreateReviewThread(w, repoId, commitId, e.userId, e.threadType)
		if err != nil {
			t.Fatal(err)
		}
	}
	// user2's comment threads don't make it an LGTM author
	_, err = b.CreateReviewThread(w, repoId, commitId, user2, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	// user2's LGTM on another commit doesn't count either
	_, err = b.CreateReviewThread(w, repoId, commitId+1, user2, addLgtmThreadType)
	if err != nil {
		t.Fatal(err)
	}

	it, err := b.GetReviewLgtmAuthorIds(w, repoId, commitId,
		addLgtmThreadType, removeLgtmThreadType)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []int64{user1, user3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetReviewLgtmAuthorIds() = %v, want %v", got, want)
	}
}

func TestCreateReviewCommentAndGetReviewCommentIds(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const repoId = uint64(1)
	const commitId = uint64(7)
	const authorId = int64(42)

	th1, err := b.CreateReviewThread(w, repoId, commitId, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	th2, err := b.CreateReviewThread(w, repoId, commitId, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}

	// Ids start at 1 and increase by 1, even across threads
	cm1, err := b.CreateReviewComment(w, repoId, commitId, th1, authorId)
	if err != nil {
		t.Fatal(err)
	}
	if cm1 != 1 {
		t.Fatalf("expected first comment id 1, got %d", cm1)
	}
	cm2, err := b.CreateReviewComment(w, repoId, commitId, th1, authorId+1)
	if err != nil {
		t.Fatal(err)
	}
	if cm2 != 2 {
		t.Fatalf("expected second comment id 2, got %d", cm2)
	}
	cm3, err := b.CreateReviewComment(w, repoId, commitId, th2, authorId)
	if err != nil {
		t.Fatal(err)
	}
	if cm3 != 3 {
		t.Fatalf("expected third comment id 3, got %d", cm3)
	}

	// GetReviewCommentIds only returns the comments of the thread
	it, err := b.GetReviewCommentIds(w, repoId, commitId, th1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int64{cm1, cm2}) {
		t.Fatalf("wrong comment ids\n got: %v\nexpected: %v", got, []int64{cm1, cm2})
	}

	// No comments -> empty result
	it, err = b.GetReviewCommentIds(w, repoId, commitId, th2+99)
	if err != nil {
		t.Fatal(err)
	}
	got, err = iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no comment ids, got %v", got)
	}
}

func TestSetAndGetReviewData(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const quotaOwner = "owner"
	const repoId = uint64(1)
	const commitId = uint64(7)

	// Get before any set should err
	_, err = b.GetReviewData(w, repoId, commitId)
	if err == nil {
		t.Fatal("should err for non existing review data")
	}

	d := review.Data{
		Description:                 "my description",
		IsWIP:                       true,
		ReviewStatus:                review.ReviewStatus_Unresolved,
		ReviewStatusLgtmCount:       2,
		ReviewStatusUnresolvedCount: 1,
		ReviewersUserIds:            []int64{10, 20},
	}
	if err := b.SetReviewData(w, quotaOwner, repoId, commitId, d); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetReviewData(w, repoId, commitId)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, d) {
		t.Fatalf("wrong data\n got: %#v\nexpected: %#v", got, d)
	}

	// Setting again overwrites
	d.Description = "new description"
	d.IsArchived = true
	if err := b.SetReviewData(w, quotaOwner, repoId, commitId, d); err != nil {
		t.Fatal(err)
	}
	got, err = b.GetReviewData(w, repoId, commitId)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, d) {
		t.Fatalf("wrong data after overwrite\n got: %#v\nexpected: %#v", got, d)
	}
}

func TestSetAndGetReviewThread(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const quotaOwner = "owner"
	const repoId = uint64(1)
	const commitId = uint64(7)
	const authorId = int64(42)

	threadId, err := b.CreateReviewThread(w, repoId, commitId, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}

	// Get before any set should err
	_, err = b.GetReviewThread(w, threadId)
	if err == nil {
		t.Fatal("should err for thread without contents")
	}

	th := review.Thread{
		Id:            threadId,
		Type:          review.ThreadType_CommentsOnFileOnCommitVersion,
		AuthorUserId:  authorId,
		CommitVersion: 3,
		IsResolved:    false,
		Filename:      "file.txt",
		CreatedOn:     time.Now(),
	}
	if err := b.SetReviewThread(w, quotaOwner, threadId, th); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetReviewThread(w, threadId)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedOn.IsZero() {
		t.Fatal("CreatedOn should be set")
	}
	got.CreatedOn = time.Time{}
	th.CreatedOn = time.Time{}
	if got != th {
		t.Fatalf("wrong thread\n got: %#v\nexpected: %#v", got, th)
	}

	// Setting again overwrites
	th.IsResolved = true
	th.CreatedOn = time.Now()
	if err := b.SetReviewThread(w, quotaOwner, threadId, th); err != nil {
		t.Fatal(err)
	}
	got, err = b.GetReviewThread(w, threadId)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedOn.IsZero() {
		t.Fatal("CreatedOn should be set after overwrite")
	}
	got.CreatedOn = time.Time{}
	th.CreatedOn = time.Time{}
	if got != th {
		t.Fatalf("wrong thread after overwrite\n got: %#v\nexpected: %#v", got, th)
	}
}

func TestSetAndGetReviewComment(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const quotaOwner = "owner"
	const repoId = uint64(1)
	const commitId = uint64(7)
	const authorId = int64(42)

	threadId, err := b.CreateReviewThread(w, repoId, commitId, authorId, commentThreadType)
	if err != nil {
		t.Fatal(err)
	}
	commentId, err := b.CreateReviewComment(w, repoId, commitId, threadId, authorId)
	if err != nil {
		t.Fatal(err)
	}

	// Get before any set should err
	_, err = b.GetReviewComment(w, commentId)
	if err == nil {
		t.Fatal("should err for comment without contents")
	}

	cm := review.Comment{
		ThreadId:     threadId,
		AuthorUserId: authorId,
		Text:         "some comment",
		T:            time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC),
	}
	if err := b.SetReviewComment(w, quotaOwner, commentId, cm); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetReviewComment(w, commentId)
	if err != nil {
		t.Fatal(err)
	}
	if got.ThreadId != cm.ThreadId ||
		got.AuthorUserId != cm.AuthorUserId ||
		got.Text != cm.Text ||
		!got.T.Equal(cm.T) {
		t.Fatalf("wrong comment\n got: %#v\nexpected: %#v", got, cm)
	}
}