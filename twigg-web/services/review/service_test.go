package review

import (
	"context"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webdb"
	"reflect"
	"sort"
	"testing"
	"time"
)

type fakeOwners struct {
	ok     bool
	err    error
	called bool
}

func (f *fakeOwners) OwnersLgmtIsOk(
	repoId uint64,
	commitId uint64,
	usersWhoLgtmd []string,
	cIdToReadOwners uint64,
	supremeLeaders []string,
	r context.Context,
) (bool, error) {
	f.called = true
	return f.ok, f.err
}

func TestSetAndGetData(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	_, isNotFoundErr, err := s.GetData(w, 1, 0, false, uint64(0), []string{})
	if err == nil {
		t.Fatal("should err")
	}
	if !isNotFoundErr {
		t.Fatal("err should be isNotFoundErr")
	}
	err = s.SetDescription(w,
		"owner", 1, 1, "blabla" /*createIfNeeded*/, true)
	if err != nil {
		t.Fatal(err)
	}

	gotD, isNotFoundErr, err := s.GetData(w, 1, 1, false, uint64(0), []string{})
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("should not isNotFoundErr")
	}
	expectedD := review.Data{
		Description:  "blabla",
		ReviewStatus: review.ReviewStatus_MissingLgtm,
	}

	if !reflect.DeepEqual(gotD, expectedD) {
		t.Fatalf("wrong data\n got: %#v\nexpected: %#v", gotD, expectedD)
	}
}
func TestSetDescriptionButNotCreateIfNeeded(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	err = s.SetDescription(w,
		"owner", 1, 1, "blabla" /*createIfNeeded*/, false)
	if err == nil {
		t.Fatal("should err")
	}

	_, isNotFoundErr, err := s.GetData(w, 1, 1, false, uint64(0), []string{})
	if err == nil {
		t.Fatal(err)
	}
	if !isNotFoundErr {
		t.Fatal("should err should be not found")
	}
}

func TestSetDescriptionForNoLGTM(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	commitId := uint64(99)
	userId := int64(420)

	// Add an LGTM
	_, err = s.AddLgtm(w, "owner", 1, commitId, 0, userId)
	if err != nil {
		t.Fatal(err)
	}

	// Set description
	err = s.SetDescription(w,
		"owner", 1, commitId, "blabla" /*createIfNeeded*/, false)
	if err != nil {
		t.Fatal("should err")
	}

	// Get data
	gotD, isNotFoundErr, err := s.GetData(w, 1, commitId, false, uint64(0), []string{})
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundErr {
		t.Fatal("should not isNotFoundErr")
	}
	expectedD := review.Data{
		Description:           "blabla",
		ReviewStatus:          review.ReviewStatus_Ready,
		ReviewStatusLgtmCount: 1,
	}
	// Check data
	if !reflect.DeepEqual(gotD, expectedD) {
		t.Fatalf("wrong data\n got: %#v\nexpected: %#v", gotD, expectedD)
	}
}

func assertRecentAndClearCreatedOn(t *testing.T, th *review.Thread) {
	t.Helper()
	if th.CreatedOn.IsZero() {
		t.Fatal("CreatedOn should be set")
	}
	if time.Since(th.CreatedOn) > time.Minute || time.Since(th.CreatedOn) < 0 {
		t.Fatalf("CreatedOn should be recent, got %v", th.CreatedOn)
	}
	th.CreatedOn = time.Time{}
}

