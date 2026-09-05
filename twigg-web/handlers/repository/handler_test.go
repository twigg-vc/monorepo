package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestHandleGet_RendersHaveMorePendingCommits(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	// Top commit
	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 10}, nil
	}
	// Create enough commits to trigger "have more"
	commits := make([]commit.Commit, maxPendingCommitsPageSize+1)
	for i := range commits {
		commits[i] = commit.Commit{
			L:       uint64(i + 1),
			ServerL: uint64(i + 1),
			Message: "ok",
		}
	}
	repoS.getRepoPendingCommits = func() (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice(commits), nil
	}
	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	req := newMockReq(nil, nil)
	h.handleGet(w, req, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	body := w.Body.String()

	if !strings.Contains(body, "HaveMorePendingCommitsToFetch") {
		t.Fatalf("expected HaveMorePendingCommitsToFetch attribute in HTML")
	}
}
func TestHandleGet_DoesNotRendersHaveMorePendingCommits(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	// Top commit
	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 10}, nil
	}
	// Create max pending commits, does not have more
	commits := make([]commit.Commit, maxPendingCommitsPageSize)
	for i := range commits {
		commits[i] = commit.Commit{
			L:       uint64(i + 1),
			ServerL: uint64(i + 1),
			Message: "ok",
		}
	}
	repoS.getRepoPendingCommits = func() (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice(commits), nil
	}
	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	req := newMockReq(nil, nil)
	h.handleGet(w, req, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	body := w.Body.String()

	if strings.Contains(body, "HaveMorePendingCommitsToFetch") {
		t.Fatalf("unexpected HaveMorePendingCommitsToFetch attribute in HTML")
	}
}
func TestHandleGet_NoMorePendingCommits(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 10}, nil
	}
	// Less than page size
	commits := []commit.Commit{
		{L: 1, ServerL: 1, Message: "ok"},
	}

	repoS.getRepoPendingCommits = func() (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice(commits), nil
	}
	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	req := newMockReq(nil, nil)
	h.handleGet(w, req, nil)

	body := w.Body.String()

	if strings.Contains(body, "HaveMorePendingCommitsToFetch") {
		t.Fatalf("did NOT expect HaveMorePendingCommitsToFetch attribute")
	}
}
func TestHandleGet_SupremeLeadersPassedToGetData(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{}, nil
	}
	repoS.getRepoPendingCommits = func() (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice([]commit.Commit{}), nil
	}

	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{"alice", "bob"}, nil
	}

	var capturedLeaders []string
	revS.getData = func(supremeLeaders []string) (review.Data, bool, error) {
		capturedLeaders = supremeLeaders
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	req := newMockReq(nil, nil)
	h.handleGet(w, req, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !reflect.DeepEqual(capturedLeaders, []string{"alice", "bob"}) {
		t.Fatalf("expected supreme leaders [alice bob] to be passed to GetData, got %v", capturedLeaders)
	}
}
func TestHandleSearchCommitsInvalidQueries(t *testing.T) {
	repoS := repoServiceMock{}
	revS := reviewServiceMock{}
	userS := userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	checkBadQuery := func(searchQuery string) {
		w := httptest.NewRecorder()
		q := url.Values{}
		q.Set(routes.RepoSearchCommitsSeachQueryQueryParamName, searchQuery)
		pathValues := map[string]string{}
		req := newMockReq(pathValues, q)
		h.HandleSearchCommits(w, req, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected bad req got %d", w.Code)
		}
	}
	// Bad request for random text
	checkBadQuery("abcde")
	// Bad request if commitId is invalid
	checkBadQuery("-1")
	// Bad request if commitVersion is negative
	checkBadQuery("1v-9")
}

func TestHandleSearchCommitsGetsLatest(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	// Mock:
	// c2v0* c2v1
	// |     |
	// c1v0* c1v1
	// |     |
	// root--/
	repoS.getRepoCommitVersion = func(n, v uint64) (commit.Commit, error) {
		switch n {
		case 0:
			if v != 0 {
				return commit.Commit{}, errors.New("not found")
			}
			return commit.Commit{
				L: 0, ServerL: 0,
				Version: 0, ServerV: 0,
				ParentL: 0, ParentServerL: 0,
				ParentV: 0, ParentServerV: 0,
			}, nil
		case 1:
			return commit.Commit{
				L: 1, ServerL: 1,
				Version: v, ServerV: v,
				ParentL: 0, ParentServerL: 0,
				ParentV: 0, ParentServerV: 0,
			}, nil
		case 2:
			return commit.Commit{
				L: 2, ServerL: 2,
				Version: v, ServerV: v,
				ParentL: 1, ParentServerL: 1,
				ParentV: v, ParentServerV: v,
			}, nil
		default:
			return commit.Commit{}, errors.New("not found")
		}
	}
	repoS.getRepoCommit = func(n uint64) (commit.Commit, error) {
		if n == 0 {
			return repoS.getRepoCommitVersion(0, 0)
		}
		return repoS.getRepoCommitVersion(n, 1)
	}
	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return repoS.getRepoCommit(2)
	}
	revS.getData = func(_ []string) (d review.Data, isNotFoundErr bool, err error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (u user.User, isNotFoundErr bool, err error) {
		return user.User{Username: "me"}, false, nil
	}

	// Helper to check response
	checkResp := func(query string, expected []twiggwc.FrontendCommit) {
		w := httptest.NewRecorder()
		q := url.Values{}
		q.Set(routes.RepoSearchCommitsSeachQueryQueryParamName, query)
		pathValues := map[string]string{}
		req := newMockReq(pathValues, q)
		h.HandleSearchCommits(w, req, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected ok req got %d", w.Code)
		}
		var gotResp []twiggwc.FrontendCommit
		if err := json.NewDecoder(w.Body).Decode(&gotResp); err != nil {
			t.Fatalf("query %q: failed to decode response: %v", query, err)
		}
		if !reflect.DeepEqual(gotResp, expected) {
			t.Errorf("query %q: \ngot:  %+v\nwant: %+v", query, gotResp, expected)
		}
	}

	// Empty query returns the top and a few parents
	checkResp("", []twiggwc.FrontendCommit{
		{
			L:              2,
			Version:        1,
			ParentL:        1,
			AuthorUsername: "me",
			ReviewStatus:   "ready",
		},
		{
			L:              1,
			Version:        1,
			ParentL:        0,
			AuthorUsername: "me",
			ReviewStatus:   "ready",
		},
		{
			L:              0,
			Version:        0,
			ParentL:        0,
			AuthorUsername: "",
			ReviewStatus:   "ready",
		},
	})

	// If a commit number is specified, return its latest v
	checkResp("c2", []twiggwc.FrontendCommit{
		{
			L:              2,
			Version:        1,
			ParentL:        1,
			AuthorUsername: "me",
			ReviewStatus:   "ready",
		},
	})
	checkResp("c/1", []twiggwc.FrontendCommit{
		{
			L:              1,
			Version:        1,
			ParentL:        0,
			AuthorUsername: "me",
			ReviewStatus:   "ready",
		},
	})

	// If a version is also specified, return it
	checkResp("c2v0", []twiggwc.FrontendCommit{
		{
			L:              2,
			Version:        0,
			ParentL:        1,
			AuthorUsername: "me",
			ReviewStatus:   "ready",
		},
	})
	checkResp("c/1v0", []twiggwc.FrontendCommit{
		{
			L:              1,
			Version:        0,
			ParentL:        0,
			AuthorUsername: "me",
			ReviewStatus:   "ready",
		},
	})
}
func TestHandleGetMorePending_BadAfterId(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "invalid")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}
func TestHandleGetMorePending_GetTopCommitError(t *testing.T) {
	repoS := repoServiceMock{
		getRepoTopCommit: func() (commit.Commit, error) {
			return commit.Commit{}, errors.New("boom!")
		},
	}
	h := NewHandler(repoS, reviewServiceMock{}, userServiceMock{})

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "1")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}
func TestHandleGetMorePending_GetPendingError(t *testing.T) {
	repoS := repoServiceMock{
		getRepoTopCommit: func() (commit.Commit, error) {
			return commit.Commit{}, nil
		},
		getRepoPendingCommitsAfter: func(_afterId uint64) (iterator.I[commit.Commit], error) {
			return nil, errors.New("krakatoa")
		},
	}
	h := NewHandler(repoS, reviewServiceMock{}, userServiceMock{})

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "1")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}
func TestHandleGetMorePending_Success(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 10}, nil
	}

	var getRepoPendingCommitsAfterCalledWithAfterId uint64
	commits := []commit.Commit{
		{L: 1, ServerL: 1, Message: "ok"},
		{L: 2, ServerL: 2, Message: "ok"},
		{L: 3, ServerL: 3, Message: "ok"},
	}
	repoS.getRepoPendingCommitsAfter = func(_afterId uint64) (iterator.I[commit.Commit], error) {
		getRepoPendingCommitsAfterCalledWithAfterId = _afterId
		return iterator.NewIterFromSlice(commits[1:]), nil
	}

	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "1")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	var resp getMorePendingResponse

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.PendingFrontendCommits) != 2 {
		t.Fatalf("expected 2 commits got %d", len(resp.PendingFrontendCommits))
	}
	if resp.PendingFrontendCommits[0].L != commits[1].L ||
		resp.PendingFrontendCommits[0].Message != commits[1].Message {
		t.Fatalf("unexpected resp.PendingFrontendCommits[0], got: %#v", resp.PendingFrontendCommits[0])
	}
	if resp.PendingFrontendCommits[1].L != commits[2].L ||
		resp.PendingFrontendCommits[1].Message != commits[2].Message {
		t.Fatalf("unexpected resp.PendingFrontendCommits[1], got: %#v", resp.PendingFrontendCommits[1])
	}
	if resp.HaveMorePendingCommitsToFetch {
		t.Fatalf("unexpected resp.HaveMorePendingCommitsToFetch=false")
	}

	if getRepoPendingCommitsAfterCalledWithAfterId != 1 {
		t.Fatalf("unexpected getRepoPendingCommitsAfterCalledWithAfterId, got: %d", getRepoPendingCommitsAfterCalledWithAfterId)
	}
}
func TestHandleGetMorePending_FiltersHiddenMessages(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 10}, nil
	}

	commits := []commit.Commit{
		{L: 1, ServerL: 1, Message: msgPrefixToHidePendingCommit + " hidden"},
		{L: 2, ServerL: 2, Message: "visible"},
	}

	repoS.getRepoPendingCommitsAfter = func(_afterId uint64) (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice(commits), nil
	}
	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "0")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	var resp getMorePendingResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.PendingFrontendCommits) != 1 {
		t.Fatalf("expected 1 commit got %d", len(resp.PendingFrontendCommits))
	}
	if resp.PendingFrontendCommits[0].L != commits[1].L ||
		resp.PendingFrontendCommits[0].Message != commits[1].Message {
		t.Fatalf("unexpected resp.PendingFrontendCommits[0], got: %#v", resp.PendingFrontendCommits[0])
	}
	if resp.HaveMorePendingCommitsToFetch {
		t.Fatalf("unexpected resp.HaveMorePendingCommitsToFetch=false")
	}
}
func TestHandleGetMorePending_HaveMorePendingCommitsToFetchTrue(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 100}, nil
	}

	// Create maxPendingCommitsPageSize+1 commits
	commits := make([]commit.Commit, maxPendingCommitsPageSize+1)
	for i := 0; i < maxPendingCommitsPageSize; i++ {
		commits[i] = commit.Commit{
			L:       uint64(i + 1),
			ServerL: uint64(i + 1),
			Message: "ok",
		}
	}

	repoS.getRepoPendingCommitsAfter = func(_afterId uint64) (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice(commits), nil
	}

	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "0")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	var resp getMorePendingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.PendingFrontendCommits) != maxPendingCommitsPageSize {
		t.Fatalf("expected %d commits got %d", maxPendingCommitsPageSize, len(resp.PendingFrontendCommits))
	}

	if !resp.HaveMorePendingCommitsToFetch {
		t.Fatalf("expected HaveMorePendingCommitsToFetch=true")
	}
}
func TestHandleGetMorePending_HaveMorePendingCommitsToFetchFalse(t *testing.T) {
	repoS := &repoServiceMock{}
	revS := &reviewServiceMock{}
	userS := &userServiceMock{}
	h := NewHandler(repoS, revS, userS)

	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{ServerL: 100}, nil
	}

	// Create maxPendingCommitsPageSize commits
	commits := make([]commit.Commit, maxPendingCommitsPageSize)
	for i := 0; i < maxPendingCommitsPageSize; i++ {
		commits[i] = commit.Commit{
			L:       uint64(i + 1),
			ServerL: uint64(i + 1),
			Message: "ok",
		}
	}

	repoS.getRepoPendingCommitsAfter = func(_afterId uint64) (iterator.I[commit.Commit], error) {
		return iterator.NewIterFromSlice(commits), nil
	}

	revS.getData = func(_ []string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	userS.get = func() (user.User, bool, error) {
		return user.User{Username: "me"}, false, nil
	}

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "0")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}

	var resp getMorePendingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(resp.PendingFrontendCommits) != maxPendingCommitsPageSize {
		t.Fatalf("expected %d commits got %d", maxPendingCommitsPageSize, len(resp.PendingFrontendCommits))
	}

	if resp.HaveMorePendingCommitsToFetch {
		t.Fatalf("expected HaveMorePendingCommitsToFetch=false")
	}
}
func TestHandleGetMorePending_IteratorError(t *testing.T) {
	repoS := &repoServiceMock{}
	userS := &userServiceMock{}
	revS := &reviewServiceMock{}
	h := NewHandler(repoS, revS, userS)

	userS.get = func() (u user.User, isNotFoundErr bool, err error) {
		return user.User{Username: "me"}, false, nil
	}
	revS.getData = func(_ []string) (d review.Data, isNotFoundErr bool, err error) {
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		return []string{ownerUsr.Username}, nil
	}
	repoS.getRepoTopCommit = func() (commit.Commit, error) {
		return commit.Commit{}, nil
	}

	iter := iterator.NewIterFromSlice([]commit.Commit{
		{L: 1, ServerL: 1},
	})

	// wrap to force Err()
	badIter := &iteratorWithErr{I: iter, err: errors.New("iter fail")}

	repoS.getRepoPendingCommitsAfter = func(afterId uint64) (iterator.I[commit.Commit], error) {
		return badIter, nil
	}

	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set("after-commit", "10")

	req := newMockReq(nil, q)

	h.handleGetMorePending(w, req, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", w.Code)
	}
}

