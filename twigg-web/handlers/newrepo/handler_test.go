package newrepo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
)

func TestHandlePost_Success(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedRepoS.createNew = func() (r repo.Repo, isAlreadyExistsErr bool, err error) {
		return repo.Repo{}, false, nil
	}
	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	var ownerId int64 = 1
	var repoName string = "my-repo"
	var desc string = "desc"

	r := newRequestForHandlePost(repoName, desc, ownerId, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if !shouldCommit {
		t.Fatalf("expected shouldCommit=true")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// CreateNew
	if mockedRepoS.createNewLastCalledWithOwnerId != ownerId {
		t.Fatalf("createNew called with unexpected owner id, got: %d", mockedRepoS.createNewLastCalledWithOwnerId)
	}
	if mockedRepoS.createNewLastCalledWithName != repoName {
		t.Fatalf("createNew called with unexpected repo name, got: %s", mockedRepoS.createNewLastCalledWithName)
	}
	if mockedRepoS.createNewLastCalledWithDesc != desc {
		t.Fatalf("createNew called with unexpected desc, got: %s", mockedRepoS.createNewLastCalledWithDesc)
	}

	// CanCreateRepo
	if mockedCanCreateRepo.canCreateRepoCalledWithUserId != ownerId {
		t.Fatalf("CanCreateRepo called with unexpected owner id, got: %d", mockedRepoS.createNewLastCalledWithOwnerId)
	}
}
func TestHandlePost_UserCanNotCreateMoreRepos(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return false, nil
	}
	mockedRepoS.createNew = func() (r repo.Repo, isAlreadyExistsErr bool, err error) {
		t.Fatalf("should not get this far")
		return
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	var ownerId int64 = 1
	var repoName string = "my-repo"
	var desc string = "desc"

	r := newRequestForHandlePost(repoName, desc, ownerId, user.Subscription_Trial)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	if mockedCanCreateRepo.canCreateRepoCalledWithUserId != ownerId {
		t.Fatalf("CanCreateRepo called with unexpected owner id, got: %d", mockedCanCreateRepo.canCreateRepoCalledWithUserId)
	}
}

func TestHandlePost_RepoAlreadyExists(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (r repo.Repo, isAlreadyExistsErr bool, err error) {
		return repo.Repo{}, true, errors.New("already exists")
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	r := newRequestForHandlePost("repo", "desc", 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	if mockedRepoS.createNewLastCalledWithOwnerId != 1 {
		t.Fatalf("createNew called with unexpected owner id, got: %d", mockedRepoS.createNewLastCalledWithOwnerId)
	}
	if mockedRepoS.createNewLastCalledWithName != "repo" {
		t.Fatalf("createNew called with unexpected repo name, got: %s", mockedRepoS.createNewLastCalledWithName)
	}
	if mockedRepoS.createNewLastCalledWithDesc != "desc" {
		t.Fatalf("createNew called with unexpected desc, got: %s", mockedRepoS.createNewLastCalledWithDesc)
	}

	if mockedCanCreateRepo.canCreateRepoCalledWithUserId != 1 {
		t.Fatalf("CanCreateRepo called with unexpected owner id, got: %d", mockedCanCreateRepo.canCreateRepoCalledWithUserId)
	}
}

func TestHandlePost_CanCreateRepoFails(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return false, errors.New("db exploded")
	}
	mockedRepoS.createNew = func() (_ repo.Repo, _ bool, _ error) {
		t.Fatalf("should not get this far")
		return
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	r := newRequestForHandlePost("repo", "desc", 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	if mockedCanCreateRepo.canCreateRepoCalledWithUserId != 1 {
		t.Fatalf("CanCreateRepo called with unexpected owner id, got: %d", mockedCanCreateRepo.canCreateRepoCalledWithUserId)
	}
}

func TestHandlePost_CreateNewFails(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (r repo.Repo, isAlreadyExistsErr bool, err error) {
		return repo.Repo{}, false, errors.New("db exploded")
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	r := newRequestForHandlePost("repo", "desc", 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	if mockedRepoS.createNewLastCalledWithOwnerId != 1 {
		t.Fatalf("createNew called with unexpected owner id, got: %d", mockedRepoS.createNewLastCalledWithOwnerId)
	}
	if mockedRepoS.createNewLastCalledWithName != "repo" {
		t.Fatalf("createNew called with unexpected repo name, got: %s", mockedRepoS.createNewLastCalledWithName)
	}
	if mockedRepoS.createNewLastCalledWithDesc != "desc" {
		t.Fatalf("createNew called with unexpected desc, got: %s", mockedRepoS.createNewLastCalledWithDesc)
	}
	if mockedCanCreateRepo.canCreateRepoCalledWithUserId != 1 {
		t.Fatalf("CanCreateRepo called with unexpected owner id, got: %d", mockedCanCreateRepo.canCreateRepoCalledWithUserId)
	}
}

func TestHandlePost_InvalidRepoName(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (repo.Repo, bool, error) {
		t.Fatalf("should not get this far")
		return repo.Repo{}, false, nil
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	r := newRequestForHandlePost("########", "desc", 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandlePost_InvalidRepoName_Empty(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (repo.Repo, bool, error) {
		t.Fatalf("should not get this far")
		return repo.Repo{}, false, nil
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	r := newRequestForHandlePost("", "desc", 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandlePost_InvalidRepoName_TooLong(t *testing.T) {
	// WARNING: Assumes max repo name len = 64
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (repo.Repo, bool, error) {
		t.Fatalf("should not get this far")
		return repo.Repo{}, false, nil
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	longName := strings.Repeat("a", 65)

	r := newRequestForHandlePost(longName, "desc", 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandlePost_DescriptionTooLong(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (repo.Repo, bool, error) {
		t.Fatalf("should not get this far")
		return repo.Repo{}, false, nil
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, newMockedPermissionsService())

	longDesc := strings.Repeat("a", 101)

	r := newRequestForHandlePost("repo", longDesc, 1, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandlePost_OrgRepo_GrantsMembersAndOwnersWritePermission(t *testing.T) {
	mockedRepoS := newMockedRepoService()
	mockedCanCreateRepo := newMockedCanCreateRepo()
	mockedPermSrv := newMockedPermissionsService()

	var orgId int64 = 99
	var repoId uint64 = 77
	var memberId int64 = 10
	var ownerId2 int64 = 11

	mockedCanCreateRepo.canCreateRepo = func() (bool, error) {
		return true, nil
	}
	mockedRepoS.createNew = func() (r repo.Repo, isAlreadyExistsErr bool, err error) {
		return repo.Repo{Id: repoId}, false, nil
	}
	mockedPermSrv.getUsersWithPermission = func(assetId string, p permissions.Permission) (iterator.I[int64], error) {
		switch p {
		case permissions.Permission_OrganizationOwner:
			return iterator.NewIterFromSlice([]int64{ownerId2}), nil
		case permissions.Permission_OrganizationMember:
			return iterator.NewIterFromSlice([]int64{memberId}), nil
		}
		t.Fatalf("unexpected permission: %v", p)
		return nil, nil
	}
	grantedPerms := map[int64]bool{}
	mockedPermSrv.grantPermissionIfNotExists = func(userId int64, p permissions.Permission, assetId string) (bool, error) {
		if p != permissions.Permission_WriteRepo {
			t.Fatalf("expected WriteRepo permission, got %v", p)
		}
		if assetId != permissions.RepoAssetId(repoId) {
			t.Fatalf("expected repoAssetId=%s, got %s", permissions.RepoAssetId(repoId), assetId)
		}
		grantedPerms[userId] = true
		return false, nil
	}

	h := newHandler(mockedCanCreateRepo, mockedRepoS, mockedPermSrv)
	r := newRequestForHandlePostForOrg("my-repo", "desc", 1, orgId, user.Subscription_Team)
	w := httptest.NewRecorder()

	shouldCommit := h.handlePost(w, r, nil)

	if !shouldCommit {
		t.Fatalf("expected shouldCommit=true")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if mockedRepoS.createNewLastCalledWithOwnerId != orgId {
		t.Fatalf("CreateNew called with wrong ownerId: got %d, want %d", mockedRepoS.createNewLastCalledWithOwnerId, orgId)
	}
	if !grantedPerms[memberId] {
		t.Fatalf("expected member %d to receive WriteRepo permission", memberId)
	}
	if !grantedPerms[ownerId2] {
		t.Fatalf("expected owner %d to receive WriteRepo permission", ownerId2)
	}
}

// Standard constructor of the mocked service that causes nil-ptr panics
// for all methods that are not replaced with mocks. I.e. if a test calls
// some method that has not been set, the test will fail.
func newMockedRepoService() *mockedRepoService {
	return &mockedRepoService{}
}

type mockedRepoService struct {
	createNew                      func() (r repo.Repo, isAlreadyExistsErr bool, err error)
	createNewLastCalledWithOwnerId int64
	createNewLastCalledWithName    string
	createNewLastCalledWithDesc    string
}

func (f *mockedRepoService) CreateNew(_ context.Context, ownerId int64, name string, desc string) (repo.Repo, bool, error) {
	f.createNewLastCalledWithOwnerId = ownerId
	f.createNewLastCalledWithName = name
	f.createNewLastCalledWithDesc = desc
	return f.createNew()
}

// Standard constructor of the mocked service that causes nil-ptr panics
// for all methods that are not replaced with mocks. I.e. if a test calls
// some method that has not been set, the test will fail.
func newMockedCanCreateRepo() *mockedCanCreateRepo {
	return &mockedCanCreateRepo{}
}

type mockedCanCreateRepo struct {
	canCreateRepo                 func() (bool, error)
	canCreateRepoCalledWithUserId int64
}

func (m *mockedCanCreateRepo) CanCreateRepo(u user.User, _ context.Context) (bool, error) {
	m.canCreateRepoCalledWithUserId = u.Id
	return m.canCreateRepo()
}

func newRequestForHandlePost(
	repoName,
	desc string,
	userId int64,
	userSubscriptionPlan user.SubscriptionPlan,
) wrappers.UserWithSubMuxRequest {
	form := url.Values{}
	form.Set(routes.NewRepoNameParameterName, repoName)
	form.Set(routes.NewRepoDescriptionParameterName, desc)
	req := httptest.NewRequest(
		"POST",
		routes.NewRepoUrl,
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r := wrappers.UserWithSubMuxRequest{
		Request: req,
		UserWithSub: user.User{
			Id:                   userId,
			SelfPaidSubscription: userSubscriptionPlan,
		},
	}
	return r
}

func newRequestForHandlePostForOrg(
	repoName, desc string,
	userId, orgId int64,
	orgSubscriptionPlan user.SubscriptionPlan,
) wrappers.UserWithSubMuxRequest {
	form := url.Values{}
	form.Set(routes.NewRepoNameParameterName, repoName)
	form.Set(routes.NewRepoDescriptionParameterName, desc)
	req := httptest.NewRequest(
		"POST",
		routes.NewRepoUrl,
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return wrappers.UserWithSubMuxRequest{
		Request:               req,
		UserWithSub:           user.User{Id: userId, SelfPaidSubscription: user.Subscription_Team},
		HaveOrgParamInRequest: true,
		OrgWithSub: user.User{
			Id:                   orgId,
			SelfPaidSubscription: orgSubscriptionPlan,
		},
	}
}

func newMockedPermissionsService() *mockedPermissionsService {
	return &mockedPermissionsService{}
}

type mockedPermissionsService struct {
	getUsersWithPermission         func(assetId string, p permissions.Permission) (iterator.I[int64], error)
	grantPermissionIfNotExists     func(userId int64, p permissions.Permission, assetId string) (bool, error)
	grantPermLastCalledWithUserId  int64
	grantPermLastCalledWithPerm    permissions.Permission
	grantPermLastCalledWithAssetId string
}

func (m *mockedPermissionsService) GetUsersWithPermission(_ context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error) {
	return m.getUsersWithPermission(assetId, p)
}

func (m *mockedPermissionsService) GrantPermissionIfNotExists(_ context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	m.grantPermLastCalledWithUserId = userId
	m.grantPermLastCalledWithPerm = p
	m.grantPermLastCalledWithAssetId = assetId
	return m.grantPermissionIfNotExists(userId, p, assetId)
}