func TestCreateThread(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const userId = 77
	const commitId = 99
	const quotaOwner = "owner"
	const repoId = 100

	th, err := s.CreateThread(w,
		quotaOwner, repoId, commitId, 10, "file.txt",
		/*line=*/ 3, userId, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	expected := review.Thread{
		Id:            1,
		Type:          review.ThreadType_CommentsOnFileOnCommitVersion,
		AuthorUserId:  userId,
		CommitVersion: 10,
		Filename:      "file.txt",
		Line:          3,
		IsResolved:    true,
	}
	assertRecentAndClearCreatedOn(t, &th)
	if expected != th {
		t.Fatal("wrong thread")
	}
	if !expected.IsInline() {
		t.Fatal("thread anchored to line 1 should be inline")
	}
	notInline := expected
	notInline.Filename = ""
	notInline.Line = 0
	if notInline.IsInline() {
		t.Fatal("thread anchored to line 0 should be inline")
	}

	_, err = s.GetThread(w, 8888)
	if err == nil {
		t.Fatal("should err for non existing")
	}
	got, err := s.GetThread(w, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertRecentAndClearCreatedOn(t, &got)
	if got != expected {
		t.Fatal("wrong result with GetThread")
	}

	th2, err := s.CreateDiscussionThread(w,
		quotaOwner, repoId, commitId, 12, userId, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	expected2 := review.Thread{
		Type:          review.ThreadType_CommentsOnCommitVersion,
		AuthorUserId:  userId,
		Id:            2,
		CommitVersion: 12,
		Filename:      "",
		IsResolved:    true,
	}
	assertRecentAndClearCreatedOn(t, &th2)
	if expected2 != th2 {
		t.Fatal("wrong thread")
	}
	if expected2.IsInline() {
		t.Fatal("thread anchored to line 0 should not be inline")
	}

	th3, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 14, userId)
	if err != nil {
		t.Fatal(err)
	}
	expected3 := review.Thread{
		Type:          review.ThreadType_AddLGTM,
		AuthorUserId:  userId,
		Id:            3,
		CommitVersion: 14,
		IsResolved:    true,
		IsLgtm:        true,
	}
	assertRecentAndClearCreatedOn(t, &th3)
	if expected3 != th3 {
		t.Fatal("wrong thread")
	}
	if expected3.IsInline() {
		t.Fatal("thread anchored to line 0 should not be inline")
	}

	th4, err := s.RemoveLastLgtm(w, quotaOwner, repoId, commitId, userId)
	if err != nil {
		t.Fatal(err)
	}
	expected4 := review.Thread{
		Type:          review.ThreadType_RemoveLGTM,
		AuthorUserId:  userId,
		Id:            4,
		CommitVersion: 14,
		IsResolved:    true,
		IsLgtm:        false,
	}
	assertRecentAndClearCreatedOn(t, &th4)
	if expected4 != th4 {
		t.Fatal("wrong thread")
	}
	if expected4.IsInline() {
		t.Fatal("thread anchored to line 0 should not be inline")
	}

}

func TestGetOkToCreateManyThreadsOnSameFile(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}
	const userId = 77
	const repoId = 20
	const quotaOwner = "owner"

	th, _ := s.CreateThread(w, quotaOwner, repoId, 98, 10, "file.txt", 0, userId, "test", true)
	expected := review.Thread{
		Type:          review.ThreadType_CommentsOnFileOnCommitVersion,
		AuthorUserId:  userId,
		Id:            1,
		CommitVersion: 10,
		Filename:      "file.txt",
		IsResolved:    true,
	}
	assertRecentAndClearCreatedOn(t, &th)
	if expected != th {
		t.Fatal("wrong thread 1")
	}
	th, _ = s.CreateThread(w, quotaOwner, repoId, 98, 10, "file.txt", 0, userId, "test", false)
	expected = review.Thread{
		Type:          review.ThreadType_CommentsOnFileOnCommitVersion,
		AuthorUserId:  userId,
		Id:            2, // Increase by 1
		CommitVersion: 10,
		Filename:      "file.txt",
		IsResolved:    false,
	}
	assertRecentAndClearCreatedOn(t, &th)
	if expected != th {
		t.Fatal("wrong thread 2")
	}
}

func TestGetThreads(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const userId = 86
	const repoId = 20
	const quotaOwner = "owner"

	s.CreateThread(w, quotaOwner, repoId, 0, 0, "c0_v0", 0, userId, "test", false)
	s.CreateThread(w, quotaOwner, repoId, 0, 1, "c0_v1", 0, userId, "test", true)
	s.CreateThread(w, quotaOwner, repoId, 1, 9, "c1_v1", 0, userId, "test", true)
	s.CreateThread(w, quotaOwner, repoId, 1, 99, "c1_v99", 0, userId, "test", true)
	s.CreateThread(w, "other", 500, 0, 1, "other repo", 0, userId, "test", true)

	it, err := s.GetThreads(w, repoId, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Next() {
		t.Fatal("no res")
	}
	th, err := it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if th.Filename != "c0_v0" {
		t.Fatal("expected c0_v0")
	}
	if th.IsResolved {
		t.Fatal("c0_v0 is not resolved")
	}
	if !it.Next() {
		t.Fatal("no res 2")
	}
	th, err = it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if th.Filename != "c0_v1" {
		t.Fatal("expected c0_v1")
	}
	if !th.IsResolved {
		t.Fatal("c0_v1 is resolved")
	}
	if it.Next() {
		t.Fatal("should be done")
	}
}

func TestGetComments(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const threadAuthor = 33
	const commentAuthor = 35
	const repoId = 20
	const quotaOwner = "owner"

	s.CreateThread(w, quotaOwner, repoId, 0, 0, "c0_v0", 0, threadAuthor, "a", false)
	th, err := s.AddToThread(w, quotaOwner, repoId, 0, 1, commentAuthor, "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if !th.IsResolved {
		t.Fatal("should be resolved")
	}

	it, err := s.GetComments(w, repoId, 0, th.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Next() {
		t.Fatal("expected comment 0")
	}
	cm, err := it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if cm.AuthorUserId != threadAuthor {
		t.Fatal("wrong user id1")
	}
	if cm.Text != "a" {
		t.Fatal("wrong first text")
	}
	if !it.Next() {
		t.Fatal("expected comment 1")
	}
	cm, err = it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if cm.AuthorUserId != commentAuthor {
		t.Fatal("wrong user id2")
	}
	if cm.Text != "b" {
		t.Fatal("wrong second text")
	}
	if it.Next() {
		t.Fatal("expected to be done")
	}
}

func TestLgtm(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}
	const repoId = 20
	const quotaOwner = "owner"

	// Test removing non existing
	if _, err := s.RemoveLastLgtm(w, quotaOwner, repoId, 0, 0); err == nil {
		t.Fatal("should err when trying to remove non-existing lgtm")
	}
	// HasLgtm returns nil error if not found
	hasLgtm, err := s.HasLgtm(w, repoId, 999, 999)
	if err != nil || hasLgtm {
		t.Fatal("expected nil err and !hasLgtm")
	}

	// Add an LGTM
	commitId := uint64(99)
	userId := int64(420)
	_, err = s.AddLgtm(w, quotaOwner, repoId, commitId, 0, userId)
	if err != nil {
		t.Fatal(err)
	}
	// Adding the same should err
	_, err = s.AddLgtm(w, quotaOwner, repoId, commitId, 0, userId)
	if err == nil {
		t.Fatal("should err when trying to add lgtm twice to same version")
	}
	// Adding to another version is ok
	_, err = s.AddLgtm(w, quotaOwner, repoId, commitId, 1, userId)
	if err != nil {
		t.Fatal(err)
	}

	// Check the hasLgtm
	hasLgtm, err = s.HasLgtm(w, repoId, commitId, userId)
	if err != nil {
		t.Fatal(err)
	}
	if !hasLgtm {
		t.Fatal("should have lgtm")
	}

	// Remove an LGTM. Should be ok to add it back on the latest version.
	_, err = s.RemoveLastLgtm(w, quotaOwner, repoId, commitId, userId)
	if err != nil {
		t.Fatal(err)
	}
	hasLgtm, err = s.HasLgtm(w, repoId, commitId, userId)
	if err != nil {
		t.Fatal(err)
	}
	if hasLgtm {
		t.Fatal("lgtm was deleted")
	}
	_, err = s.AddLgtm(w, quotaOwner, repoId, commitId, 1, userId)
	if err != nil {
		t.Fatal(err)
	}
	hasLgtm, err = s.HasLgtm(w, repoId, commitId, userId)
	if err != nil {
		t.Fatal(err)
	}
	if !hasLgtm {
		t.Fatal("lgtm was added back")
	}

	// Can't add LGTM in version i if there's LGTM on version i+1
	_, err = s.AddLgtm(w, quotaOwner, repoId, commitId, 0, userId)
	if err == nil {
		t.Fatal("should err when adding LGTM on past version")
	}
}

func TestReviewStatus(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const repoId = 11
	const quotaOwner = "owner"
	const commitId = uint64(10)
	// Helper function to check the data
	checkData := func(expected review.Data) {
		d, isNotFoundErr, err := s.GetData(w, repoId, commitId, false, uint64(0), []string{})
		if err != nil && !isNotFoundErr {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(d, expected) {
			t.Fatalf("wrong data\n got: %#v\nexpected: %#v", d, expected)
		}
	}
	// Data starts as missing-lgtm
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_MissingLgtm,
		ReviewStatusLgtmCount:       0,
		ReviewStatusUnresolvedCount: 0,
	})

	// Add a resolved thread. Nothing should change.
	const commitVersion = uint64(4)
	const filename = "myfile"
	const userId = 42
	th1, err := s.CreateThread(w, quotaOwner, repoId, commitId, commitVersion, filename,
		/*line=*/ 0, userId, "comment",
		/*resolved=*/ true)
	if err != nil {
		t.Fatal(err)
	}
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_MissingLgtm,
		ReviewStatusLgtmCount:       0,
		ReviewStatusUnresolvedCount: 0,
	})

	// Unresolve the thread by adding a comment. Status should change.
	th1, _ = s.AddToThread(w, quotaOwner, repoId, commitId, th1.Id, userId, "comment",
		/*resolved=*/ false)
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_Unresolved,
		ReviewStatusLgtmCount:       0,
		ReviewStatusUnresolvedCount: 1,
	})

	// Add an LGTM. Status should still be unresolved, but lgtm count +1
	s.AddLgtm(w, quotaOwner, repoId, commitId, commitVersion, userId)
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_Unresolved,
		ReviewStatusLgtmCount:       1,
		ReviewStatusUnresolvedCount: 1,
	})

	// Resolve the unresolved thread
	s.AddToThread(w, quotaOwner, repoId, commitId, th1.Id, userId, "resolve",
		/*resolved=*/ true)
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_Ready,
		ReviewStatusLgtmCount:       1,
		ReviewStatusUnresolvedCount: 0,
	})
	// Removing the lgtm only works if the user is correct
	_, err = s.RemoveLastLgtm(w, quotaOwner, repoId, commitId, userId+999)
	if err == nil {
		t.Fatal("should get not found")
	}
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_Ready,
		ReviewStatusLgtmCount:       1,
		ReviewStatusUnresolvedCount: 0,
	})
	// Now remove the correct lgtm. Status should update.
	s.RemoveLastLgtm(w, quotaOwner, repoId, commitId, userId)
	checkData(review.Data{
		Description:                 "",
		IsWIP:                       false,
		IsArchived:                  false,
		ReviewStatus:                review.ReviewStatus_MissingLgtm,
		ReviewStatusLgtmCount:       0,
		ReviewStatusUnresolvedCount: 0,
	})
}

