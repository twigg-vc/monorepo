package repository

import (
	"context"
	"errors"
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
	cr := newCommitRenderer(userS, nil, newMockReq(nil, nil), nil, httptest.NewRecorder())

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
	cr := newCommitRenderer(userS, nil, newMockReq(nil, nil), nil, w)

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
	cr := newCommitRenderer(userS, nil, newMockReq(nil, nil), nil, w)

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
	cr := newCommitRenderer(nil, revS, req, nil, httptest.NewRecorder())

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
	cr := newCommitRenderer(nil, revS, newMockReq(nil, nil), nil, w)

	_, ok := cr.getSupremeLeaders()

	if ok {
		t.Fatalf("getSupremeLeaders is ok, want not ok")
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