type iteratorWithErr struct {
	iterator.I[commit.Commit]
	err error
}

func (i *iteratorWithErr) Err() error {
	return i.err
}

type repoServiceMock struct {
	getRepoTopCommit           func() (commit.Commit, error)
	getRepoPendingCommits      func() (iterator.I[commit.Commit], error)
	getRepoPendingCommitsAfter func(afterCommit uint64) (iterator.I[commit.Commit], error)
	getRepoCommit              func(n uint64) (commit.Commit, error)
	getRepoCommitVersion       func(n, v uint64) (commit.Commit, error)
	getRepoFile                func() error
}

func (m repoServiceMock) GetRepoTopCommit(rl context.Context, repoId uint64) (commit.Commit, error) {
	return m.getRepoTopCommit()
}
func (m repoServiceMock) GetRepoPendingCommits(rl context.Context, repoId uint64, ascendingOrder bool) (iterator.I[commit.Commit], error) {
	return m.getRepoPendingCommits()
}
func (m repoServiceMock) GetRepoPendingCommitsAfter(rl context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	return m.getRepoPendingCommitsAfter(afterId)
}

func (m repoServiceMock) GetRepoCommit(rl context.Context, repoId uint64, n commit.LocalId) (commit.Commit, error) {
	return m.getRepoCommit(n)
}
func (m repoServiceMock) GetRepoCommitVersion(rl context.Context, repoId uint64, n commit.LocalId, v uint64) (commit.Commit, error) {
	return m.getRepoCommitVersion(n, v)
}
func (m repoServiceMock) GetRepoFile(rl context.Context, repoId uint64, a commit.Commit, filename string, w io.Writer) error {
	return m.getRepoFile()
}