func TestGetLgtmAuthors(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const repoId = uint64(30)
	const quotaOwner = "owner"
	const commitId = uint64(123)

	// user1: 1 event -> AddLGTM => odd => should appear
	const user1 = int64(1)
	if _, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 0, user1); err != nil {
		t.Fatalf("failed to add lgtm for user1: %v", err)
	}

	// user2: 2 events -> AddLGTM, RemoveLGTM => even => should NOT appear
	const user2 = int64(2)
	if _, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 0, user2); err != nil {
		t.Fatalf("failed to add lgtm for user2: %v", err)
	}
	if _, err := s.RemoveLastLgtm(w, quotaOwner, repoId, commitId, user2); err != nil {
		t.Fatalf("failed to remove lgtm for user2: %v", err)
	}

	// user3: 3 events -> AddLGTM, RemoveLGTM, AddLGTM => odd => should appear
	const user3 = int64(3)
	if _, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 0, user3); err != nil {
		t.Fatalf("failed to add lgtm (1) for user3: %v", err)
	}
	if _, err := s.RemoveLastLgtm(w, quotaOwner, repoId, commitId, user3); err != nil {
		t.Fatalf("failed to remove lgtm for user3: %v", err)
	}
	if _, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 1, user3); err != nil {
		t.Fatalf("failed to add lgtm (2) for user3: %v", err)
	}

	it, err := s.GetLgtmAuthors(w, repoId, commitId)
	if err != nil {
		t.Fatalf("GetLgtmAuthors returned error: %v", err)
	}

	var got []int64
	for it.Next() {
		userId, err := it.Get()
		if err != nil {
			t.Fatalf("iterator Get returned error: %v", err)
		}
		got = append(got, userId)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterator Err returned error: %v", err)
	}

	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })

	want := []int64{user1, user3}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetLgtmAuthors() = %v, want %v", got, want)
	}
}

