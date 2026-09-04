package server

import (
	"bytes"
	"errors"
	"monorepo/buildmeta"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
	"monorepo/twigg/workdir"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// API Key used for tests
const testApiKey = "fake-api-key"

func TestInit(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	cl0 := srv.Top()
	if cl0.ServerL != 0 {
		t.Fatal("first cl be of the root")
	}
	if cl0.L != 0 {
		t.Fatal("Root should have L=0")
	}
	if cl0.ParentL != 0 {
		t.Fatal("Root has parent=0")
	}
	if cl0.Birth != commit.BirthReasonCommit {
		t.Fatal("wrong root birth reason")
	}

	c0 := srv.GetLatest(0)
	if c0.ServerL != 0 {
		t.Fatal("failed to get first commit")
	}
}

func TestCommitNotFound(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	srv.CheckNotFound(99)
	srv.CheckVersionNotFound(0, 99)
}

func TestGracefullErrMessages(t *testing.T) {
	root1, ag1, wd, l := client.NewTest("owner", 1, t)
	_, _, err := ag1.PullAllSubmittedAfter(root1, "invalid url format", testApiKey, nil, l)
	if !errors.Is(err, client.ErrFailedToReachServer) {
		t.Fatal(err)
	}
	_, _, err = ag1.PullAllSubmittedAfter(root1, "http://localhost:11111", testApiKey, nil, l)
	if !errors.Is(err, client.ErrFailedToReachServer) {
		t.Fatal(err)
	}
	srv := NewTestServer(testApiKey, t)
	_, _, err = ag1.PullAllSubmittedAfter(root1, srv.RootUrl()+"/"+srv.RandomResponsePath(), testApiKey, nil, l)
	if !errors.Is(err, client.ErrNotTwiggServer) {
		t.Fatal(err)
	}
	_, _, err = ag1.PullTopCommit(root1, srv.RootUrl()+"/"+srv.RandomResponsePath(), testApiKey, nil, l)
	if !errors.Is(err, client.ErrNotTwiggServer) {
		t.Fatal(err)
	}
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd, "c1", &root1, l)
	_, _, err = ag1.Push(&c1, srv.RootUrl()+"/"+srv.RandomResponsePath(), testApiKey, l)
	if !errors.Is(err, client.ErrNotTwiggServer) {
		t.Fatal(err)
	}
	_, err = ag1.SetNextServerId(srv.RootUrl()+"/"+srv.RandomResponsePath(), testApiKey, 100)
	if !errors.Is(err, client.ErrNotTwiggServer) {
		t.Fatal(err)
	}
	_, err = ag1.SetNextServerId("http://localhost:11111", testApiKey, 100)
	if !errors.Is(err, client.ErrFailedToReachServer) {
		t.Fatal(err)
	}
}

func TestNothingToPull(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root1, ag1, _, l := client.NewTest("owner", 1, t)

	added := 0
	onPull := func(new commit.Commit, hasOld bool, old commit.Commit) error {
		added++
		return nil
	}

	_, _, err := ag1.PullAllSubmittedAfter(root1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l)
	if !errors.Is(err, client.ErrNothingToPull) {
		t.Fatalf("should return NothingToPull, got: %s", err)
	}
	if added != 0 {
		t.Fatal("expected nothing added")
	}
}

func TestServerGracefullyHandlesInvalidRequests(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	_, ag1, _, l := client.NewTest("owner", 1, t)

	// Create an invalid commit argument and pass it to the server.
	// The server should not error in any way, and should just send an error
	// response to the client
	invalidCommit := commit.Commit{
		L:          999,
		HasServerL: true,
		HasServerV: true,
		ServerL:    999,
		ServerV:    1,
	}
	_, _, err := ag1.PullAllSubmittedAfter(invalidCommit, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, nil, l)
	if !strings.Contains(err.Error(), "not found") {
		t.Fatal("expected not found error")
	}
}

func TestSinglePush(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create the commit c1 and push it
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)

	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Version != 0 {
		t.Fatal("version should not update after push")
	}
	// c1 should get the server L and V after push
	if !c1.HasServerL || !c1.HasServerV {
		t.Fatal("c1 was pushed directly but is not marked public")
	}
	if c1.ServerL != 1 || c1.ServerV != 0 {
		t.Fatal("wrong server IDs of c1")
	}
}

func TestWriteFile(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create the commit c1 and push it
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)

	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	serverC1 := srv.GetLatest(1)
	buff := bytes.NewBuffer(nil)
	srv.WriteFile(serverC1, "a.txt", buff)
	if buff.String() != "aaa" {
		t.Fatal("unexpected file content")
	}
}