type reviewServiceMock struct {
	getData               func(supremeLeaders []string) (d review.Data, isNotFoundErr bool, err error)
	resolveSupremeLeaders func(ownerUsr user.User) ([]string, error)
}

func (m reviewServiceMock) GetData(r context.Context, repoId uint64, cId commit.LocalId, checkOwners bool,
	cIdToReadOwners uint64, supremeLeaders []string) (review.Data, bool, error) {
	return m.getData(supremeLeaders)
}

func (m reviewServiceMock) ResolveSupremeLeaders(db context.Context, ownerUsr user.User) ([]string, error) {
	return m.resolveSupremeLeaders(ownerUsr)
}

type userServiceMock struct {
	get func() (u user.User, isNotFoundErr bool, err error)
}

func (us userServiceMock) Get(r context.Context, id int64) (u user.User, isNotFoundErr bool, err error) {
	return us.get()
}

func newMockReq(pathValues map[string]string, q url.Values) wrappers.UserWithReadPermissionMuxRequest {
	httpReq := httptest.NewRequest("GET", "/?"+q.Encode(), nil)
	for key, val := range pathValues {
		httpReq.SetPathValue(key, val)
	}
	return wrappers.UserWithReadPermissionMuxRequest{
		Request:                     httpReq,
		MaybeUserWithReadPermission: &user.User{},
		RepoOwnerUsr:                user.User{},
		Repo:                        repo.Repo{},
		Flags:                       featureflags.Flags{},
	}
}