func TestGetDataCheckOwnersDowngradesReadyWhenOwnersNotOk(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	us, err := user.NewService(mockJobLimitSetter{}, stripeclient.NewMockStripeClient(), b, "test-salt")
	if err != nil {
		t.Fatal(err)
	}

	fo := &fakeOwners{
		ok:  false,
		err: nil,
	}

	s, err := New(b, fo, us)
	if err != nil {
		t.Fatal("unexpected error instantiating service:", err)
	}

	const repoId = uint64(50)
	const quotaOwner = "owner"
	const commitId = uint64(7)
	const userEmail = "user@user.com"
	const username = "user"
	const plainPassword = "user123"

	user, _ := us.RegisterNewUser(w, userEmail, username, plainPassword)

	if _, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 0, user.Id); err != nil {
		t.Fatalf("AddLgtm returned error: %v", err)
	}

	d, _, err := s.GetData(w, repoId, commitId, false /*checkOwners*/, 0, []string{})
	if err != nil {
		t.Fatalf("GetData (without owners check) returned error: %v", err)
	}
	if d.ReviewStatus != review.ReviewStatus_Ready {
		t.Fatalf("expected status READY before owners check, got %v", d.ReviewStatus)
	}

	d, _, err = s.GetData(w, repoId, commitId, true /*checkOwners*/, 0, []string{"supreme"})
	if err != nil {
		t.Fatalf("GetData (with owners check) returned error: %v", err)
	}

	if !fo.called {
		t.Fatalf("expected OwnersLgmtIsOk to be called when status is READY and checkOwners = true")
	}

	if d.ReviewStatus != review.ReviewStatus_MissingOwnersApproval {
		t.Fatalf("expected status MissingOwnersApproval after failed owners check, got %v", d.ReviewStatus)
	}
}

