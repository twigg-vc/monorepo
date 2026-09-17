package repository

import (
	"context"
	"errors"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/user"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCommitRenderer_GetAuthorUsernameReadsEachAuthorOnce(t *testing.T) {
	userS := &countingUserServiceMock{
		usersById: map[int64]user.User{
			7: {Username: "leader"},
			8: {Username: "twigger"},
		},
	}
	cr := newCommitRenderer(userS, nil, newMockReq(nil, nil), 0, nil, httptest.NewRecorder())

	for _, want := range []struct {
		authorId int64
		username string
	}{{7, "leader"}, {8, "twigger"}, {7, "leader"}, {8, "twigger"}} {
		got, ok := cr.getAuthorUsername(want.authorId)
		if !ok {
			t.Fatalf("getAuthorUsername(%d) is not ok", want.authorId)
		}
		if got != want.username {
			t.Fatalf("getAuthorUsername(%d) = %q, want %q", want.authorId, got, want.username)
		}
	}
	if userS.getCalls != 2 {
		t.Fatalf("read the users %d times, want 2", userS.getCalls)
	}
}

func TestCommitRenderer_GetAuthorUsernameFailsOnUnknownAuthor(t *testing.T) {
	userS := &countingUserServiceMock{usersById: map[int64]user.User{}}
	w := httptest.NewRecorder()
	cr := newCommitRenderer(userS, nil, newMockReq(nil, nil), 0, nil, w)

	_, ok := cr.getAuthorUsername(7)

	if ok {
		t.Fatalf("getAuthorUsername is ok, want not ok")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCommitRenderer_GetAuthorUsernameFailsOnUserServiceError(t *testing.T) {
	userS := &countingUserServiceMock{err: errors.New("boom")}
	w := httptest.NewRecorder()
	cr := newCommitRenderer(userS, nil, newMockReq(nil, nil), 0, nil, w)

	_, ok := cr.getAuthorUsername(7)

	if ok {
		t.Fatalf("getAuthorUsername is ok, want not ok")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCommitRenderer_GetSupremeLeadersResolvesThemOnce(t *testing.T) {
	var resolveCalls int
	revS := &reviewServiceMock{}
	revS.resolveSupremeLeaders = func(ownerUsr user.User) ([]string, error) {
		resolveCalls++
		return []string{ownerUsr.Username, "leader"}, nil
	}
	req := newMockReq(nil, nil)
	req.RepoOwnerUsr = user.User{Username: "owner"}
	cr := newCommitRenderer(nil, revS, req, 0, nil, httptest.NewRecorder())

	for range 3 {
		got, ok := cr.getSupremeLeaders()
		if !ok {
			t.Fatalf("getSupremeLeaders is not ok")
		}
		want := []string{"owner", "leader"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("getSupremeLeaders() = %v, want %v", got, want)
		}
	}
	if resolveCalls != 1 {
		t.Fatalf("resolved the supreme leaders %d times, want 1", resolveCalls)
	}
}

func TestCommitRenderer_GetSupremeLeadersFailsOnReviewServiceError(t *testing.T) {
	revS := &reviewServiceMock{}
	revS.resolveSupremeLeaders = func(user.User) ([]string, error) {
		return nil, errors.New("boom")
	}
	w := httptest.NewRecorder()
	cr := newCommitRenderer(nil, revS, newMockReq(nil, nil), 0, nil, w)

	_, ok := cr.getSupremeLeaders()

	if ok {
		t.Fatalf("getSupremeLeaders is ok, want not ok")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCommitRenderer_GetReviewStatusPassesTheSupremeLeaders(t *testing.T) {
	revS := &reviewServiceMock{}
	revS.resolveSupremeLeaders = func(user.User) ([]string, error) {
		return []string{"leader"}, nil
	}
	var gotSupremeLeaders []string
	revS.getData = func(supremeLeaders []string) (review.Data, bool, error) {
		gotSupremeLeaders = supremeLeaders
		return review.Data{ReviewStatus: review.ReviewStatus_Ready}, false, nil
	}
	cr := newCommitRenderer(nil, revS, newMockReq(nil, nil), 0, nil,
		httptest.NewRecorder())

	got, ok := cr.getReviewStatus(3)

	if !ok {
		t.Fatalf("getReviewStatus is not ok")
	}
	if got != review.ReviewStatus_Ready {
		t.Fatalf("getReviewStatus() = %v, want %v", got, review.ReviewStatus_Ready)
	}
	if !reflect.DeepEqual(gotSupremeLeaders, []string{"leader"}) {
		t.Fatalf("passed the supreme leaders %v, want [leader]", gotSupremeLeaders)
	}
}

func TestCommitRenderer_GetReviewStatusAcceptsMissingReviewData(t *testing.T) {
	revS := &reviewServiceMock{}
	revS.resolveSupremeLeaders = func(user.User) ([]string, error) {
		return nil, nil
	}
	revS.getData = func([]string) (review.Data, bool, error) {
		return review.Data{ReviewStatus: review.ReviewStatus_MissingLgtm},
			true, errors.New("no review data")
	}
	w := httptest.NewRecorder()
	cr := newCommitRenderer(nil, revS, newMockReq(nil, nil), 0, nil, w)

	got, ok := cr.getReviewStatus(3)

	if !ok {
		t.Fatalf("getReviewStatus is not ok")
	}
	if got != review.ReviewStatus_MissingLgtm {
		t.Fatalf("getReviewStatus() = %v, want %v", got, review.ReviewStatus_MissingLgtm)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCommitRenderer_GetReviewStatusFailsOnGetDataError(t *testing.T) {
	revS := &reviewServiceMock{}
	revS.resolveSupremeLeaders = func(user.User) ([]string, error) {
		return nil, nil
	}
	revS.getData = func([]string) (review.Data, bool, error) {
		return review.Data{}, false, errors.New("boom")
	}
	w := httptest.NewRecorder()
	cr := newCommitRenderer(nil, revS, newMockReq(nil, nil), 0, nil, w)

	_, ok := cr.getReviewStatus(3)

	if ok {
		t.Fatalf("getReviewStatus is ok, want not ok")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status is %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

type countingUserServiceMock struct {
	usersById map[int64]user.User
	err       error
	getCalls  int
}

func (m *countingUserServiceMock) Get(r context.Context, id int64) (
	u user.User, isNotFoundErr bool, err error) {
	m.getCalls++
	if m.err != nil {
		return user.User{}, false, m.err
	}
	u, isFound := m.usersById[id]
	if !isFound {
		return user.User{}, true, errors.New("user not found")
	}
	return u, false, nil
}