func TestCommitWorkdirSize(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create the commit c1 and push it
	c1FileContent := "aaaaaa"
	wd.WriteFile("a.txt", c1FileContent)
	c1, _ := ag.Commit(wd, "c1", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	serverC1 := srv.GetLatest(1)
	n := srv.GetCommitWorkdirSize(serverC1)
	if n != int64(len(c1FileContent)) {
		t.Fatalf("got workdir size %d", n)
	}
}

// Mock pushObserver that sets messages of new commits to "mock"
// and of updates to old commits to "mock2"
type mockPushObserver struct{}

func (m mockPushObserver) OnPush(
	newCommit *commit.Commit, hasOld bool, oldCommit commit.Commit) {
	if hasOld {
		newCommit.Message = "mock2"
	} else {
		newCommit.Message = "mock"
	}
}

func TestPushObserver(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	// Set the mock pushObserver to modify the commits
	srv.SetPushObserver(mockPushObserver{})

	// Create and push a commit
	root, ag, wd, l := client.NewTest("owner", 1, t)
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	c := srv.GetLatest(c1.L)
	if c.Message != "mock" {
		t.Fatal("mock push observer didn't work")
	}

	// Push a new version. The observer will set its message to mock2
	// because there is a previous version
	c1Ammended, err := ag.Amend(&c1, false, wd, "ammended", l)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ag.Push(&c1Ammended, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	c = srv.GetLatest(c1Ammended.L)
	if c.Message != "mock2" {
		t.Fatal("mock push observer didn't work")
	}

}

func TestPushAncestors(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write c1 and c2. Push starting from c2. Expect c1 and then c2 to push.
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("b.txt", "bbb")
	c2, _ := ag.Commit(wd, "c2", &c1, l)

	// Push starting from c2
	_, _, err := ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	if !c2.HasServerL || !c2.HasServerV {
		t.Fatal("c2 was pushed indirectly but is not marked public")
	}
	if c2.ServerL != 2 || c2.ServerV != 0 {
		t.Fatal("wrong server IDs of c2")
	}

	// c1 should also be pushed
	c1, _ = ag.GetLatest(c1.L, l)
	if !c1.IsOnServer() {
		t.Fatal("c1 should be pushed")
	}
	if c1.ServerL != 1 || c1.ServerV != 0 {
		t.Fatal("wrong server IDs of c1")
	}
}

func TestSimpleAmendAndPush(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create the commit c1 and push it
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)

	// Amend it, and push again
	c1V1, _ := ag.Amend(&c1, false, wd, "c1v1", l)
	if c1V1.Version != 1 {
		t.Fatal("expecte v1 after amend")
	}
	ag.Push(&c1V1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if c1V1.Version != 1 {
		t.Fatal("version should not update after push")
	}
	if !c1V1.IsOnServer() {
		t.Fatal("c1V1 should have server ids")
	}
	if c1V1.ServerL != 1 || c1V1.ServerV != 1 {
		t.Fatal("wrong server IDs of c1V1")
	}
}

func TestCantPushWithNonPushedObsoleteParent(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create a stack c1 and c2
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("b.txt", "bbb")
	ag.Commit(wd, "c2", &c1, l)
	// Amend c1 without rebasing c2, causing it to become orphan
	ag.Amend(&c1, false, wd, "c1v1", l)
	// Now try pushing c2
	c2, _ := ag.GetLatest(2, l)
	isObsParentErr, _, err := ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err == nil {
		t.Fatal("expected error")
	}
	if !isObsParentErr {
		t.Fatal("expected isObsParentErr")
	}
}

func TestOkToPushWithObsoleteParentIfParentIsPushed(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create a stack c1 and c2
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("b.txt", "bbb")
	ag.Commit(wd, "c2", &c1, l)
	// Now push only c1
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	// Amend c1 without rebasing c2, causing it to become orphan
	ag.Amend(&c1, false, wd, "c1v1", l)
	// Now try pushing c2. It should be ok because the previous version of the
	// parent has been pushed before.
	c2, _ := ag.GetLatest(2, l)
	isObsParentErr, isBadApiKeyErr, err := ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil || isObsParentErr || isBadApiKeyErr {
		t.Fatal(err)
	}
}

func TestPersistance(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push a commit using root as base
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	if c1.IsSubmitted {
		t.Fatal("c1 should be pending")
	}
	if srv.NPending() != 1 {
		t.Fatal("expected 1 pending")
	}

	srv.Restart()
	gotC1 := srv.GetLatest(1)
	if gotC1.IsSubmitted {
		t.Fatal("state not persisted")
	}
	if srv.NPending() != 1 {
		t.Fatal("expected 1 pending")
	}
	// Push should still be ok
	wd.WriteFile("b.txt", "bbb")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	_, _, err = ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	gotC2 := srv.GetLatest(2)
	if gotC2.IsSubmitted {
		t.Fatal("c2 should be pending")
	}
	if srv.NPending() != 2 {
		t.Fatal("expected 2 pending")
	}

	// Submit and restart
	srv.Submit(c1.L)
	srv.Restart()
	gotC1 = srv.GetLatest(1)
	if !gotC1.IsSubmitted {
		t.Fatal("state not persisted bc c1 was submitted")
	}
	if srv.NPending() != 1 {
		t.Fatal("expected 1 pending")
	}
	if srv.Top().L != 1 {
		t.Fatal("wrong top commit")
	}
}

func TestPending(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push two commits
	buildmeta.Version = "client-build-version"
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	c1RootHash := wd.RootDirHash()
	wd.WriteFile("b.txt", "bbb")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	c2RootHash := wd.RootDirHash()
	buildmeta.Version = "server-build-version"
	_, _, err := ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	if srv.NPending() != 2 {
		t.Fatal("expected 2 pending CLs")
	}

	p, closeP := srv.Pending()
	defer closeP()

	// Check cl 1
	if !p.Next() {
		t.Fatalf("should be able to iterate")
	}
	gotCl1, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	expectedCl1 := commit.Commit{
		Birth:                commit.BirthReasonCommit,
		L:                    1,
		Version:              0,
		HasServerL:           true,
		ServerL:              1,
		HasServerV:           true,
		ServerV:              0,
		TreeVersion:          1,
		RootDirHash:          c1RootHash,
		ParentL:              0,
		ParentV:              0,
		HasParentServerL:     true,
		ParentServerL:        0,
		HasParentServerV:     true,
		ParentServerV:        0,
		ParentTreeVersion:    0,
		Status:               commit.StatusLatest,
		ObsReason:            commit.ObsoleteReasonNone,
		Message:              "c1",
		HasRebaseConflicts:   false,
		Children:             nil, // Children are remove on the server
		IsSubmitted:          false,
		ClientBuildVersion:   "client-build-version",
		ServerBuildVersion:   "server-build-version",
		HasDiffData:          true,
		DiffDataLinesCreated: 1,
		DiffDataFilesCreated: 1,
	}
	// DeepEqual doesnt handle time well
	expectedCl1.CreatedOn = time.Unix(0, 0)
	gotCl1.CreatedOn = time.Unix(0, 0)
	if !reflect.DeepEqual(gotCl1, expectedCl1) {
		t.Fatal("wrong cl 1")
	}

	if !p.Next() {
		t.Fatalf("should be able to iterate")
	}
	gotCl2, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	expectedCl2 := commit.Commit{
		Birth:                commit.BirthReasonCommit,
		L:                    2,
		Version:              0,
		HasServerL:           true,
		ServerL:              2,
		HasServerV:           true,
		ServerV:              0,
		TreeVersion:          2,
		RootDirHash:          c2RootHash,
		ParentL:              1,
		ParentV:              0,
		HasParentServerL:     true,
		ParentServerL:        1,
		HasParentServerV:     true,
		ParentServerV:        0,
		ParentTreeVersion:    1,
		Status:               commit.StatusLatest,
		ObsReason:            commit.ObsoleteReasonNone,
		Message:              "c2",
		HasRebaseConflicts:   false,
		Children:             nil,
		IsSubmitted:          false,
		ClientBuildVersion:   "client-build-version",
		ServerBuildVersion:   "server-build-version",
		HasDiffData:          true,
		DiffDataLinesCreated: 1,
		DiffDataFilesCreated: 1,
	}
	// DeepEqual doesnt handle time well
	expectedCl2.CreatedOn = time.Unix(0, 0)
	gotCl2.CreatedOn = time.Unix(0, 0)
	if !reflect.DeepEqual(gotCl2, expectedCl2) {
		t.Fatal("wrong cl 2")
	}

	if p.Next() {
		t.Fatalf("should be done iterating")
	}
	err = p.Err()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPendingAfter(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push three commits
	buildmeta.Version = "client-build-version"
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	c1RootHash := wd.RootDirHash()
	wd.WriteFile("b.txt", "bbb")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	c2RootHash := wd.RootDirHash()
	wd.WriteFile("c.txt", "ccc")
	c3, _ := ag.Commit(wd, "c3", &c2, l)
	buildmeta.Version = "server-build-version"
	_, _, err := ag.Push(&c3, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	if srv.NPending() != 3 {
		t.Fatal("expected 3 pending CLs")
	}

	// Get after "c3", expect "c2", "c1"
	p, closeP := srv.PendingAfter(c3.L)
	defer closeP()

	// Check cl 2
	if !p.Next() {
		t.Fatalf("should be able to iterate")
	}
	gotCl2, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	expectedCl2 := commit.Commit{
		Birth:                commit.BirthReasonCommit,
		L:                    2,
		Version:              0,
		HasServerL:           true,
		ServerL:              2,
		HasServerV:           true,
		ServerV:              0,
		TreeVersion:          2,
		RootDirHash:          c2RootHash,
		ParentL:              1,
		ParentV:              0,
		HasParentServerL:     true,
		ParentServerL:        1,
		HasParentServerV:     true,
		ParentServerV:        0,
		ParentTreeVersion:    1,
		Status:               commit.StatusLatest,
		ObsReason:            commit.ObsoleteReasonNone,
		Message:              "c2",
		HasRebaseConflicts:   false,
		Children:             nil,
		IsSubmitted:          false,
		ClientBuildVersion:   "client-build-version",
		ServerBuildVersion:   "server-build-version",
		HasDiffData:          true,
		DiffDataLinesCreated: 1,
		DiffDataFilesCreated: 1,
	}
	// DeepEqual doesnt handle time well
	expectedCl2.CreatedOn = time.Unix(0, 0)
	gotCl2.CreatedOn = time.Unix(0, 0)
	if !reflect.DeepEqual(gotCl2, expectedCl2) {
		t.Fatal("wrong cl 2")
	}

	// Check cl 1
	if !p.Next() {
		t.Fatalf("should be able to iterate")
	}
	gotCl1, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	expectedCl1 := commit.Commit{
		Birth:                commit.BirthReasonCommit,
		L:                    1,
		Version:              0,
		HasServerL:           true,
		ServerL:              1,
		HasServerV:           true,
		ServerV:              0,
		TreeVersion:          1,
		RootDirHash:          c1RootHash,
		ParentL:              0,
		ParentV:              0,
		HasParentServerL:     true,
		ParentServerL:        0,
		HasParentServerV:     true,
		ParentServerV:        0,
		ParentTreeVersion:    0,
		Status:               commit.StatusLatest,
		ObsReason:            commit.ObsoleteReasonNone,
		Message:              "c1",
		HasRebaseConflicts:   false,
		Children:             nil, // Children are remove on the server
		IsSubmitted:          false,
		ClientBuildVersion:   "client-build-version",
		ServerBuildVersion:   "server-build-version",
		HasDiffData:          true,
		DiffDataLinesCreated: 1,
		DiffDataFilesCreated: 1,
	}
	// DeepEqual doesnt handle time well
	expectedCl1.CreatedOn = time.Unix(0, 0)
	gotCl1.CreatedOn = time.Unix(0, 0)
	if !reflect.DeepEqual(gotCl1, expectedCl1) {
		t.Fatal("wrong cl 1")
	}

	if p.Next() {
		t.Fatalf("should be done iterating")
	}
	err = p.Err()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSubmit(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write two commits and push them
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	_, _, err := ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	if err != nil {
		t.Fatal(err)
	}

	// Commit tree:
	//
	// c2_v0
	// |
	// c1_v0
	// |
	// root

	// Submit c1:
	//
	// c2_v0
	// |
	// *c1_v0  c1_v1
	// |       |
	// root ---/
	canSubmit := srv.CanSubmit(1)
	if !canSubmit {
		t.Fatal("c0 should not cause conflict")
	}
	srv.Submit(1)
	gotC1 := srv.GetLatest(1)
	if !gotC1.IsSubmitted {
		t.Fatal("c1 is submitted")
	}
	if gotC1.Birth != commit.BirthReasonSubmit {
		t.Fatal("c1 was submitted")
	}
	if gotC1.Version != 1 {
		t.Fatal("c1 submit has v1")
	}
	// Submit c2:
	//
	// *c2_v0  c2_v1
	// |       |
	// *c1_v0  c1_v1
	// |       |
	// root ---/
	canSubmit = srv.CanSubmit(2)
	if !canSubmit {
		t.Fatal("c2 should not cause conflict")
	}
	srv.Submit(2)
	gotC2 := srv.GetLatest(2)
	if !gotC2.IsSubmitted {
		t.Fatal("c2 is submitted")
	}
	if gotC2.Version != 1 {
		t.Fatal("c2 submit has v1")
	}
	if gotC2.Birth != commit.BirthReasonSubmit {
		t.Fatal("c2 was submitted")
	}
}

func TestPullChangesSubmitted(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write one commits push
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	_, _, err := ag1.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	if err != nil {
		t.Fatal(err)
	}

	var pulled commit.Commit
	nPulled := 0
	onPull := func(new commit.Commit, hasOld bool, old commit.Commit) error {
		nPulled++
		pulled = new
		return nil
	}

	// Submit CL 1 (the commit pushed) on the server
	srv.Submit(1)
	// Pulling should pull the commit bc it's been submitted
	_, _, err = ag1.PullAllSubmittedAfter(root1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l1)
	if err != nil {
		t.Fatal(err)
	}
	if !pulled.HasServerL ||
		pulled.ServerL != 1 || pulled.ServerV != 1 ||
		!pulled.HasServerV {
		t.Fatal("pulled did not get server LV")
	}
	if nPulled != 1 || !pulled.IsSubmitted {
		t.Fatal("submitted commit was not pulled")
	}
	if pulled.Birth != commit.BirthReasonSubmit {
		t.Fatal("commit should be submitted")
	}
	// c1 should now be marked as submitted and its version should increase
	c1, err = ag1.GetLatest(c1.L, l1)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Birth != commit.BirthReasonSubmit {
		t.Fatal("c1 should be submitted")
	}
	if !c1.IsSubmitted {
		t.Fatal("c1 not marked as submitted")
	}
	if c1.Version != 1 {
		t.Fatal("c1 should now be v1")
	}
	if c1.L != 1 {
		t.Fatal("c1 should have  #1")
	}
	if c1.ServerL != 1 {
		t.Fatal("expected serverL 1")
	}
	if c1.ServerV != 1 {
		t.Fatal("expected serverV 1")
	}

}

func TestPullFromOtherClient(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write two commits, push and submit
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	srv.Submit(1)
	srv.Submit(2)

	nPulled := 0
	onPull := func(new commit.Commit, hasOld bool, old commit.Commit) error {
		nPulled++
		return nil
	}

	// Pull commits to a different client
	_, ag2, wd2, l2 := client.NewTest("owner", 2, t)
	_, _, err := ag2.PullAllSubmittedAfter(root1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l2)
	if err != nil {
		t.Fatal(err)
	}
	if nPulled != 2 {
		t.Fatal("should have pulled 2 commits")
	}

	c1, _ = ag2.GetLatest(1, l2)
	if c1.Birth != commit.BirthReasonSubmit {
		t.Fatal("c1 was  submitted")
	}
	c2, _ = ag2.GetLatest(2, l2)
	if c2.Birth != commit.BirthReasonSubmit {
		t.Fatal("c2 was  submitted")
	}

	// Inially the wd2 is empty
	if wd2.HasFile("a.txt") || wd2.HasFile("b.txt") {
		t.Fatal("wd2 should be empty")
	}

	// Load c1 and check the wd
	err = ag2.Load(c1.TreeVersion, wd2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if wd2.ReadFile("a.txt") != "aaa" {
		t.Fatal("a.txt not loaded")
	}
	if wd2.HasFile("b.txt") {
		t.Fatal("b should not yet be here")
	}
	// Load c2 and check the wd
	err = ag2.Load(c2.TreeVersion, wd2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if wd2.ReadFile("a.txt") != "aaa" {
		t.Fatal("a.txt not loaded")
	}
	if wd2.ReadFile("b.txt") != "bbb" {
		t.Fatal("b.txt not loaded")
	}
}

func TestPullDetached(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write two commits, push and submit
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	srv.Submit(1)
	srv.Submit(2)

	// Pull only c2 (i.e. detached) to a different client
	root2, ag2, wd2, l2 := client.NewTest("owner", 2, t)
	nPulled := 0
	onPull := func(new commit.Commit,
		hasOld bool, old commit.Commit) error {
		nPulled++
		return nil
	}
	_, _, err := ag2.PullCommit(
		/*commitId*/ 2,
		/*hasV*/ true,
		/*commitV*/ 1,
		/*base*/ root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l2)
	if err != nil {
		t.Fatal(err)
	}
	if nPulled != 1 {
		t.Fatal("should have pulled 1 commit")
	}
	// Check the commit pulled
	detachedC2, err := ag2.GetLatestByServerId(2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if detachedC2.Message != "c2" ||
		!detachedC2.IsSubmitted ||
		!detachedC2.HasParentServerL ||
		!detachedC2.HasParentServerV {
		t.Fatal("invalid pulled detachedC2")
	}
	if !detachedC2.IsDetached {
		t.Fatal("c2 not pulled detached")
	}

	// Loading it to the workdir works ok
	err = ag2.Load(detachedC2.TreeVersion, wd2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if wd2.ReadFile("a.txt") != "aaa" || wd2.ReadFile("b.txt") != "bbb" {
		t.Fatal("failed to load detached")
	}
}
func TestPullDetachedSendsChildren(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner1", 1, t)

	// Write three commits and push them
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	wd1.WriteFile("c.txt", "ccc")
	c3, _ := ag1.Commit(wd1, "c3", &c2, l1)
	ag1.Push(&c3, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)

	// Pull only c2 (i.e. detached) to a different client
	root2, ag2, _, l2 := client.NewTest("owner1", 2, t)
	var pulled commit.Commit
	onPull := func(new commit.Commit,
		hasOld bool, old commit.Commit) error {
		pulled = new
		return nil
	}
	_, _, err := ag2.PullCommit(
		/*commitId*/ 2,
		/*hasV*/ true,
		/*commitV*/ 0,
		/*base*/ root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l2)
	if err != nil {
		t.Fatal(err)
	}
	// c3 was pushed as a child of c2, so it must come with it
	if !reflect.DeepEqual(pulled.Children, []commit.LocalId{3}) {
		t.Fatalf("children=%v, expected [3]", pulled.Children)
	}
	if !reflect.DeepEqual(pulled.ChildrenVersions, []uint64{0}) {
		t.Fatalf("childrenVersions=%v, expected [0]", pulled.ChildrenVersions)
	}

	// The top commit has no children
	_, _, err = ag2.PullCommit(
		/*commitId*/ 3,
		/*hasV*/ true,
		/*commitV*/ 0,
		/*base*/ root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled.Children) != 0 {
		t.Fatalf("children=%v, expected none", pulled.Children)
	}
}

func TestPullDetachedWithoutVersion(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write two commits, push and submit
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	srv.Submit(1)
	srv.Submit(2)

	// Pull only c2 (i.e. detached) to a different client without specifying v
	root2, ag2, _, l2 := client.NewTest("owner", 2, t)
	nPulled := 0
	onPull := func(new commit.Commit,
		hasOld bool, old commit.Commit) error {
		nPulled++
		return nil
	}
	_, _, err := ag2.PullCommit(
		/*commitId*/ 2,
		/*hasV*/ false,
		/*commitV*/ 9999,
		/*base*/ root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l2)
	if err != nil {
		t.Fatal(err)
	}
	if nPulled != 1 {
		t.Fatal("should have pulled 1 commit")
	}
	// Check the commit pulled
	detachedC2, err := ag2.GetLatestByServerId(2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if detachedC2.ServerL != 2 ||
		detachedC2.ServerV != 1 || !detachedC2.IsDetached {
		t.Fatal("invalid pulled detachedC2")
	}
}

func TestPullTopWithKnownParent(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write a commit, push and submit
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	ag1.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	srv.Submit(1)

	// Pull the top commit directly. It'll be auto attached to the parent,
	// since the parent is known
	root2, ag2, wd2, l2 := client.NewTest("owner", 2, t)
	_, _, err := ag2.PullTopCommit(root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, nil, l2)
	if err != nil {
		t.Fatal(err)
	}
	// Check the commit pulledC1
	pulledC1, err := ag2.GetLatestByServerId(1, l2)
	if err != nil {
		t.Fatal(err)
	}
	if pulledC1.Message != "c1" ||
		!pulledC1.IsSubmitted ||
		!pulledC1.HasParentServerL ||
		!pulledC1.HasParentServerV {
		t.Fatal("invalid pulled pulledC1")
	}
	if pulledC1.IsDetached {
		t.Fatal("c1 not attached to local parent")
	}
	err = ag2.Load(pulledC1.TreeVersion, wd2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if wd2.ReadFile("a.txt") != "aaa" {
		t.Fatal("failed to load detached")
	}
}

func TestPullTopWithUnknownParent(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write a 2 commits, push and submit
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	srv.Submit(1)
	srv.Submit(2)

	// Pull the top commit directly. It won't be attached because c1 was
	// not pulled yet
	root2, ag2, wd2, l2 := client.NewTest("owner", 2, t)
	_, _, err := ag2.PullTopCommit(root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, nil, l2)
	if err != nil {
		t.Fatal(err)
	}
	// Check the commit pulledC1
	pulledC2, err := ag2.GetLatestByServerId(2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if pulledC2.Message != "c2" ||
		!pulledC2.IsSubmitted ||
		!pulledC2.HasParentServerL ||
		!pulledC2.HasParentServerV {
		t.Fatal("invalid pulled pulledC2")
	}
	if !pulledC2.IsDetached {
		t.Fatal("c2 not detached")
	}
	err = ag2.Load(pulledC2.TreeVersion, wd2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if wd2.ReadFile("a.txt") != "aaa" ||
		wd2.ReadFile("b.txt") != "bbb" {
		t.Fatal("failed to load detached")
	}
}

func TestPullTopCommit_PullAllSubmittedAfter(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	// Write a 2 commits, push and submit
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	srv.Submit(1)
	srv.Submit(2)

	// Pull the top to a different client
	root2, ag2, _, l2 := client.NewTest("owner", 2, t)
	_, _, err := ag2.PullTopCommit(root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, nil, l2)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ag2.GetLatestByServerId(2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsDetached {
		t.Fatal("2 not detached")
	}

	// Now pull all submitted after root
	nPulled := 0
	onPull := func(new commit.Commit,
		hasOld bool, old commit.Commit) error {
		nPulled++
		return nil
	}
	_, _, err = ag2.PullAllSubmittedAfter(root2,
		srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, onPull, l2)
	if err != nil {
		t.Fatal(err)
	}
	if nPulled != 2 {
		t.Fatalf("expected to pull 1 got %d", nPulled)
	}

	c, err = ag2.GetLatestByServerId(1, l2)
	if err != nil {
		t.Fatal(err)
	}
	if c.IsDetached {
		t.Fatal("1 is still detached")
	}
	c, err = ag2.GetLatestByServerId(2, l2)
	if err != nil {
		t.Fatal(err)
	}
	if c.IsDetached {
		t.Fatal("2 is still detached")
	}
}

func TestAmendAndPush(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push a commit
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	c1, _ = ag.GetLatest(1, l)
	if !c1.HasServerL {
		t.Fatal("should have ServerL")
	}
	if c1.ServerL != 1 || c1.ServerV != 0 {
		t.Fatal("wrong server vals")
	}
	srvC1 := srv.GetLatest(1)
	if srvC1.L != 1 || srvC1.Version != 0 {
		t.Fatal("wrong L or V")
	}
	if !srvC1.HasServerL {
		t.Fatal("should have ServerL")
	}
	if srvC1.ServerL != 1 || srvC1.ServerV != 0 {
		t.Fatal("wrong server vals")
	}

	// Commit should be pending on the server
	if !srv.IsPending(1) {
		t.Fatal("c1 should be pending on the server")
	}

	// Amend the commit and push
	wd.WriteFile("a.txt", "AAA")
	c1New, _ := ag.Amend(&c1, true, wd, "c1'", l)
	if c1.ObsReason != commit.ObsoleteReasonAmend {
		t.Fatal("c1 was ammended")
	}
	_, _, err := ag.Push(&c1New, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	c1New, _ = ag.GetLatest(1, l)
	if !c1New.HasServerL {
		t.Fatal("should have Server L")
	}
	if c1New.ServerL != 1 || c1New.ServerV != 1 {
		t.Fatal("wrong server vals")
	}

	// The ammended should still be pending on the server
	if !srv.IsPending(1) {
		t.Fatal("c1 should be pending on the server")
	}

	srvC1 = srv.GetLatest(1)
	if srvC1.L != 1 || srvC1.Version != 1 {
		t.Fatal("wrong L or V")
	}
	if srvC1.Birth != commit.BirthReasonAmend {
		t.Fatal("c1 was born of an amend")
	}
	if !srvC1.HasServerL {
		t.Fatal("should have ServerL")
	}
	if srvC1.ServerL != 1 || srvC1.ServerV != 1 {
		t.Fatal("wrong server vals")
	}
}

func TestPendingIsUpdatedWhenAmmendedAndPushed(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push c1_v0
	wd.WriteFile("a.txt", "aaa")
	c1V0, _ := ag.Commit(wd, "c1_v0", &root, l)
	ag.Push(&c1V0, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	// Amend and push
	wd.WriteFile("a.txt", "AAA")
	c1V1, _ := ag.Amend(&c1V0, true, wd, "c1_v1", l)
	ag.Push(&c1V1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)

	iter, closeIter := srv.Pending()
	defer closeIter()
	if !iter.Next() {
		t.Fatal("c1 should be pending")
	}
	c, err := iter.Get()
	if err != nil {
		t.Fatal(err)
	}
	if c.Message != "c1_v1" {
		t.Fatal("wrong message")
	}
	if c.L != 1 {
		t.Fatal("wrong id")
	}
	if c.Version != 1 {
		t.Fatal("wrong version")
	}
}

func TestParentCl(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push 2 commits.
	// Both are children of root:
	// c1  c2
	// |  /
	// root
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	wd.WriteFile("b.txt", "bbb")
	c2, _ := ag.Commit(wd, "c2", &root, l)
	ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)

	if srv.GetLatest(1).ParentL != 0 {
		t.Fatal("ParentCl of 1 should be 0")
	}
	if srv.GetLatest(2).ParentL != 0 {
		t.Fatal("ParentCl of 2 should be 0")
	}

	// Submit CL 2 first
	srv.Submit(2)
	c2 = srv.GetLatest(2)
	if c2.ParentL != 0 {
		t.Fatal("ParentCl of 2 should be 0")
	}
	if c2.Birth != commit.BirthReasonSubmit {
		t.Fatal("c2 was submitted")
	}
	if srv.GetLatest(1).ParentL != 0 {
		t.Fatal("ParentCl of 1 should still be 0")
	}
	// Now submit CL 1
	srv.Submit(1)
	cl1 := srv.GetLatest(1)
	if cl1.Version != 1 {
		t.Fatal("since CL 1 was rebased, it's version changed")
	}
	if cl1.ParentL != c2.L {
		t.Fatal("the parent should be c2")
	}
	if cl1.Birth != commit.BirthReasonSubmit {
		t.Fatal("c1 was submitted")
	}

}

func TestGetVersion(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Write and push 1 commit.
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	// Ammend and push again
	newC1, _ := ag.Amend(&c1, true, wd, "ammend c1", l)
	ag.Push(&newC1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)

	if srv.GetVersion(1, 1).Version != 1 {
		t.Fatal("GetVersion 1 failed")
	}
	if srv.GetVersion(1, 0).Version != 0 {
		t.Fatal("GetVersion 1 failed")
	}

}

func TestAmmendMultipleTimesAndManyClients(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	// Client 1 pushes c11
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)
	wd1.WriteFile("a.txt", "aaa")
	c1_0, _ := ag1.Commit(wd1, "c11", &root1, l1)
	ag1.Push(&c1_0, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	// Client 1 ammends multiple times, but pushed only once
	c1_1, _ := ag1.Amend(&c1_0, true, wd1, "amend 1: msg only", l1)
	wd1.WriteFile("a.txt", "AAA")
	c1_2, _ := ag1.Amend(&c1_1, true, wd1, "amend 2: change file", l1)
	wd1.WriteFile("c.txt", "ccc")
	c1_3, _ := ag1.Amend(&c1_2, true, wd1, "amend 3: add file", l1)
	c1_4, _ := ag1.Amend(&c1_3, true, wd1, "last ammend", l1)
	ag1.Push(&c1_4, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)

	// Client 2 pushes c22
	root2, ag2, wd2, l2 := client.NewTest("owner", 2, t)
	wd2.WriteFile("b.txt", "b")
	c22, _ := ag2.Commit(wd2, "c2", &root2, l2)
	ag2.Push(&c22, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l2)

	// CL1 will be the second ammend of c11.
	// Not that only 2 versions were pushed
	cl1 := srv.GetLatest(1)
	if cl1.Message != "last ammend" {
		t.Fatal("wrong cl1")
	}
	if cl1.Version != 1 {
		t.Fatal("wrong version for cl1")
	}

	// CL2 is the second pushed by the other client
	cl2 := srv.GetLatest(2)
	if cl2.Message != "c2" {
		t.Fatal("wrong cl2")
	}
	if cl2.Version != 0 {
		t.Fatal("wrong version for cl2")
	}
}

func TestAmmendParentAndPush(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	// Client pushes c1 and c2.
	// c1 is parent of c2
	root, ag, wd, l := client.NewTest("owner", 1, t)
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	wd.WriteFile("b.txt", "bbbb")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)

	// Both start out as version 0 on the server
	if srv.GetLatest(1).Version != 0 {
		t.Fatal("cl1 wrong initial version on server")
	}
	if srv.GetLatest(2).Version != 0 {
		t.Fatal("cl2 wrong initial version on server")
	}

	// Client now amends c1 (parent of c2) using auto-rebase for the children
	// and pushes. Children are not pushed, so commit 2 should not yet be pushed
	newC1, _ := ag.Amend(&c1, true, wd, "amend c1", l)
	ag.Push(&newC1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if srv.GetLatest(1).Version != 1 {
		t.Fatal("c1 wrong version on server after amend push")
	}
	if srv.GetLatest(2).Version != 0 {
		t.Fatal("c2 should noy yet change version")
	}
	// Now push the child that was auto rebased
	newC2, _ := ag.GetLatest(2, l)
	ag.Push(&newC2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if srv.GetLatest(2).Version != 1 {
		t.Fatal("c2New not pushed")
	}
}

func TestPushCommitWithOldBase(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Client creates c1, then c2.
	// Then pushes starting from c2. c1 and c2 will be pushed.
	// c1 creates a.txt
	// c2 modifies a.txt
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("a.txt", "aaaAAAA")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	// Submit c1 and c2
	srv.Submit(1)
	srv.Submit(2)
	// Pulling will mark the commits as submitted
	ag.PullAllSubmittedAfter(root, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, nil, l)
	c1, _ = ag.GetLatest(c1.L, l)
	if !c1.IsSubmitted {
		t.Fatal("c1 should be submitted")
	}
	c2, _ = ag.GetLatest(c2.L, l)
	if !c2.IsSubmitted {
		t.Fatal("c2 should be submitted")
	}

	// Create c3 as child of c1
	// c3 creates b.txt
	//
	// c2
	// |  c3
	// c1/
	// |
	// ~
	wd.WriteFile("a.txt", "aaa")
	wd.WriteFile("b.txt", "bbbb")
	c3, _ := ag.Commit(wd, "c3", &c1, l)
	// Push using c1 as base. Since its submitted, we can use it.
	_, _, err := ag.Push(&c3, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	// Diffing c3 and c2 should show b created and a modified:
	// in c2: a.txt=aaaAAA
	// in c3: a.txt=aaa, b.txt=bbb
	serverC3 := srv.GetLatest(c3.L)
	serverC2 := srv.GetLatest(c2.L)
	diffs := srv.Diff(serverC3, serverC2)
	checkDiffs(diffs,
		[]string{tree.RootPath, "a.txt", "b.txt"},
		[]uint32{0, 1, 1},
		[]tree.DiffType{tree.DiffTypeAnyModified,
			tree.DiffTypeAnyModified, tree.DiffTypeCreated},
		t,
	)
}

func TestPushWithConflicts(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create two commits, one with conflict, and push.
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("a.txt", "aaaAAAA")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	wd.WriteFile("a.txt", "bbbbb")
	_, err := ag.Amend(&c1, true, wd, "amend c1 to conflict c2", l)
	if err != nil {
		t.Fatal(err)
	}
	// Get the refreshed ones
	c1, err = ag.GetLatest(1, l)
	if err != nil {
		t.Fatal(err)
	}
	c2, err = ag.GetLatest(2, l)
	if err != nil {
		t.Fatal(err)
	}
	if !c2.HasRebaseConflicts {
		t.Fatal("c2 should have a conflicts")
	}
	// Oush both. Pushing a commit with conflicts is ok.
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	_, _, err = ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	// Submit c1 is ok, but c2 can't be submitted bc it has a conflict
	srv.Submit(1)
	if srv.CanSubmit(2) {
		t.Fatal("c2 cant be submitted bc it has a conflict")
	}
}

func TestDiffVersions(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create the following commit graph:
	//
	// c2
	// |
	// c1   c3
	// |    |
	// root-/
	wd.WriteFile("c1.txt", "c2")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	wd.WriteFile("c2.txt", "c2")
	c2, _ := ag.Commit(wd, "c2", &c1, l)
	wd.Delete("c1.txt")
	wd.Delete("c2.txt")
	wd.WriteFile("c3.txt", "c3.txt")
	c3, _ := ag.Commit(wd, "c3", &root, l)

	// Push all to the server
	ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	ag.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	ag.Push(&c3, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)

	// Submit them in order. Note that c3 will be rebased:
	// * -> obsolete commit
	//
	//      c3v1
	//      |
	// c2*  c2v1
	// |    |
	// c1*  c1v1  c3*
	// |    |     |
	// root-/-----/

	srv.Submit(1)
	srv.Submit(2)
	srv.Submit(3)

	// Now lets check the diffs of c3.
	c2V1 := srv.GetLatest(2)
	c3V1 := srv.GetLatest(3)
	c3V0 := srv.GetVersion(3, 0)

	// When diffing agains the parent, we'll see that c3v1 crated c3.txt
	c3v1MinusC2V1 := srv.Diff(c3V1, c2V1)
	checkDiffs(c3v1MinusC2V1,
		[]string{tree.RootPath, "c3.txt"},
		[]uint32{0, 1},
		[]tree.DiffType{tree.DiffTypeAnyModified,
			tree.DiffTypeCreated}, t)

	// When diffing commits of the same Id, we'll only compare what they
	// changed with respect to their parents. c3v0 only created c3.txt on top
	// of root. c3v1 is a rebase of c3v0 onto c2v1; so when diffing c3v0 and
	// c3v1 we should get an empty diff
	c3v1MinusC3V0 := srv.Diff(c3V1, c3V0)
	checkDiffs(c3v1MinusC3V0,
		[]string{tree.RootPath},
		[]uint32{0},
		[]tree.DiffType{tree.DiffTypeAnyModified}, t)
}

func TestBadApiKey(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root1, ag1, _, l := client.NewTest("owner", 1, t)

	invalidApiKey := testApiKey + "invalid"

	isBadApiKeyErr, isOldProtocolErr, err := ag1.PullAllSubmittedAfter(root1, srv.RootUrl()+"/"+srv.ServerPath(), invalidApiKey, nil, l)
	if err == nil || !isBadApiKeyErr || isOldProtocolErr {
		t.Fatalf("expected bad api key error, got: %s", err)
	}

	fakeCommit := commit.Commit{
		L: 10,
	}
	_, isBadApiKeyErr, err = ag1.Push(&fakeCommit, srv.RootUrl()+"/"+srv.ServerPath(), invalidApiKey, l)
	if err == nil || !isBadApiKeyErr {
		t.Fatalf("expected bad api key error, got: %s", err)
	}
}

func TestLoad(t *testing.T) {
	srv := NewTestServer(testApiKey, t)

	root, ag, wd, l := client.NewTest("owner", 1, t)

	// Create the commit c1 and push it
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	// Load the commit to an arbitrary folder.
	// Use a TestWorkdir just bc it auto cleans up and allows to easily
	// check the contents
	serverWorkdir := workdir.NewTest("test_server_workdir", t)
	srv.Load(1, serverWorkdir)
	if !serverWorkdir.HasFile("a.txt") {
		t.Fatal("expected to find file a.txt")
	}
	if serverWorkdir.ReadFile("a.txt") != "aaa" {
		t.Fatal("unexpected contents of a.txt")
	}
}

func TestSimpleRollback(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root, ag, wd, l := client.NewTest("owner", 1, t)
	c0RootDirHash := wd.RootDirHash()

	// Create the commit, push and submit it
	wd.WriteFile("a.txt", "aaa")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}
	srv.Submit(1)

	// Create a rollback of c1. It reverts the c1, i.e. turns the workdir into
	// an empty one.
	// Use some fake variables for variables we can't really control
	buildmeta.Version = "server-build-version"
	fakeMsg := ""
	fakeTime := time.Now()
	authorId := int64(99)
	rb := srv.CreateRollback(1, authorId)
	rb.CreatedOn = fakeTime
	rb.Message = fakeMsg
	expectedRb := commit.Commit{
		Birth:                  commit.BirthReasonRollback,
		L:                      2,
		Version:                0,
		HasServerL:             true,
		ServerL:                2,
		HasServerV:             true,
		ServerV:                0,
		TreeVersion:            2,
		RootDirHash:            c0RootDirHash,
		ParentL:                1,
		ParentV:                1,
		HasParentServerL:       true,
		ParentServerL:          1,
		HasParentServerV:       true,
		ParentServerV:          1,
		ParentTreeVersion:      1,
		Status:                 commit.StatusLatest,
		ObsReason:              commit.ObsoleteReasonNone,
		Message:                fakeMsg,
		CreatedOn:              fakeTime,
		HasRebaseConflicts:     false,
		Children:               nil,
		IsSubmitted:            false,
		ServerBuildVersion:     "server-build-version",
		AuthorUserId:           authorId,
		IsRollbackOfL:          1,
		IsRollbackOfV:          1,
		FirstVersionIsRollback: true,
		HasDiffData:            true,
		DiffDataLinesDeleted:   1,
		DiffDataFilesDeleted:   1,
	}
	if !reflect.DeepEqual(rb, expectedRb) {
		t.Fatalf("expected commit %+v got %+v", expectedRb, rb)
	}
	srv.Load(rb.L, wd)
	if wd.HasFile("a.txt") {
		t.Fatal("should not have file")
	}

	// The top commit should not be modified (children are attached on submit)
	top := srv.GetLatest(1)
	if len(top.Children) != 0 {
		t.Fatal("child was added")
	}
}

// Excludes the no-change and undefined diffs
func checkDiffs(
	diffs []tree.Diff,
	expectedBaseNames []string,
	expectedDepths []uint32,
	expectedTypes []tree.DiffType, t testing.TB) {
	t.Helper()

	// baseName -> number of times that they appeared
	baseNameToNumberOfTimesChecked := make(map[string]int)
	directoryBaseNames := make(map[string]bool)
	for _, d := range diffs {
		if d.Type == tree.DiffTypeNoChange || d.Type == tree.DiffTypeUndefined {
			continue
		}

		i := slices.Index(expectedBaseNames, d.Data.BaseName)
		if i < 0 {
			t.Fatalf("got unexpected base name: %s", d.Data.BaseName)
		}
		if expectedDepths[i] != d.Data.Depth {
			t.Fatalf("expected depth %d for %s, got %d",
				expectedDepths[i], d.Data.BaseName, d.Data.Depth)
		}
		if expectedTypes[i] != d.Type {
			t.Fatalf("expected type %d for %s, got %d",
				expectedTypes[i], d.Data.BaseName, d.Type)
		}
		n, ok := baseNameToNumberOfTimesChecked[d.Data.BaseName]
		if !ok {
			baseNameToNumberOfTimesChecked[d.Data.BaseName] = 1
		} else {
			baseNameToNumberOfTimesChecked[d.Data.BaseName] = n + 1
		}
		if d.Data.IsDir {
			directoryBaseNames[d.Data.BaseName] = true
		}
	}
	if len(baseNameToNumberOfTimesChecked) != len(expectedBaseNames) {
		for _, expectedBaseName := range expectedBaseNames {
			n, ok := baseNameToNumberOfTimesChecked[expectedBaseName]
			if !ok {
				t.Fatalf("got no diff for %s", expectedBaseName)
			}

			_, ok = directoryBaseNames[expectedBaseName]
			if ok {
				if n != 2 {
					t.Fatalf("%s is a directory but only appeared %d (expect 2)",
						expectedBaseName, n)
				}
			}
		}
		panic("checkDiffs has some bug")
	}
}
func TestRenameCommit(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root, ag, wd, l := client.NewTest("owner", 1, t)

	// create a commit and push it
	wd.WriteFile("a.txt", "hello")
	c1, _ := ag.Commit(wd, "original message", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	const authorId = int64(42)
	srv.RenameCommit(1, "new message", authorId)

	latest := srv.GetLatest(1)
	if latest.Message != "new message" {
		t.Fatalf("expected message %q got %q", "new message", latest.Message)
	}
	if latest.Version != 1 {
		t.Fatalf("expected version 1 got %d", latest.Version)
	}
	if latest.L != 1 {
		t.Fatalf("expected same L=1 got %d", latest.L)
	}
	if latest.AuthorUserId != authorId {
		t.Fatalf("expected authorId %d got %d", authorId, latest.AuthorUserId)
	}
	if !latest.HasServerV {
		t.Fatal("renamed commit must have HasServerV=true")
	}
	if latest.ServerV != 1 {
		t.Fatalf("expected ServerV 1 got %d", latest.ServerV)
	}

	// Tree must be unchanged
	original := srv.GetVersion(1, 0)
	if latest.TreeVersion != original.TreeVersion {
		t.Fatal("tree version changed after rename")
	}
	if latest.RootDirHash != original.RootDirHash {
		t.Fatal("root dir hash changed after rename")
	}

	// Old version must be obsolete
	if original.Status == commit.StatusLatest {
		t.Fatal("old version should be obsolete after rename")
	}

	// Now Submitting the `latest` and trying to rename again must fail
	srv.Submit(1)

	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	sl := srv.BindW(w)
	_, err = srv.GetServer().RenameCommit(1, "yet another message", authorId, sl)
	if err == nil {
		t.Fatal("expected error renaming a submitted commit")
	}
}

func TestRenameCommitFailsOnInvalidMessage(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root, ag, wd, l := client.NewTest("owner", 1, t)

	// create a commit and push it
	wd.WriteFile("a.txt", "hello")
	c1, _ := ag.Commit(wd, "c1", &root, l)
	_, _, err := ag.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l)
	if err != nil {
		t.Fatal(err)
	}

	w, closeW, _ := srv.BeginWrite()
	defer closeW()
	sl := srv.BindW(w)

	// try to rename with empty message
	_, err = srv.GetServer().RenameCommit(1, "", 0, sl)
	if err == nil {
		t.Fatal("expected error for empty message")
	}

	// trying to rename with a too long  message
	longMsg := strings.Repeat("a", client.MaxMsgLen+1)
	_, err = srv.GetServer().RenameCommit(1, longMsg, 0, sl)
	if err == nil {
		t.Fatal("expected error for too long message")
	}

	// exactly MaxMsgLen characters with multi-byte UTF-8 chars must succeed
	msg50Accented := strings.Repeat("é", client.MaxMsgLen) // each "é" is 2 bytes, but 1 char
	_, err = srv.GetServer().RenameCommit(1, msg50Accented, 0, sl)
	if err != nil {
		t.Fatalf("expected success for %d-char accented message, got: %v", client.MaxMsgLen, err)
	}

	// renaming to the same message (msg50Accented) must fail
	_, err = srv.GetServer().RenameCommit(1, msg50Accented, 0, sl)
	if err == nil {
		t.Fatal("expected error when renaming to the same message")
	}
}
func TestSetNextServerId(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)

	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	_, _, err := ag1.Push(&c1, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	if err != nil {
		t.Fatal(err)
	}

	w, closeW, commitW := srv.BeginWrite()
	sl := srv.BindW(w)
	s := srv.GetServer()
	if err := s.SetNextServerId(c1.ServerL, sl); err == nil {
		t.Fatal("expected error when id is not greater than the next id")
	}
	if err := s.SetNextServerId(c1.ServerL+20_000, sl); err == nil {
		t.Fatal("expected error when id is too far ahead of the next id")
	}
	if err := s.SetNextServerId(100, sl); err != nil {
		t.Fatal(err)
	}
	if err := commitW(); err != nil {
		t.Fatal(err)
	}
	closeW()

	// The next pushed commit must get the new id
	wd1.WriteFile("b.txt", "bbb")
	c2, _ := ag1.Commit(wd1, "c2", &c1, l1)
	_, _, err = ag1.Push(&c2, srv.RootUrl()+"/"+srv.ServerPath(), testApiKey, l1)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ServerL != 100 {
		t.Fatalf("expected pushed commit to get server id 100, got %d", c2.ServerL)
	}
}
func TestClientSetNextServerId(t *testing.T) {
	srv := NewTestServer(testApiKey, t)
	root1, ag1, wd1, l1 := client.NewTest("owner", 1, t)
	url := srv.RootUrl() + "/" + srv.ServerPath()

	// Refused: 0 is not greater than the server's next id
	notOkMsg, err := ag1.SetNextServerId(url, testApiKey, 0)
	if err != nil {
		t.Fatal(err)
	}
	if notOkMsg == "" {
		t.Fatal("expected a notOkMsg when id is not greater than the next id")
	}

	// Refused: bad api key
	notOkMsg, err = ag1.SetNextServerId(url, "wrong-key", 100)
	if err != nil {
		t.Fatal(err)
	}
	if notOkMsg == "" {
		t.Fatal("expected a notOkMsg when the api key is invalid")
	}

	// Happy path: the next pushed commit gets the requested id
	notOkMsg, err = ag1.SetNextServerId(url, testApiKey, 100)
	if err != nil {
		t.Fatal(err)
	}
	if notOkMsg != "" {
		t.Fatalf("unexpected notOkMsg: %s", notOkMsg)
	}
	wd1.WriteFile("a.txt", "aaa")
	c1, _ := ag1.Commit(wd1, "c1", &root1, l1)
	_, _, err = ag1.Push(&c1, url, testApiKey, l1)
	if err != nil {
		t.Fatal(err)
	}
	if c1.ServerL != 100 {
		t.Fatalf("expected pushed commit to get server id 100, got %d", c1.ServerL)
	}
}