func TestGetDataNotCalledWhenNotReady(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	fo := &fakeOwners{
		ok:  true,
		err: nil,
	}

	s, err := New(b, fo, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service:", err)
	}

	const repoId = uint64(60)
	const commitId = uint64(8)

	d, isNotFoundErr, err := s.GetData(w, repoId, commitId, true /*checkOwners*/, 0, []string{"supreme"})
	if err != nil && !isNotFoundErr {
		t.Fatalf("GetData returned error: %v", err)
	}

	if d.ReviewStatus != review.ReviewStatus_MissingLgtm {
		t.Fatalf("expected status MissingLgtm, got %v", d.ReviewStatus)
	}

	if fo.called {
		t.Fatalf("expected OwnersLgmtIsOk NOT to be called when status != READY")
	}
}

func TestGetDataReadyWhenOwnersAddLgtm(t *testing.T) {

	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	us, err := user.NewService(mockJobLimitSetter{}, stripeclient.NewMockStripeClient(), b, "test-salt")
	if err != nil {
		t.Fatal(err)
	}

	fo := &fakeOwners{
		ok:  true,
		err: nil,
	}

	s, err := New(b, fo, us)
	if err != nil {
		t.Fatal("unexpected error instantiating service:", err)
	}

	const repoId = uint64(70)
	const quotaOwner = "owner"
	const commitId = uint64(9)
	const userEmail = "user@user.com"
	const username = "user"
	const plainPassword = "user123"

	user, _ := us.RegisterNewUser(w, userEmail, username, plainPassword)

	if _, err := s.AddLgtm(w, quotaOwner, repoId, commitId, 0, user.Id); err != nil {
		t.Fatalf("AddLgtm returned error: %v", err)
	}

	d, _, err := s.GetData(w, repoId, commitId, true /*checkOwners*/, 0, []string{"supreme"})
	if err != nil {
		t.Fatalf("GetData returned error: %v", err)
	}

	if !fo.called {
		t.Fatalf("expected OwnersLgmtIsOk to be called")
	}
	if d.ReviewStatus != review.ReviewStatus_Ready {
		t.Fatalf("expected status READY when owners check passes, got %v", d.ReviewStatus)
	}
}
func TestAddAndRemoveReviewer(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const quotaOwner = "owner"
	const repoId = uint64(1)
	const commitId = uint64(123)

	// Add first reviewer (should create Data and persist)
	if err := s.AddReviewer(w, quotaOwner, repoId, commitId, 10); err != nil {
		t.Fatalf("AddReviewer failed: %v", err)
	}
	d, isNotFoundErr, err := s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if isNotFoundErr {
		t.Fatal("expected data to exist after AddReviewer")
	}
	if d.ReviewersUserIds[0] != int64(10) {
		t.Fatalf("wrong reviewers\n got: %#v\nexpected: %#v", d.ReviewersUserIds, []int64{10})
	}

	// Add another reviewer
	if err := s.AddReviewer(w, quotaOwner, repoId, commitId, 20); err != nil {
		t.Fatalf("AddReviewer(2) failed: %v", err)
	}
	d, _, err = s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if !reflect.DeepEqual(d.ReviewersUserIds, []int64{10, 20}) {
		t.Fatalf("wrong reviewers\n got: %#v\nexpected: %#v", d.ReviewersUserIds, []int64{10, 20})
	}

	// Add duplicate (should "not add")
	if err := s.AddReviewer(w, quotaOwner, repoId, commitId, 20); err != nil {
		t.Fatalf("AddReviewer(duplicate) failed: %v", err)
	}
	d, _, err = s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if !reflect.DeepEqual(d.ReviewersUserIds, []int64{10, 20}) {
		t.Fatalf("duplicate add should not change list\n got: %#v\nexpected: %#v", d.ReviewersUserIds, []int64{10, 20})
	}

	// Remove non-existing
	if err := s.RemoveReviewer(w, quotaOwner, repoId, commitId, 999); err != nil {
		t.Fatalf("RemoveReviewer(non-existing) failed: %v", err)
	}
	d, _, err = s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if !reflect.DeepEqual(d.ReviewersUserIds, []int64{10, 20}) {
		t.Fatalf("remove non-existing should not change list\n got: %#v\nexpected: %#v", d.ReviewersUserIds, []int64{10, 20})
	}

	// Remove existing
	if err := s.RemoveReviewer(w, quotaOwner, repoId, commitId, 10); err != nil {
		t.Fatalf("RemoveReviewer failed: %v", err)
	}
	d, _, err = s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if !reflect.DeepEqual(d.ReviewersUserIds, []int64{20}) {
		t.Fatalf("wrong reviewers after remove\n got: %#v\nexpected: %#v", d.ReviewersUserIds, []int64{20})
	}

	// Remove last
	if err := s.RemoveReviewer(w, quotaOwner, repoId, commitId, 20); err != nil {
		t.Fatalf("RemoveReviewer(2) failed: %v", err)
	}
	d, _, err = s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}

	if len(d.ReviewersUserIds) != 0 {
		t.Fatalf("expected empty reviewers after removing all, got: %#v", d.ReviewersUserIds)
	}
}

