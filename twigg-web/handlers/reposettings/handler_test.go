package reposettings

import (
	"context"
	"errors"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	reposervice "monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/server"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPushToGitMirrorPayloadArgsGob(t *testing.T) {
	// original struct
	original := &pushTopToGitMirrorPayloadArgs{
		RepoId:     123,
		GitRepoUrl: "https://example.com/repo.git",
	}

	// encode
	payload, err := original.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// decode into a new struct
	decoded := &pushTopToGitMirrorPayloadArgs{}
	if err := decoded.decode(payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.RepoId != original.RepoId {
		t.Fatalf("RepoId mismatch: got %v, want %v", decoded.RepoId, original.RepoId)
	}
	if decoded.GitRepoUrl != original.GitRepoUrl {
		t.Fatalf("gitRepoUrl mismatch: got %v, want %v", decoded.GitRepoUrl, original.GitRepoUrl)
	}
}

func TestHandlePostRemoveRepoPermission(t *testing.T) {
	var userToRevoke = user.User{Id: 999, Username: "john"}
	var orgId int64 = 456
	var repoId uint64 = 789

	tests := []struct {
		name                 string
		repoOwnerIsOrg       bool
		hasPermissionResult  bool
		hasPermissionErr     error
		expectedStatus       int
		expectedBody         string
		expectedShouldCommit bool
		expectRevokeCalled   bool
	}{
		{
			name:                 "org repo, HasPermission error",
			repoOwnerIsOrg:       true,
			hasPermissionErr:     errors.New("boom"),
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "Internal err checking permissions",
			expectedShouldCommit: false,
			expectRevokeCalled:   false,
		},
		{
			name:                 "org repo, user is org owner",
			repoOwnerIsOrg:       true,
			hasPermissionResult:  true,
			expectedStatus:       http.StatusBadRequest,
			expectedBody:         "can not revoke permission from a org owner",
			expectedShouldCommit: false,
			expectRevokeCalled:   false,
		},
		{
			name:                 "org repo, user is not org owner",
			repoOwnerIsOrg:       true,
			hasPermissionResult:  false,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectRevokeCalled:   true,
		},
		{
			name:                 "non-org repo, skips owner check",
			repoOwnerIsOrg:       false,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectRevokeCalled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledRevoke := false

			mockedUserSrv := mockRepoSettingsUserService{
				getByUsername: func(_ string) (user.User, bool, error) {
					return userToRevoke, false, nil
				},
			}
			mockedPermSrv := mockRepoSettingsDb{
				hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
					if !tt.repoOwnerIsOrg {
						t.Fatal("HasPermission should not be called for non-org repo")
					}
					if userId != userToRevoke.Id {
						t.Fatalf("unexpected userId: %d", userId)
					}
					if p != permissions.Permission_OrganizationOwner {
						t.Fatalf("unexpected permission: %v", p)
					}
					if assetId != permissions.OrganizationAssetId(orgId) {
						t.Fatalf("unexpected assetId: %q", assetId)
					}
					return tt.hasPermissionResult, tt.hasPermissionErr
				},
				revokePermissionIfExists: func(userId int64, p permissions.Permission, assetId string) error {
					if !tt.expectRevokeCalled {
						t.Fatal("RevokePermissionIfExists should not be called")
					}
					calledRevoke = true
					if userId != userToRevoke.Id {
						t.Fatalf("unexpected userId: %d", userId)
					}
					if p != permissions.Permission_WriteRepo {
						t.Fatalf("unexpected permission: %v", p)
					}
					if assetId != permissions.RepoAssetId(repoId) {
						t.Fatalf("unexpected assetId: %q", assetId)
					}
					return nil
				},
			}
			h := handler{userS: mockedUserSrv, db: mockedPermSrv}

			rr := httptest.NewRecorder()
			req := newRemoveRepoPermissionReq(userToRevoke.Username, tt.repoOwnerIsOrg, orgId, repoId)

			shouldCommit := h.handlePostRemoveRepoPermission(rr, req, nil)

			if shouldCommit != tt.expectedShouldCommit {
				t.Fatalf("expected shouldCommit %v, got %v", tt.expectedShouldCommit, shouldCommit)
			}
			if tt.expectRevokeCalled && !calledRevoke {
				t.Fatal("expected RevokePermissionIfExists to be called")
			}
			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if tt.expectedBody != "" && strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Fatalf("expected body %q, got %q", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestHandlePostAddRepoPermission(t *testing.T) {
	var orgId int64 = 456
	var repoId uint64 = 789

	tests := []struct {
		name                 string
		userToAdd            user.User
		repoOwnerIsOrg       bool
		isOrgOwner           bool
		isOrgMember          bool
		ownerCheckErr        error
		memberCheckErr       error
		hadPermission        bool
		expectedStatus       int
		expectedBody         string
		expectedShouldCommit bool
		expectGrantCalled    bool
	}{
		{
			name:                 "try to grant perm on non-org repo, expect handler to return 200 and 'ok'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       false,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectGrantCalled:    true,
		},
		{
			name:                 "try to grant perm to user that is repo owner, expect handler to return 200 and 'Already had'",
			userToAdd:            user.User{Id: orgId, Username: "owner"},
			repoOwnerIsOrg:       false,
			expectedStatus:       http.StatusOK,
			expectedBody:         "Already had",
			expectedShouldCommit: false,
			expectGrantCalled:    false,
		},
		{
			name:                 "try to grant perm on org repo when owner check errors, expect handler to return 500 and 'Internal err checking permissions'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       true,
			ownerCheckErr:        errors.New("boom"),
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "Internal err checking permissions",
			expectedShouldCommit: false,
			expectGrantCalled:    false,
		},
		{
			name:                 "try to grant perm on org repo when member check errors, expect handler to return 500 and 'Internal err checking permissions'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       true,
			isOrgOwner:           false,
			memberCheckErr:       errors.New("boom"),
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "Internal err checking permissions",
			expectedShouldCommit: false,
			expectGrantCalled:    false,
		},
		{
			name:                 "try to grant perm to user not in org, expect handler to return 400 and 'can not add user that is not part of the organization'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       true,
			isOrgOwner:           false,
			isOrgMember:          false,
			expectedStatus:       http.StatusBadRequest,
			expectedBody:         "can not add user that is not part of the organization",
			expectedShouldCommit: false,
			expectGrantCalled:    false,
		},
		{
			name:                 "try to grant perm to org owner on org repo, expect handler to return 200 and 'ok'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       true,
			isOrgOwner:           true,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectGrantCalled:    true,
		},
		{
			name:                 "try to grant perm to org member on org repo, expect handler to return 200 and 'ok'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       true,
			isOrgOwner:           false,
			isOrgMember:          true,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectGrantCalled:    true,
		},
		{
			name:                 "try to grant perm to org member that already had write permission, expect handler to return 200 and 'Already had'",
			userToAdd:            user.User{Id: 999, Username: "alice"},
			repoOwnerIsOrg:       true,
			isOrgOwner:           false,
			isOrgMember:          true,
			hadPermission:        true,
			expectedStatus:       http.StatusOK,
			expectedBody:         "Already had",
			expectedShouldCommit: false,
			expectGrantCalled:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledGrant := false

			mockedUserSrv := mockRepoSettingsUserService{
				getByUsername: func(_ string) (user.User, bool, error) {
					return tt.userToAdd, false, nil
				},
			}
			mockedPermSrv := mockRepoSettingsDb{
				hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
					if !tt.repoOwnerIsOrg {
						t.Fatal("HasPermission should not be called for non-org repo")
					}
					if userId != tt.userToAdd.Id {
						t.Fatalf("unexpected userId: %d", userId)
					}
					if assetId != permissions.OrganizationAssetId(orgId) {
						t.Fatalf("unexpected assetId: %q", assetId)
					}
					switch p {
					case permissions.Permission_OrganizationOwner:
						return tt.isOrgOwner, tt.ownerCheckErr
					case permissions.Permission_OrganizationMember:
						return tt.isOrgMember, tt.memberCheckErr
					default:
						t.Fatalf("unexpected permission: %v", p)
						return false, nil
					}
				},
				grantPermissionIfNotExists: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
					if !tt.expectGrantCalled {
						t.Fatal("GrantPermissionIfNotExists should not be called")
					}
					calledGrant = true
					if userId != tt.userToAdd.Id {
						t.Fatalf("unexpected userId: %d", userId)
					}
					if p != permissions.Permission_WriteRepo {
						t.Fatalf("unexpected permission: %v", p)
					}
					if assetId != permissions.RepoAssetId(repoId) {
						t.Fatalf("unexpected assetId: %q", assetId)
					}
					return tt.hadPermission, nil
				},
			}
			h := handler{userS: mockedUserSrv, db: mockedPermSrv}

			rr := httptest.NewRecorder()
			req := newAddRepoPermissionReq(tt.userToAdd.Username, tt.repoOwnerIsOrg, orgId, repoId)

			shouldCommit := h.handlePostAddRepoPermission(rr, req, nil)

			if shouldCommit != tt.expectedShouldCommit {
				t.Fatalf("expected shouldCommit %v, got %v", tt.expectedShouldCommit, shouldCommit)
			}
			if tt.expectGrantCalled && !calledGrant {
				t.Fatal("expected GrantPermissionIfNotExists to be called")
			}
			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if tt.expectedBody != "" && strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Fatalf("expected body %q, got %q", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestHandlePostArchive(t *testing.T) {
	var loggedInUserId int64 = 1
	var orgId int64 = 3
	var repoId uint64 = 42

	tests := []struct {
		name                 string
		repoOwnerIsOrg       bool
		ownerId              int64
		hasOrgOwnerPerm      bool
		hasPermissionErr     error
		expectedStatus       int
		expectedBody         string
		expectedShouldCommit bool
		expectArchiveCalled  bool
	}{
		{
			name:                 "try to archive non-org repo as owner, expect 200 and 'ok'",
			repoOwnerIsOrg:       false,
			ownerId:              loggedInUserId,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectArchiveCalled:  true,
		},
		{
			name:                 "try to archive non-org repo as non-owner, expect 400 and 'only owner can archive repo'",
			repoOwnerIsOrg:       false,
			ownerId:              2,
			expectedStatus:       http.StatusBadRequest,
			expectedBody:         "only owner can archive repo",
			expectedShouldCommit: false,
			expectArchiveCalled:  false,
		},
		{
			name:                 "try to archive org repo when HasPermission errors, expect 500 and 'failed to check org owner permission'",
			repoOwnerIsOrg:       true,
			ownerId:              orgId,
			hasPermissionErr:     errors.New("boom"),
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "failed to check org owner permission",
			expectedShouldCommit: false,
			expectArchiveCalled:  false,
		},
		{
			name:                 "try to archive org repo as non-org-owner, expect 400 and 'only org owner can archive org repo'",
			repoOwnerIsOrg:       true,
			ownerId:              orgId,
			hasOrgOwnerPerm:      false,
			expectedStatus:       http.StatusBadRequest,
			expectedBody:         "only org owner can archive org repo",
			expectedShouldCommit: false,
			expectArchiveCalled:  false,
		},
		{
			name:                 "try to archive org repo as org owner, expect 200 and 'ok'",
			repoOwnerIsOrg:       true,
			ownerId:              orgId,
			hasOrgOwnerPerm:      true,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
			expectArchiveCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledArchive := false

			mockedPermSrv := mockRepoSettingsDb{
				hasPermission: func(
					userId int64,
					p permissions.Permission,
					assetId string,
				) (bool, error) {
					if !tt.repoOwnerIsOrg {
						t.Fatal("HasPermission should not be called for non-org repo")
					}
					if userId != loggedInUserId {
						t.Fatalf("unexpected userId: %d", userId)
					}
					if p != permissions.Permission_OrganizationOwner {
						t.Fatalf("unexpected permission: %v", p)
					}
					if assetId !=
						permissions.OrganizationAssetId(orgId) {
						t.Fatalf("unexpected assetId: %q", assetId)
					}
					return tt.hasOrgOwnerPerm, tt.hasPermissionErr
				},
				revokeAllPermissionsToAsset: func(assetId string) error {
					if assetId !=
						permissions.RepoAssetId(repoId) {
						t.Fatalf("unexpected assetId: %q", assetId)
					}
					return nil
				},
			}
			mockedRepoSrv := mockRepoSettingsRepoService{
				archiveRepo: func(ownerIdArg int64, rid uint64) error {
					calledArchive = true
					if ownerIdArg != tt.ownerId {
						t.Fatalf("unexpected ownerId: %d", ownerIdArg)
					}
					if rid != repoId {
						t.Fatalf("unexpected repoId: %d", rid)
					}
					return nil
				},
			}
			h := handler{db: mockedPermSrv, repoS: mockedRepoSrv}

			rr := httptest.NewRecorder()
			req := newArchiveRepoReq(
				tt.repoOwnerIsOrg, tt.ownerId, loggedInUserId, repoId)

			shouldCommit := h.handlePostArchive(rr, req, nil)

			if shouldCommit != tt.expectedShouldCommit {
				t.Fatalf("expected shouldCommit %v, got %v",
					tt.expectedShouldCommit, shouldCommit)
			}
			if tt.expectArchiveCalled && !calledArchive {
				t.Fatal("expected ArchiveRepo to be called")
			}
			if !tt.expectArchiveCalled && calledArchive {
				t.Fatal("expected ArchiveRepo not to be called")
			}
			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if tt.expectedBody != "" &&
				strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Fatalf("expected body %q, got %q",
					tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestHandlePostSetPublic(t *testing.T) {
	const ownerId int64 = 7
	const repoId uint64 = 42
	const repoName = "my-repo"

	tests := []struct {
		name                 string
		setPublicErr         error
		expectedStatus       int
		expectedBody         string
		expectedShouldCommit bool
	}{
		{
			name:                 "set repo public, expect 200 and 'ok'",
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
		},
		{
			name:                 "SetPublic errors, expect 500 and no commit",
			setPublicErr:         errors.New("boom"),
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "failed to set repo public",
			expectedShouldCommit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledSetPublic := false
			mockedRepoSrv := mockRepoSettingsRepoService{
				setPublic: func(gotOwnerId int64, gotDisplayName string) error {
					calledSetPublic = true
					if gotOwnerId != ownerId {
						t.Fatalf("unexpected ownerId: %d", gotOwnerId)
					}
					if gotDisplayName != repoName {
						t.Fatalf("unexpected displayName: %q", gotDisplayName)
					}
					return tt.setPublicErr
				},
			}
			h := handler{repoS: mockedRepoSrv}

			rr := httptest.NewRecorder()
			req := wrappers.UserRepoMuxRequest{
				Request: httptest.NewRequest(http.MethodPost, "/", nil),
				Repo: repo.Repo{
					Id:          repoId,
					OwnerId:     ownerId,
					DisplayName: repoName,
				},
			}

			shouldCommit := h.handlePostSetPublic(rr, req, nil)

			if !calledSetPublic {
				t.Fatal("expected SetPublic to be called")
			}
			if shouldCommit != tt.expectedShouldCommit {
				t.Fatalf("expected shouldCommit %v, got %v",
					tt.expectedShouldCommit, shouldCommit)
			}
			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Fatalf("expected body %q, got %q",
					tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestHandlePostSetPrivate(t *testing.T) {
	const ownerId int64 = 7
	const repoId uint64 = 42
	const repoName = "my-repo"

	tests := []struct {
		name                 string
		setPrivateErr        error
		expectedStatus       int
		expectedBody         string
		expectedShouldCommit bool
	}{
		{
			name:                 "set repo private, expect 200 and 'ok'",
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
		},
		{
			name:                 "SetPrivate errors, expect 500 and no commit",
			setPrivateErr:        errors.New("boom"),
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "failed to set repo private",
			expectedShouldCommit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledSetPrivate := false
			mockedRepoSrv := mockRepoSettingsRepoService{
				setPrivate: func(gotOwnerId int64, gotDisplayName string) error {
					calledSetPrivate = true
					if gotOwnerId != ownerId {
						t.Fatalf("unexpected ownerId: %d", gotOwnerId)
					}
					if gotDisplayName != repoName {
						t.Fatalf("unexpected displayName: %q", gotDisplayName)
					}
					return tt.setPrivateErr
				},
			}
			h := handler{repoS: mockedRepoSrv}

			rr := httptest.NewRecorder()
			req := wrappers.UserRepoMuxRequest{
				Request: httptest.NewRequest(http.MethodPost, "/", nil),
				Repo: repo.Repo{
					Id:          repoId,
					OwnerId:     ownerId,
					DisplayName: repoName,
				},
			}

			shouldCommit := h.handlePostSetPrivate(rr, req, nil)

			if !calledSetPrivate {
				t.Fatal("expected SetPrivate to be called")
			}
			if shouldCommit != tt.expectedShouldCommit {
				t.Fatalf("expected shouldCommit %v, got %v",
					tt.expectedShouldCommit, shouldCommit)
			}
			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Fatalf("expected body %q, got %q",
					tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func newArchiveRepoReq(
	repoOwnerIsOrg bool,
	ownerId int64,
	loggedInUserId int64,
	repoId uint64,
) wrappers.UserRepoMuxRequest {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	return wrappers.UserRepoMuxRequest{
		Request: req,
		RepoOwnerUsr: user.User{
			Id:             ownerId,
			IsOrganization: repoOwnerIsOrg,
		},
		Repo: repo.Repo{
			Id:      repoId,
			OwnerId: ownerId,
		},
		UserWithWritePermission: user.User{
			Id: loggedInUserId,
		},
	}
}

func TestHandlePostSetDescription(t *testing.T) {
	const ownerId int64 = 7
	const repoId uint64 = 42
	const repoName = "my-repo"
	const newDescription = "a new description"

	tests := []struct {
		name                 string
		description          string
		setDescriptionErr    error
		expectDbCalled       bool
		expectedStatus       int
		expectedBody         string
		expectedShouldCommit bool
	}{
		{
			name:                 "set repo description, expect 200 and 'ok'",
			description:          newDescription,
			expectDbCalled:       true,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
		},
		{
			name:                 "empty description is allowed, expect 200 and 'ok'",
			description:          "",
			expectDbCalled:       true,
			expectedStatus:       http.StatusOK,
			expectedBody:         "ok",
			expectedShouldCommit: true,
		},
		{
			name:                 "description longer than max, expect 400 and no db call",
			description:          strings.Repeat("a", reposervice.MaxDescriptionLength+1),
			expectDbCalled:       false,
			expectedStatus:       http.StatusBadRequest,
			expectedBody:         "got too long description",
			expectedShouldCommit: false,
		},
		{
			name:                 "SetRepoDescription errors, expect 500 and no commit",
			description:          newDescription,
			setDescriptionErr:    errors.New("boom"),
			expectDbCalled:       true,
			expectedStatus:       http.StatusInternalServerError,
			expectedBody:         "failed to set repo description",
			expectedShouldCommit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledSetDescription := false
			mockedDb := mockRepoSettingsDb{
				setRepoDescription: func(gotOwnerId int64, gotDisplayName, gotDescription string) error {
					calledSetDescription = true
					if gotOwnerId != ownerId {
						t.Fatalf("unexpected ownerId: %d", gotOwnerId)
					}
					if gotDisplayName != repoName {
						t.Fatalf("unexpected displayName: %q", gotDisplayName)
					}
					if gotDescription != tt.description {
						t.Fatalf("unexpected description: %q", gotDescription)
					}
					return tt.setDescriptionErr
				},
			}
			h := handler{db: mockedDb}

			form := url.Values{}
			form.Set(routes.RepoDescriptionParamName, tt.description)
			httpReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
			httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			rr := httptest.NewRecorder()
			req := wrappers.UserRepoMuxRequest{
				Request: httpReq,
				Repo: repo.Repo{
					Id:          repoId,
					OwnerId:     ownerId,
					DisplayName: repoName,
				},
			}

			shouldCommit := h.handlePostSetDescription(rr, req, nil)

			if calledSetDescription != tt.expectDbCalled {
				t.Fatalf("expected db called %v, got %v",
					tt.expectDbCalled, calledSetDescription)
			}
			if shouldCommit != tt.expectedShouldCommit {
				t.Fatalf("expected shouldCommit %v, got %v",
					tt.expectedShouldCommit, shouldCommit)
			}
			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Fatalf("expected body %q, got %q",
					tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func newAddRepoPermissionReq(
	usernameParam string,
	repoOwnerIsOrg bool,
	repoOwnerOrgId int64,
	repoId uint64,
) wrappers.UserRepoMuxRequest {
	form := url.Values{}
	form.Set(routes.UsernameParameterName, usernameParam)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return wrappers.UserRepoMuxRequest{
		Request: req,
		RepoOwnerUsr: user.User{
			Id:             repoOwnerOrgId,
			IsOrganization: repoOwnerIsOrg,
		},
		Repo: repo.Repo{Id: repoId},
	}
}

func newRemoveRepoPermissionReq(
	usernameParam string,
	repoOwnerIsOrg bool,
	repoOwnerOrgId int64,
	repoId uint64,
) wrappers.UserRepoMuxRequest {
	form := url.Values{}
	form.Set(routes.UsernameParameterName, usernameParam)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return wrappers.UserRepoMuxRequest{
		Request: req,
		RepoOwnerUsr: user.User{
			Id:             repoOwnerOrgId,
			IsOrganization: repoOwnerIsOrg,
		},
		Repo: repo.Repo{Id: repoId},
	}
}

type mockRepoSettingsUserService struct {
	getByUsername func(username string) (user.User, bool, error)
}

func (m mockRepoSettingsUserService) Get(_ context.Context, _ int64) (user.User, bool, error) {
	panic("unexpected call to Get")
}

func (m mockRepoSettingsUserService) GetByUsername(_ context.Context, username string) (user.User, bool, error) {
	return m.getByUsername(username)
}

type mockRepoSettingsDb struct {
	hasPermission               func(userId int64, p permissions.Permission, assetId string) (bool, error)
	revokePermissionIfExists    func(userId int64, p permissions.Permission, assetId string) error
	grantPermissionIfNotExists  func(userId int64, p permissions.Permission, assetId string) (bool, error)
	revokeAllPermissionsToAsset func(assetId string) error
	setRepoDescription          func(ownerId int64, displayName, description string) error
}

func (m mockRepoSettingsDb) SetRepoDescription(_ context.Context, ownerId int64, displayName, description string) error {
	return m.setRepoDescription(ownerId, displayName, description)
}

func (m mockRepoSettingsDb) HasPermission(_ context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	return m.hasPermission(userId, p, assetId)
}

func (m mockRepoSettingsDb) GrantPermissionIfNotExists(_ context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	if m.grantPermissionIfNotExists == nil {
		panic("unexpected call to GrantPermissionIfNotExists")
	}
	return m.grantPermissionIfNotExists(userId, p, assetId)
}

func (m mockRepoSettingsDb) RevokePermissionIfExists(_ context.Context, userId int64, p permissions.Permission, assetId string) error {
	return m.revokePermissionIfExists(userId, p, assetId)
}

func (m mockRepoSettingsDb) RevokeAllPermissionsToAsset(_ context.Context, assetId string) error {
	return m.revokeAllPermissionsToAsset(assetId)
}

func (m mockRepoSettingsDb) GetUsersWithPermission(_ context.Context, _ string, _ permissions.Permission) (iterator.I[int64], error) {
	panic("unexpected call to GetUsersWithPermission")
}

func (m mockRepoSettingsDb) BeginRead() (context.Context, func(), error) {
	panic("unexpected call to BeginRead")
}

type mockRepoSettingsRepoService struct {
	archiveRepo func(ownerId int64, repoId uint64) error
	setPublic   func(ownerId int64, displayName string) error
	setPrivate  func(ownerId int64, displayName string) error
}

func (m mockRepoSettingsRepoService) ArchiveRepo(_ context.Context, ownerId int64, repoId uint64) error {
	return m.archiveRepo(ownerId, repoId)
}
func (m mockRepoSettingsRepoService) SetPublic(_ context.Context, ownerId int64, displayName string) error {
	return m.setPublic(ownerId, displayName)
}
func (m mockRepoSettingsRepoService) SetPrivate(_ context.Context, ownerId int64, displayName string) error {
	return m.setPrivate(ownerId, displayName)
}
func (m mockRepoSettingsRepoService) SetGitMirrorEnabled(_ context.Context, _ int64, _ string, _ bool) error {
	panic("unexpected call to SetGitMirrorEnabled")
}
func (m mockRepoSettingsRepoService) SetGitMirrorUrl(_ context.Context, _ uint64, _ int64, _ string, _ string) error {
	panic("unexpected call to SetGitMirrorUrl")
}
func (m mockRepoSettingsRepoService) GetServerByRepoId(_ context.Context, _ uint64) (server.Server, error) {
	panic("unexpected call to GetServerByRepoId")
}
func (m mockRepoSettingsRepoService) GetServerRead(_ context.Context) server.Read {
	panic("unexpected call to GetServerRead")
}
func (m mockRepoSettingsRepoService) GetServer(_ context.Context, _ int64, _ string) (server.Server, bool, error) {
	panic("unexpected call to GetServer")
}
func (m mockRepoSettingsRepoService) GetServerWrite(_ context.Context) server.Write {
	panic("unexpected call to GetServerWrite")
}