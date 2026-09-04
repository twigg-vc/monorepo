package organization

import (
	"context"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestHandleGetCreateOrganization(t *testing.T) {
	tests := []struct {
		name                  string
		featureEnabled        bool
		expectedStatus        int
		expectedBodyToContain string
	}{
		{
			name:                  "feature disabled",
			featureEnabled:        false,
			expectedStatus:        http.StatusServiceUnavailable,
			expectedBodyToContain: "feature is not enabled",
		},
		{
			name:                  "renders component",
			featureEnabled:        true,
			expectedStatus:        http.StatusOK,
			expectedBodyToContain: "<new-organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler{}
			rr := httptest.NewRecorder()
			r := wrappers.UserWithSubMuxRequest{
				Flags: featureflags.Flags{
					OrganizationFeatureIsEnabled: tt.featureEnabled,
				},
			}

			h.handleGetCreateOrganization(rr, r, nil)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			body := rr.Body.String()
			if !strings.Contains(body, tt.expectedBodyToContain) {
				t.Fatalf("expected body to contain %q, got %q", tt.expectedBodyToContain, body)
			}
		})
	}
}

func TestHandlePostCreateOrganization_FeatureDisabled(t *testing.T) {
	mockedUserSrv := mockUserService{
		createNewOrganizationUser: func(_ string) (u user.User, err error) {
			t.Fatalf("should not get this far")
			return
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(_ int64, _ permissions.Permission, _ string) (alreadyExists bool, err error) {
			t.Fatalf("should not get this far")
			return
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	organizationFeatureIsEnabled := false
	req := createHandlePostCreateOrganizationReq("my-org", 123, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostCreateOrganization(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "feature is not enabled" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostCreateOrganization_InvalidName(t *testing.T) {
	mockedUserSrv := mockUserService{
		createNewOrganizationUser: func(_ string) (u user.User, err error) {
			t.Fatalf("should not get this far")
			return
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(_ int64, _ permissions.Permission, _ string) (alreadyExists bool, err error) {
			t.Fatalf("should not get this far")
			return
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	var orgName string = "1"
	var userWithSubId int64 = 123
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostCreateOrganizationReq(orgName, userWithSubId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostCreateOrganization(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "invalid new org name" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostCreateOrganization_MaxNumberOfOrgsUserCanBeOwner(t *testing.T) {
	var userWithSubId int64 = 123
	var organizationFeatureIsEnabled bool = true

	mockedPermSrv := mockPermissionsService{
		countUserAssetsWithPermission: func(userId int64, p permissions.Permission) (int64, error) {
			if userId != 123 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			return orgCreationOwnerLimit, nil
		},
	}
	h := handler{userS: mockUserService{}, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := createHandlePostCreateOrganizationReq("my-org", userWithSubId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostCreateOrganization(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "user already owns too many organizations" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostCreateOrganization_CreateOrganizationError(t *testing.T) {
	mockedUserSrv := mockUserService{
		createNewOrganizationUser: func(_ string) (user.User, error) {
			return user.User{}, errors.New("boom")
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(_ int64, _ permissions.Permission, _ string) (alreadyExists bool, err error) {
			t.Fatalf("should not get this far")
			return
		},
		countUserAssetsWithPermission: func(userId int64, permission permissions.Permission) (int64, error) {
			return 0, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	var orgName string = "my-org"
	var userWithSubId int64 = 123
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostCreateOrganizationReq(orgName, userWithSubId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostCreateOrganization(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "internal error creating org" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostCreateOrganization_GrantPermissionError(t *testing.T) {
	mockedUserSrv := mockUserService{
		createNewOrganizationUser: func(orgUsername string) (user.User, error) {
			if orgUsername != "my-org" {
				t.Fatalf("unexpected orgUsername: %q", orgUsername)
			}
			return user.User{Id: 456}, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error) {
			if userId != 123 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if assetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return false, errors.New("boom")
		},
		countUserAssetsWithPermission: func(userId int64, permission permissions.Permission) (int64, error) {
			return 0, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	var orgName string = "my-org"
	var userWithSubId int64 = 123
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostCreateOrganizationReq(orgName, userWithSubId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostCreateOrganization(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "internal error creating org" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostCreateOrganization_Success(t *testing.T) {
	calledCreate := false
	calledGrant := false

	mockedUserSrv := mockUserService{
		createNewOrganizationUser: func(orgUsername string) (user.User, error) {
			calledCreate = true

			if orgUsername != "my-org" {
				t.Fatalf("unexpected orgUsername: %q", orgUsername)
			}

			return user.User{Id: 456}, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error) {
			calledGrant = true

			if userId != 123 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if assetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return false, nil
		},
		countUserAssetsWithPermission: func(userId int64, permission permissions.Permission) (int64, error) {
			return 0, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	var orgName string = "my-org"
	var userWithSubId int64 = 123
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostCreateOrganizationReq(orgName, userWithSubId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostCreateOrganization(rr, req, nil)

	if !shouldCommit {
		t.Fatal("expected shouldCommit to be true")
	}
	if !calledCreate {
		t.Fatal("expected CreateNewOrganizationUser to be called")
	}
	if !calledGrant {
		t.Fatal("expected GrantPermissionIfNotExists to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_FeatureDisabled(t *testing.T) {
	h := handler{userS: mockUserService{}, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = false
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "feature is not enabled" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_EmptyUsername(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(_ string) (u user.User, isNotFoundErr bool, err error) {
			t.Fatalf("should not get this far")
			return
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = ""
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "empty username param" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_UserNotFound(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{}, true, fmt.Errorf("user not found")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "User to grant permission not found" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_GetByUsernameError(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{}, false, errors.New("boom")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Internal err getting usr" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_GrantToOrganizationErr(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999, IsOrganization: true}, false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Can not grant permission to a organization user" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_InvalidPermissionParamFormValue(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			t.Fatalf("should not get this far")
			return false, nil
		},
	}

	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()

	var usernameParam string = "john"
	var permissionParam string = "not-a-number"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true

	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "invalid permission param form value" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_InvalidPermission(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		grantPermissionIfNotExists: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			t.Fatalf("should not get this far")
			return false, nil
		},
	}

	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()

	var usernameParam string = "john"
	var permissionParam string = "1"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true

	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "invalid permission" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_HasPermissionCheckError(t *testing.T) {
	var mockUserToGrantPermission = user.User{Id: 999, Username: "john"}
	var permissionParam = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId = int64(456)
	var organizationFeatureIsEnabled = true

	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != mockUserToGrantPermission.Username {
				t.Fatalf("unexpected username: %q", username)
			}
			return mockUserToGrantPermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			return false, errors.New("boom")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(mockUserToGrantPermission.Username, permissionParam, userWithOwnerPermissionId, orgId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Failed to check existing permissions" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_CannotGrantOwnerToMember(t *testing.T) {
	var mockUserToGrantPermission = user.User{Id: 999, Username: "john"}
	var permissionParam = "3" // Permission_OrganizationOwner
	var userWithOwnerPermissionId int64 = 123
	var orgId = int64(456)
	var organizationFeatureIsEnabled = true

	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != mockUserToGrantPermission.Username {
				t.Fatalf("unexpected username: %q", username)
			}
			return mockUserToGrantPermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			if userId != mockUserToGrantPermission.Id {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationMember {
				t.Fatalf("expected conflicting perm check for Member, got %v", p)
			}
			if assetId != permissions.OrganizationAssetId(orgId) {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return true, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		mockUserToGrantPermission.Username,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "user already has conflicting organization role" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_CannotGrantMemberToOwner(t *testing.T) {
	var mockUserToGrantPermission = user.User{Id: 999, Username: "john"}
	var permissionParam = "4" // Permission_OrganizationMember
	var userWithOwnerPermissionId int64 = 123
	var orgId = int64(456)
	var organizationFeatureIsEnabled = true

	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != mockUserToGrantPermission.Username {
				t.Fatalf("unexpected username: %q", username)
			}
			return mockUserToGrantPermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			if userId != mockUserToGrantPermission.Id {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("expected conflicting perm check for Owner, got %v", p)
			}
			if assetId != permissions.OrganizationAssetId(orgId) {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return true, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(mockUserToGrantPermission.Username, permissionParam, userWithOwnerPermissionId, orgId, organizationFeatureIsEnabled)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "user already has conflicting organization role" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_NoAvailableSeats(t *testing.T) {
	if NoSeatsLeftErrMsg != "organization has no available seats" {
		t.Fatalf("NoSeatsLeftErrMsg changed. Did you remember to change the frontend?")
	}

	var mockUserToGrantPermission = user.User{Id: 999, Username: "john"}
	var orgId = int64(456)

	mockedUserSrv := mockUserService{
		getByUsername: func(_ string) (u user.User, isNotFoundErr bool, err error) {
			return mockUserToGrantPermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			return false, nil
		},
		countUsersWithPermission: func(_ string, _ permissions.Permission) (int64, error) {
			return 5, nil
		},
	}
	mockedOrgHelper := mockOrgHelper{
		orgCanAddOwnerOrMember: func(_ user.User, _ int64, _ int64) (bool, error) {
			return false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	req := createHandlePostGrantOwnerOrMemberPermToUserReq("john", "4", 123, orgId, true)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != NoSeatsLeftErrMsg {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_GrantPermissionError(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			return false, nil
		},
		countUsersWithPermission: func(_ string, _ permissions.Permission) (int64, error) {
			return 1, nil
		},
	}
	mockedOrgHelper := mockOrgHelper{
		orgCanAddOwnerOrMember: func(_ user.User, _ int64, _ int64) (bool, error) {
			return true, nil
		},
		grantUserPermissionToOrgIfNotExist: func(userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error) {
			if userId != 999 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationMember {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if orgAssetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", orgAssetId)
			}
			return false, errors.New("boom")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "4"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Failed to grant permission" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_PermissionAlreadyExists(t *testing.T) {
	if PermissionAlreadyExitsErrMsg != "permission already exists" {
		t.Fatalf("PermissionAlreadyExitsErrMsg changed. Did you remember to change the frontend code?")
	}
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			return false, nil
		},
		countUsersWithPermission: func(_ string, _ permissions.Permission) (int64, error) {
			return 0, nil
		},
	}
	mockedOrgHelper := mockOrgHelper{
		orgCanAddOwnerOrMember: func(_ user.User, _ int64, _ int64) (bool, error) {
			return true, nil
		},
		grantUserPermissionToOrgIfNotExist: func(userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error) {
			if userId != 999 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationMember {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if orgAssetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", orgAssetId)
			}
			return true, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "4"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit to be false")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != PermissionAlreadyExitsErrMsg {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostGrantOwnerOrMemberPermToUser_Success(t *testing.T) {
	var mockUserToGrantPermission = user.User{Id: 999, Username: "john"}
	var permissionParam = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId = int64(456)
	var expectedNumOwners int64 = 1
	var expectedNumMembers int64 = 2
	var grantCalled bool

	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (u user.User, isNotFoundErr bool, err error) {
			if username != mockUserToGrantPermission.Username {
				t.Fatalf("unexpected username: %q", username)
			}
			return mockUserToGrantPermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(_ int64, _ permissions.Permission, _ string) (bool, error) {
			return false, nil
		},
		countUsersWithPermission: func(assetId string, p permissions.Permission) (int64, error) {
			if assetId != permissions.OrganizationAssetId(orgId) {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			switch p {
			case permissions.Permission_OrganizationOwner:
				return expectedNumOwners, nil
			case permissions.Permission_OrganizationMember:
				return expectedNumMembers, nil
			default:
				return 0, errors.New("unexpected permission")
			}

		},
	}
	mockedOrgHelper := mockOrgHelper{
		orgCanAddOwnerOrMember: func(orgU user.User, numOwners int64, numMembers int64) (bool, error) {
			if orgU.Id != orgId {
				t.Fatalf("unexpected orgU.Id: %d", orgU.Id)
			}
			if numOwners != expectedNumOwners {
				t.Fatalf("unexpected numOwners: %d", numOwners)
			}
			if numMembers != expectedNumMembers {
				t.Fatalf("unexpected numMembers: %d", numMembers)
			}
			return true, nil
		},
		grantUserPermissionToOrgIfNotExist: func(userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error) {
			if userId != mockUserToGrantPermission.Id {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			if orgAssetId != permissions.OrganizationAssetId(orgId) {
				t.Fatalf("unexpected assetId: %q", orgAssetId)
			}
			grantCalled = true
			return false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostGrantOwnerOrMemberPermToUserReq(
		mockUserToGrantPermission.Username,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)
	shouldCommit := h.handlePostGrantOwnerOrMemberPermToUser(rr, req, nil)

	if !shouldCommit {
		t.Fatal("expected shouldCommit to be true")
	}
	if !grantCalled {
		t.Fatal("expected GrantUserPermissionToOrgIfNotExist to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "ok" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_InvalidUsernameParam(t *testing.T) {
	h := handler{userS: mockUserService{}, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = ""
	var permissionParam string = "4"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "empty username param" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_FeatureDisabled(t *testing.T) {
	h := handler{userS: mockUserService{}, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = false
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "feature is not enabled" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
func TestHandlePostRevokeOwnerOrMemberPermToUser_InvalidPermission(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "1"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "invalid permission to revoke" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_GetByUsernameError(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			return user.User{}, false, errors.New("boom")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Internal err getting usr" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
func TestHandlePostRevokeOwnerOrMemberPermToUser_RevokeFromOrgUserErr(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999, IsOrganization: true}, false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockPermissionsService{}}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Can not revoke permission from organization user" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_DoesNotHavePermission(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			if userId != 999 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationMember {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if assetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "4"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Does not have permission to revoke" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_LastOwner(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			if userId != 999 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if assetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return true, nil
		},
	}
	mockedOrgHelper := mockOrgHelper{
		revokeUserPermissionFromOrgIfExist: func(userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error) {
			return true, errors.New("cant remove the only owner")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "organization needs a least one owner" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_HasPermissionError(t *testing.T) {
	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != "john" {
				t.Fatalf("unexpected username: %q", username)
			}
			return user.User{Id: 999}, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			if userId != 999 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationMember {
				t.Fatalf("unexpected permission: %v", p)
			}
			expectedAssetId := permissions.OrganizationAssetId(456)
			if assetId != expectedAssetId {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return false, errors.New("boom")
		},
		revokePermissionIfExists: func(userId int64, p permissions.Permission, assetId string) error {
			t.Fatalf("should not revoke permission")
			return nil
		},
	}

	h := handler{
		userS:   mockedUserSrv,
		permSrv: mockedPermSrv,
	}

	rr := httptest.NewRecorder()
	var usernameParam string = "john"
	var permissionParam string = "4"
	var userWithOwnerPermissionId int64 = 123
	var orgId int64 = 456
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		usernameParam,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Failed to check permission" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_RevokeError(t *testing.T) {
	var mockUserToRevokePermission = user.User{Id: 999, Username: "john"}
	var permissionParam = "4"
	var userWithOwnerPermissionId = int64(123)
	var orgId = int64(456)
	var organizationFeatureIsEnabled = true

	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != mockUserToRevokePermission.Username {
				t.Fatalf("unexpected username: %q", username)
			}
			return mockUserToRevokePermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			return true, nil
		},
	}
	mockedOrgHelper := mockOrgHelper{
		revokeUserPermissionFromOrgIfExist: func(userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error) {
			return false, errors.New("boom")
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		mockUserToRevokePermission.Username,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if shouldCommit {
		t.Fatal("expected shouldCommit false")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "Failed to revoke permission" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandlePostRevokeOwnerOrMemberPermToUser_Success(t *testing.T) {
	var mockUserToRevokePermission = user.User{Id: 999, Username: "john"}
	var permissionParam = "3"
	var userWithOwnerPermissionId int64 = 123
	var orgId = int64(456)
	var revokeCalled bool

	mockedUserSrv := mockUserService{
		getByUsername: func(username string) (user.User, bool, error) {
			if username != mockUserToRevokePermission.Username {
				t.Fatalf("unexpected username: %q", username)
			}
			return mockUserToRevokePermission, false, nil
		},
	}
	mockedPermSrv := mockPermissionsService{
		hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
			if userId != mockUserToRevokePermission.Id {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			if assetId != permissions.OrganizationAssetId(orgId) {
				t.Fatalf("unexpected assetId: %q", assetId)
			}
			return true, nil
		},
	}
	mockedOrgHelper := mockOrgHelper{
		revokeUserPermissionFromOrgIfExist: func(userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error) {
			if userId != mockUserToRevokePermission.Id {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if p != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p)
			}
			if orgAssetId != permissions.OrganizationAssetId(orgId) {
				t.Fatalf("unexpected assetId: %q", orgAssetId)
			}
			revokeCalled = true
			return false, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, orgHelper: mockedOrgHelper}

	rr := httptest.NewRecorder()
	var organizationFeatureIsEnabled bool = true
	req := createHandlePostRevokeOwnerOrMemberPermToUserReq(
		mockUserToRevokePermission.Username,
		permissionParam,
		userWithOwnerPermissionId,
		orgId,
		organizationFeatureIsEnabled,
	)

	shouldCommit := h.handlePostRevokeOwnerOrMemberPermToUser(rr, req, nil)

	if !shouldCommit {
		t.Fatal("expected shouldCommit true")
	}
	if !revokeCalled {
		t.Fatal("expected RevokeUserPermissionFromOrgIfExist to be called")
	}
	if strings.TrimSpace(rr.Body.String()) != "ok" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleManageOrgSubscription_GetNewCustomerPortalSessionError(t *testing.T) {
	mockedStripeClient := mockStripeClient{
		getNewCustomerPortalSession: func(orgId int64, stripeId string) (string, error) {
			if orgId != 456 {
				t.Fatalf("unexpected orgId: %d", orgId)
			}

			if stripeId != "stripe-org-id" {
				t.Fatalf("unexpected stripeId: %q", stripeId)
			}

			return "", errors.New("boom")
		},
	}
	h := handler{stripeClient: mockedStripeClient}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	orgOwnerReq := wrappers.OrgOwnerMuxRequest{
		Request: req,
		Org: user.User{
			Id:                   456,
			StripeId:             "stripe-org-id",
			StripeSubscriptionID: "stripe-sub-id",
			IsOrganization:       true,
		},
	}

	h.handleManageOrgSubscription(rr, orgOwnerReq, nil)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "something is wrong" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleManageOrgSubscription_Success(t *testing.T) {
	mockedStripeClient := mockStripeClient{
		getNewCustomerPortalSession: func(orgId int64, stripeId string) (string, error) {
			if orgId != 456 {
				t.Fatalf("unexpected orgId: %d", orgId)
			}
			if stripeId != "stripe-org-id" {
				t.Fatalf("unexpected stripeId: %q", stripeId)
			}
			return "https://stripe.com/session", nil
		},
	}
	h := handler{stripeClient: mockedStripeClient}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	orgOwnerReq := wrappers.OrgOwnerMuxRequest{
		Request: req,
		Org: user.User{
			Id:                   456,
			StripeId:             "stripe-org-id",
			StripeSubscriptionID: "stripe-sub-id",
			IsOrganization:       true,
		},
	}

	h.handleManageOrgSubscription(rr, orgOwnerReq, nil)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}
	location := rr.Header().Get("Location")
	if location != "https://stripe.com/session" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestHandleGetUserOrganizations_GetUserAssetIdsWithPermissionErr(t *testing.T) {
	mockedPermSrv := mockPermissionsService{
		getUserAssetIdsWithPermission: func(userId int64, p ...permissions.Permission) (iterator.I[string], error) {
			if userId != 123 {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if len(p) != 2 {
				t.Fatalf("unexpected permissions length: %d", len(p))
			}
			if p[0] != permissions.Permission_OrganizationOwner {
				t.Fatalf("unexpected permission: %v", p[0])
			}
			if p[1] != permissions.Permission_OrganizationMember {
				t.Fatalf("unexpected permission: %v", p[1])
			}
			return nil, errors.New("boom")
		},
	}
	h := handler{permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := wrappers.UserWithSubMuxRequest{
		UserWithSub: user.User{
			Id: 123,
		},
	}

	h.handleGetUserOrganizations(rr, req, nil)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "failed to get organizations" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleGetUserOrganizations_GetOrgErr(t *testing.T) {
	mockedUserSrv := mockUserService{
		get: func(id int64) (u user.User, isNotFoundErr bool, err error) {
			if id != 456 {
				t.Fatalf("unexpected id: %d", id)
			}
			return user.User{}, false, errors.New("boom")
		},
	}
	mockedPermSrv := mockPermissionsService{
		getUserAssetIdsWithPermission: func(userId int64, p ...permissions.Permission) (iterator.I[string], error) {
			return &fakeStringIterator{
				values: []string{
					permissions.OrganizationAssetId(456),
				},
			}, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := wrappers.UserWithSubMuxRequest{
		UserWithSub: user.User{
			Id: 123,
		},
	}

	h.handleGetUserOrganizations(rr, req, nil)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "failed to get organization" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleGetUserOrganizations_Success(t *testing.T) {
	calledGetUserAssetIds := false

	user1 := user.User{Id: 123, Username: "user-123"}
	org1 := user.User{Id: 111, Username: "acme", IsOrganization: true}
	org2 := user.User{Id: 222, Username: "fert-tech", IsOrganization: true}
	mockedUserSrv := mockUserService{
		get: func(id int64) (u user.User, isNotFoundErr bool, err error) {
			switch id {
			case org1.Id:
				return org1, false, nil
			case org2.Id:
				return org2, false, nil
			default:
				t.Fatalf("unexpected id: %d", id)
				return
			}
		},
	}
	mockedPermSrv := mockPermissionsService{
		getUserAssetIdsWithPermission: func(userId int64, p ...permissions.Permission) (iterator.I[string], error) {
			calledGetUserAssetIds = true

			if userId != user1.Id {
				t.Fatalf("unexpected userId: %d", userId)
			}
			if len(p) != 2 {
				t.Fatalf("unexpected permissions length: %d", len(p))
			}
			return &fakeStringIterator{
				values: []string{
					permissions.OrganizationAssetId(org1.Id),
					permissions.OrganizationAssetId(org2.Id),
				},
			}, nil
		},
	}
	h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv}

	rr := httptest.NewRecorder()
	req := wrappers.UserWithSubMuxRequest{
		UserWithSub: user.User{
			Id: user1.Id,
		},
	}

	h.handleGetUserOrganizations(rr, req, nil)

	if !calledGetUserAssetIds {
		t.Fatal("expected GetUserAssetIdsWithPermission to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	bodyStr := rr.Body.String()
	if !strings.Contains(bodyStr, org1.Username) {
		t.Fatalf("body does not contain org1.Username(%s), got: %q", org1.Username, bodyStr)
	}
	if !strings.Contains(bodyStr, org2.Username) {
		t.Fatalf("body does not contain org2.Username(%s), got: %q", org1.Username, bodyStr)
	}
}

func TestHandleGetOrganization(t *testing.T) {
	testCases := []struct {
		desc                          string
		featureIsDisabled             bool
		mockGetLimitsErr              bool
		mockGetUsersWithPermissionErr bool
		mockHasPermissionErr          bool
		mockGetUserErr                bool
		mockGetOrgReposErr            bool
		expectedStatus                int
	}{
		{
			desc:              "featureIsDisabled",
			featureIsDisabled: true,
			expectedStatus:    http.StatusServiceUnavailable,
		},
		{
			desc:             "mockGetLimitsErr",
			mockGetLimitsErr: true,
			expectedStatus:   http.StatusInternalServerError,
		},
		{
			desc:                          "mockGetUsersWithPermissionErr",
			mockGetUsersWithPermissionErr: true,
			expectedStatus:                http.StatusInternalServerError,
		},
		{
			desc:                 "mockHasPermissionErr",
			mockHasPermissionErr: true,
			expectedStatus:       http.StatusInternalServerError,
		},
		{
			desc:           "mockGetUserErr",
			mockGetUserErr: true,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			desc:               "mockGetOrgReposErr",
			mockGetOrgReposErr: true,
			expectedStatus:     http.StatusInternalServerError,
		},
		{
			desc:           "no errors",
			expectedStatus: http.StatusOK,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			mockOrgId := int64(456)
			mockedTrackQueue := mockTrackQueue{
				getLimits: func(userId int64) (maxJobs int, maxTimeout time.Duration, err error) {
					if userId != mockOrgId {
						t.Fatalf("unexpected userId: %d", userId)
						return
					}
					if tC.mockGetLimitsErr {
						err = errors.New("boom")
						return
					}
					return 10, time.Duration(100 * time.Millisecond), nil
				},
			}

			mockUserWithOwnerOrMemberPermissionId := int64(111)
			mockUsersIdWithOwnerPermission := []int64{111, 222, 333}
			mockUsersIdWithMemberPermission := []int64{444, 555, 666}

			mockedPermSrv := mockPermissionsService{
				getUsersWithPermission: func(assetId string, p permissions.Permission) (it iterator.I[int64], err error) {
					if assetId != permissions.OrganizationAssetId(mockOrgId) {
						t.Fatalf("unexpected assetId: %s", assetId)
						return
					}
					if tC.mockGetUsersWithPermissionErr {
						err = errors.New("boom")
						return
					}
					if p == permissions.Permission_OrganizationOwner {
						return iterator.NewIterFromSlice(mockUsersIdWithOwnerPermission), nil
					}
					if p == permissions.Permission_OrganizationMember {
						return iterator.NewIterFromSlice(mockUsersIdWithMemberPermission), nil
					}
					t.Fatalf("unexpected permission: %d", p)
					return
				},
				hasPermission: func(userId int64, p permissions.Permission, assetId string) (hasPerm bool, err error) {
					if userId != mockUserWithOwnerOrMemberPermissionId {
						t.Fatalf("unexpected userId: %d", userId)
						return
					}
					if p != permissions.Permission_OrganizationOwner {
						t.Fatalf("unexpected permission: %d", p)
						return
					}
					if assetId != permissions.OrganizationAssetId(mockOrgId) {
						t.Fatalf("unexpected assetId: %s", assetId)
						return
					}
					if tC.mockHasPermissionErr {
						err = errors.New("boom")
						return
					}
					return true, nil
				},
			}

			mockedUserSrv := mockUserService{
				get: func(id int64) (u user.User, isNotFoundErr bool, err error) {
					if tC.mockGetUserErr {
						err = errors.New("boom")
						return
					}
					if slices.Contains(mockUsersIdWithOwnerPermission, id) {
						return user.User{Id: id, Username: fmt.Sprintf("owner-username-%d", id)}, false, nil
					}
					if slices.Contains(mockUsersIdWithMemberPermission, id) {
						return user.User{Id: id, Username: fmt.Sprintf("member-username-%d", id)}, false, nil
					}
					t.Fatalf("unexpected id: %d", id)
					return
				},
			}
			mockedRepoSrv := mockRepoService{
				getAllByOwnerId: func(ownerId int64) (iterator.I[repo.Repo], error) {
					if ownerId != mockOrgId {
						t.Fatalf("unexpected ownerId: %d", ownerId)
					}
					if tC.mockGetOrgReposErr {
						return nil, errors.New("boom")
					}
					return iterator.NewIterFromSlice([]repo.Repo{}), nil
				},
			}
			h := handler{userS: mockedUserSrv, permSrv: mockedPermSrv, trackQueue: mockedTrackQueue, repoService: mockedRepoSrv}

			req := createHandleGetOrganizationPageReq(
				mockUserWithOwnerOrMemberPermissionId,
				mockOrgId,
				!tC.featureIsDisabled,
			)
			rr := httptest.NewRecorder()
			h.handleGetOrganizationPage(rr, req, context.Background())

			if rr.Code != tC.expectedStatus {
				t.Fatalf("expected status %d, got %d", tC.expectedStatus, rr.Code)
			}

			if rr.Code == http.StatusOK {
				bodyStr := rr.Body.String()
				if !strings.Contains(bodyStr, "<organization-page") {
					t.Error("Response body missing <organization-page> tag")
				}
				expectedOrgMaxTrackJobs := `OrgMaxTrackJobs="10"`
				if !strings.Contains(bodyStr, expectedOrgMaxTrackJobs) {
					t.Errorf("Expected attribute %s not found in body", expectedOrgMaxTrackJobs)
				}
				expectedOrgMaxTrackMilliseconds := `OrgMaxTrackMilliseconds="100"`
				if !strings.Contains(bodyStr, expectedOrgMaxTrackMilliseconds) {
					t.Errorf("Expected attribute %s not found in body", expectedOrgMaxTrackMilliseconds)
				}
				expectedCurrentUserIsOrgOwner := `CurrentUserIsOrgOwner`
				if !strings.Contains(bodyStr, expectedCurrentUserIsOrgOwner) {
					t.Errorf("Expected attribute %s not found in body", expectedCurrentUserIsOrgOwner)
				}

				// This is hacky but it works
				// Owners
				ownersToTest := []string{"owner-username-111", "owner-username-222", "owner-username-333"}
				for _, name := range ownersToTest {
					// We check for the name surrounded by the escaped quotes
					expectedSnippet := fmt.Sprintf("&#34;Username&#34;:&#34;%s&#34;", name)
					if !strings.Contains(bodyStr, expectedSnippet) {
						t.Errorf("Could not find owner %s in response body", name)
					}
				}
				// Members
				membersToTest := []string{"member-username-444", "member-username-555", "member-username-666"}
				for _, name := range membersToTest {
					expectedSnippet := fmt.Sprintf("&#34;Username&#34;:&#34;%s&#34;", name)
					if !strings.Contains(bodyStr, expectedSnippet) {
						t.Errorf("Could not find member %s in response body", name)
					}
				}
			}
		})
	}
}

type mockUserService struct {
	createNewOrganizationUser func(organizationUsername string) (user.User, error)
	getByUsername             func(username string) (u user.User, isNotFoundErr bool, err error)
	get                       func(id int64) (u user.User, isNotFoundErr bool, err error)
}

func (m mockUserService) CreateNewOrganizationUser(w context.Context, organizationUsername string) (user.User, error) {
	return m.createNewOrganizationUser(organizationUsername)
}
func (m mockUserService) GetByUsername(r context.Context, username string) (user.User, bool, error) {
	return m.getByUsername(username)
}
func (m mockUserService) Get(r context.Context, id int64) (u user.User, isNotFoundErr bool, err error) {
	return m.get(id)
}

type mockPermissionsService struct {
	grantPermissionIfNotExists    func(userId int64, p permissions.Permission, assetId string) (bool, error)
	hasPermission                 func(userId int64, p permissions.Permission, assetId string) (bool, error)
	getUsersWithPermission        func(assetId string, p permissions.Permission) (iterator.I[int64], error)
	revokePermissionIfExists      func(userId int64, p permissions.Permission, assetId string) error
	getUserAssetIdsWithPermission func(userId int64, p ...permissions.Permission) (iterator.I[string], error)
	countUserAssetsWithPermission func(userId int64, permission permissions.Permission) (int64, error)
	countUsersWithPermission      func(assetId string, p permissions.Permission) (int64, error)
}

func (m mockPermissionsService) GrantPermissionIfNotExists(wl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	return m.grantPermissionIfNotExists(userId, p, assetId)
}
func (m mockPermissionsService) HasPermission(rl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	return m.hasPermission(userId, p, assetId)
}
func (m mockPermissionsService) GetUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error) {
	return m.getUsersWithPermission(assetId, p)
}
func (m mockPermissionsService) RevokePermissionIfExists(wl context.Context, userId int64, p permissions.Permission, assetId string) error {
	return m.revokePermissionIfExists(userId, p, assetId)
}
func (m mockPermissionsService) GetUserAssetIdsWithPermission(rl context.Context, userId int64, p ...permissions.Permission) (iterator.I[string], error) {
	return m.getUserAssetIdsWithPermission(userId, p...)
}
func (m mockPermissionsService) CountUserAssetsWithPermission(rl context.Context, userId int64, p permissions.Permission) (int64, error) {
	return m.countUserAssetsWithPermission(userId, p)
}
func (m mockPermissionsService) CountUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (int64, error) {
	return m.countUsersWithPermission(assetId, p)
}

type mockStripeClient struct {
	getNewCustomerPortalSession func(userId int64, stripeCustomerID string) (portalSessionUrl string, err error)
}

func (m mockStripeClient) GetNewCustomerPortalSession(userID int64, stripeCustomerID string) (portalSessionUrl string, err error) {
	return m.getNewCustomerPortalSession(userID, stripeCustomerID)
}

type mockTrackQueue struct {
	getLimits func(ownerId int64) (maxJobs int, maxTimeout time.Duration, err error)
}

func (m mockTrackQueue) GetLimits(ownerId int64, tx context.Context) (maxJobs int, maxTimeout time.Duration, err error) {
	return m.getLimits(ownerId)
}

type mockOrgHelper struct {
	grantUserPermissionToOrgIfNotExist func(userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error)
	revokeUserPermissionFromOrgIfExist func(userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error)
	orgCanAddOwnerOrMember             func(org user.User, numberOfOwners int64, numberOfMembers int64) (bool, error)
}

func (m mockOrgHelper) GrantUserPermissionToOrgIfNotExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error) {
	return m.grantUserPermissionToOrgIfNotExist(userId, orgAssetId, p)
}
func (m mockOrgHelper) RevokeUserPermissionFromOrgIfExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error) {
	return m.revokeUserPermissionFromOrgIfExist(userId, orgAssetId, p)
}
func (m mockOrgHelper) OrgCanAddOwnerOrMember(org user.User, numberOfOwners int64, numberOfMembers int64) (bool, error) {
	return m.orgCanAddOwnerOrMember(org, numberOfOwners, numberOfMembers)
}

type mockRepoService struct {
	getAllByOwnerId func(ownerId int64) (iterator.I[repo.Repo], error)
}

func (m mockRepoService) GetAllByOwnerId(rl context.Context, ownerId int64) (iterator.I[repo.Repo], error) {
	return m.getAllByOwnerId(ownerId)
}

func createHandlePostCreateOrganizationReq(
	orgNameParam string,
	userWithSubId int64,
	organizationFeatureIsEnabled bool,
) wrappers.UserWithSubMuxRequest {

	form := url.Values{}
	form.Set(routes.NewOrganizationNameParamName, orgNameParam)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return wrappers.UserWithSubMuxRequest{
		Request:     req,
		Flags:       featureflags.Flags{OrganizationFeatureIsEnabled: organizationFeatureIsEnabled},
		UserWithSub: user.User{Id: userWithSubId},
	}
}

func createHandlePostGrantOwnerOrMemberPermToUserReq(
	usernameParam string,
	permissionParam string,
	userWithOwnerPermissionId int64,
	orgId int64,
	organizationFeatureIsEnabled bool,
) wrappers.OrgOwnerMuxRequest {

	form := url.Values{}
	form.Set(routes.UsernameParameterName, usernameParam)
	form.Set(routes.PermissionParamName, permissionParam)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return wrappers.OrgOwnerMuxRequest{
		Request:                 req,
		Flags:                   featureflags.Flags{OrganizationFeatureIsEnabled: organizationFeatureIsEnabled},
		UserWithOwnerPermission: user.User{Id: userWithOwnerPermissionId},
		Org:                     user.User{Id: orgId},
	}
}

func createHandlePostRevokeOwnerOrMemberPermToUserReq(
	usernameParam string,
	permissionParam string,
	userWithOwnerPermissionId int64,
	orgId int64,
	organizationFeatureIsEnabled bool,
) wrappers.OrgOwnerMuxRequest {

	form := url.Values{}
	form.Set(routes.UsernameParameterName, usernameParam)
	form.Set(routes.PermissionParamName, permissionParam)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return wrappers.OrgOwnerMuxRequest{
		Request:                 req,
		Flags:                   featureflags.Flags{OrganizationFeatureIsEnabled: organizationFeatureIsEnabled},
		UserWithOwnerPermission: user.User{Id: userWithOwnerPermissionId},
		Org:                     user.User{Id: orgId},
	}
}

func createHandleGetOrganizationPageReq(
	userWithOwnerOrMemberPermissionId int64,
	orgId int64,
	organizationFeatureIsEnabled bool,
) wrappers.OrgOwnerOrMemberMuxRequest {

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	return wrappers.OrgOwnerOrMemberMuxRequest{
		Request:                         req,
		Flags:                           featureflags.Flags{OrganizationFeatureIsEnabled: organizationFeatureIsEnabled},
		UserWithOwnerOrMemberPermission: user.User{Id: userWithOwnerOrMemberPermissionId},
		Org:                             user.User{Id: orgId},
	}
}

type fakeInt64Iterator struct {
	values []int64
	index  int
}

func (it *fakeInt64Iterator) Next() bool {
	return it.index < len(it.values)
}
func (it *fakeInt64Iterator) Get() (int64, error) {
	v := it.values[it.index]
	it.index++
	return v, nil
}
func (it *fakeInt64Iterator) Err() error {
	return nil
}

type fakeStringIterator struct {
	values []string
	index  int
	err    error
}

func (it *fakeStringIterator) Next() bool {
	return it.index < len(it.values)
}

func (it *fakeStringIterator) Get() (string, error) {
	v := it.values[it.index]
	it.index++
	return v, nil
}

func (it *fakeStringIterator) Err() error {
	return it.err
}