func TestAddReviewerMaxLimit(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeW()

	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal("unexpected error instantiating service")
	}

	const quotaOwner = "owner"
	const repoId = uint64(2)
	const commitId = uint64(456)

	originalMaxReviewers := MaxReviewers
	t.Cleanup(func() {
		MaxReviewers = originalMaxReviewers
	})
	MaxReviewers = 20
	// Fill up to MaxReviewers
	for i := 0; i < MaxReviewers; i++ {
		if err := s.AddReviewer(w, quotaOwner, repoId, commitId, int64(i+1)); err != nil {
			t.Fatalf("failed adding reviewer %d: %v", i+1, err)
		}
	}

	// Next add must fail
	if err := s.AddReviewer(w, quotaOwner, repoId, commitId, 999999); err == nil {
		t.Fatalf("expected error when adding reviewer past MaxReviewers=%d", MaxReviewers)
	}

	d, _, err := s.GetData(w, repoId, commitId, false, 0, []string{})
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	// `MaxReviewers+1` because in the service we have:
	// That mens when len(d.ReviewersUserIds) == MaxReviewers it can be added one more reviewer
	if len(d.ReviewersUserIds) != MaxReviewers {
		t.Fatalf("expected %d reviewers, got %d", MaxReviewers, len(d.ReviewersUserIds))
	}
}

