package usersettings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
)

const reqUserId = 1

func TestHandlePostSetUsername_OK(t *testing.T) {
	us := newFakeUserService()
	us.chooseUsernameAndStartTrial = func() (userservice.User, error) {
		return userservice.User{}, nil
	}
	rs := newFakeRepoService()
	rs.nonArchivedRepoCountIsGreaterThan = func(ownerId int64, n int) (bool, error) {
		return false, nil
	}
	var createNewOwnerId int64
	var createNewName string
	var createNewDesc string
	rs.createNew = func(ownerId int64, name string, desc string) (repo.Repo, bool, error) {
		createNewOwnerId = ownerId
		createNewName = name
		createNewDesc = desc
		return repo.Repo{}, false, nil
	}
	h := handler{userS: us, repoS: rs}
	form := url.Values{}
	form.Set(routes.SetUsernameParamName, "Marcos")
	req := httptest.NewRequest("POST", "/", nil)
	req.Form = form
	r := wrappers.UserMuxRequest{
		Request: req,
		User: userservice.User{
			Id:    reqUserId,
			State: user.UserState_NoUsername,
		},
	}
	w := httptest.NewRecorder()
	shouldCommit := h.handlePostSetUsername(w, r, nil)
	if !shouldCommit {
		t.Fatalf("expected commit = true")
	}
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (302), got %d", w.Code)
	}
	if createNewOwnerId != reqUserId {
		t.Fatalf("CreateNew called with unexpected owner id, got: %d",
			createNewOwnerId)
	}
	if createNewName != repo.DemoRepoName {
		t.Fatalf("CreateNew called with unexpected name, got: %s",
			createNewName)
	}
	if createNewDesc != repo.DemoRepoDescription {
		t.Fatalf("CreateNew called with unexpected description, got: %s",
			createNewDesc)
	}
}

func TestHandlePostSetUsername_UserAlreadyHasRepos(t *testing.T) {
	us := newFakeUserService()
	us.chooseUsernameAndStartTrial = func() (userservice.User, error) {
		return userservice.User{}, nil
	}

	// createNew is intentionally not mocked: calling it panics.
	rs := newFakeRepoService()
	rs.nonArchivedRepoCountIsGreaterThan = func(ownerId int64, n int) (bool, error) {
		return true, nil
	}
	h := handler{userS: us, repoS: rs}
	form := url.Values{}
	form.Set(routes.SetUsernameParamName, "marcos")
	req := httptest.NewRequest("POST", "/", nil)
	req.Form = form
	r := wrappers.UserMuxRequest{
		Request: req,
		User: userservice.User{
			Id:    reqUserId,
			State: user.UserState_NoUsername,
		},
	}
	w := httptest.NewRecorder()
	shouldCommit := h.handlePostSetUsername(w, r, nil)
	if !shouldCommit {
		t.Fatalf("expected commit = true")
	}
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (302), got %d", w.Code)
	}
}

func TestHandlePostSetUsername_InvalidUsername(t *testing.T) {
	// No method needs to be mocked because the username will be
	// read and considered invalid
	us := newFakeUserService()
	h := handler{userS: us}
	form := url.Values{}
	form.Set(routes.SetUsernameParamName, "INVALID USERNAME !!!")
	req := httptest.NewRequest("POST", "/", nil)
	req.Form = form
	r := wrappers.UserMuxRequest{
		Request: req,
		User: userservice.User{
			Id:    1,
			State: user.UserState_NoUsername,
		},
	}
	w := httptest.NewRecorder()

	shouldCommit := h.handlePostSetUsername(w, r, nil)
	if shouldCommit {
		t.Fatalf("expected commit = false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePostSetUsername_WrongState(t *testing.T) {
	// No method needs to be mocked because the state will be
	// read and considered invalid
	us := newFakeUserService()
	h := handler{userS: us}
	form := url.Values{}
	form.Set(routes.SetUsernameParamName, "marcos")
	req := httptest.NewRequest("POST", "/", nil)
	req.Form = form
	r := wrappers.UserMuxRequest{
		Request: req,
		User: userservice.User{
			Id:    1,
			State: user.UserState_NotSignedUp, // <-- not allowed
		},
	}
	w := httptest.NewRecorder()

	shouldCommit := h.handlePostSetUsername(w, r, nil)
	if shouldCommit {
		t.Fatalf("expected commit = false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlePostSetUsername_ServiceError(t *testing.T) {
	us := newFakeUserService()
	us.chooseUsernameAndStartTrial = func() (userservice.User, error) {
		return userservice.User{}, errors.New("THE DB BLEW UP")
	}
	h := handler{userS: us}
	form := url.Values{}
	form.Set(routes.SetUsernameParamName, "marcos")
	req := httptest.NewRequest("POST", "/", nil)
	req.Form = form
	r := wrappers.UserMuxRequest{
		Request: req,
		User: userservice.User{
			Id:    1,
			State: user.UserState_NoUsername,
		},
	}

	w := httptest.NewRecorder()
	shouldCommit := h.handlePostSetUsername(w, r, nil)
	if shouldCommit {
		t.Fatalf("expected commit = false")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// Standard constructor of the mocked service that causes nil-ptr panics
// for all methods that are not replaced with mocks. I.e. if a test calls
// some method that has not been set, the test will fail.
func newFakeUserService() *mockUserService {
	return &mockUserService{}
}

type mockUserService struct {
	chooseUsernameAndStartTrial func() (userservice.User, error)
	updateCliKey                func() error
	deleteCliKey                func() error
}

func (f *mockUserService) ChooseUsernameAndStartTrial(w context.Context, id int64, username string) (userservice.User, error) {
	return f.chooseUsernameAndStartTrial()
}
func (f *mockUserService) UpdateCliKey(w context.Context, userId int64, key string) error {
	return f.updateCliKey()
}
func (f *mockUserService) DeleteCliKey(w context.Context, userId int64) error {
	return f.deleteCliKey()
}

func newFakeRepoService() *mockRepoService {
	return &mockRepoService{}
}

type mockRepoService struct {
	createNew                         func(ownerId int64, name string, desc string) (r repo.Repo, isAlreadyExistsErr bool, err error)
	nonArchivedRepoCountIsGreaterThan func(ownerId int64, n int) (bool, error)
}

func (f *mockRepoService) CreateNew(_ context.Context, ownerId int64, name string, desc string) (repo.Repo, bool, error) {
	return f.createNew(ownerId, name, desc)
}
func (f *mockRepoService) NonArchivedRepoCountIsGreaterThan(_ context.Context, ownerId int64, n int) (bool, error) {
	return f.nonArchivedRepoCountIsGreaterThan(ownerId, n)
}