func TestResolveSupremeLeaders_UserOwned(t *testing.T) {
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
	s, err := New(b, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ownerUsr := user.User{
		Id:             42,
		Username:       "supreme-user",
		IsOrganization: false,
	}

	leaders, err := s.ResolveSupremeLeaders(w, ownerUsr)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(leaders, []string{"supreme-user"}) {
		t.Fatalf("expected [supreme-user], got %v", leaders)
	}
}

func TestResolveSupremeLeaders_OrgOwned(t *testing.T) {
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

	us, err := user.NewService(mockJobLimitSetter{}, stripeclient.NewMockStripeClient(), b, "test-salt")
	if err != nil {
		t.Fatal(err)
	}

	u1, err := us.RegisterNewUser(w, "owner1@example.com", "owner1", "pass123")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := us.RegisterNewUser(w, "owner2@example.com", "owner2", "pass456")
	if err != nil {
		t.Fatal(err)
	}

	orgUser := user.User{
		Id:             999,
		Username:       "my-org",
		IsOrganization: true,
	}

	orgAssetId := permissions.OrganizationAssetId(orgUser.Id)
	_, err = b.GrantPermissionIfNotExists(w, u1.Id, permissions.Permission_OrganizationOwner, orgAssetId)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GrantPermissionIfNotExists(w, u2.Id, permissions.Permission_OrganizationOwner, orgAssetId)
	if err != nil {
		t.Fatal(err)
	}

	s, err := New(b, nil, us)
	if err != nil {
		t.Fatal(err)
	}

	leaders, err := s.ResolveSupremeLeaders(w, orgUser)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(leaders)
	want := []string{"owner1", "owner2"}
	if !reflect.DeepEqual(leaders, want) {
		t.Fatalf("got %v, want %v", leaders, want)
	}
}

type mockJobLimitSetter struct{}

func (js mockJobLimitSetter) PutLimits(ownerId int64, maxJobs int,
	maxTimeout time.Duration, tx context.Context) error {
	return nil
}