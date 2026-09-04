package orghelper

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"testing"
)

type revokeCall struct {
	userId     int64
	permission permissions.Permission
	assetId    string
}

func TestEnforceOrgSeatLimit(t *testing.T) {
	orgAssetId := permissions.OrganizationAssetId(1_000)

	tests := []struct {
		name          string
		owners        []int64
		members       []int64
		orgRepos      []uint64
		numberOfSeats int64
		wantRevoked   []revokeCall
		wantErr       bool
	}{
		{
			name:          "within_limit_noop",
			owners:        []int64{1},
			members:       []int64{2, 3, 4},
			numberOfSeats: 100,
			wantRevoked:   nil,
		},
		{
			name:          "at_exact_limit_noop",
			owners:        []int64{1, 2},
			members:       []int64{3, 4, 5},
			numberOfSeats: 5,
			wantRevoked:   nil,
		},
		{
			name:          "evict_some_members",
			owners:        []int64{1},
			members:       []int64{101, 102, 103},
			numberOfSeats: 2,
			// 1+3=4 total, limit=2: remove 2 members
			wantRevoked: []revokeCall{
				{101, permissions.Permission_OrganizationMember, orgAssetId},
				{102, permissions.Permission_OrganizationMember, orgAssetId},
			},
		},
		{
			name:          "evict_all_members_owners_preserved",
			owners:        []int64{1, 2},
			members:       []int64{101, 102, 103},
			numberOfSeats: 2,
			// 2+3=5 total, limit=2: remove all 3 members, both owners untouched
			wantRevoked: []revokeCall{
				{101, permissions.Permission_OrganizationMember, orgAssetId},
				{102, permissions.Permission_OrganizationMember, orgAssetId},
				{103, permissions.Permission_OrganizationMember, orgAssetId},
			},
		},
		{
			name:          "evict_members_then_owners_last_owner_kept",
			owners:        []int64{10, 20, 30},
			members:       []int64{101, 102},
			numberOfSeats: 1,
			// 3+2=5 total, limit=1: remove all 2 members then 2 owners; 1 owner always kept
			wantRevoked: []revokeCall{
				{101, permissions.Permission_OrganizationMember, orgAssetId},
				{102, permissions.Permission_OrganizationMember, orgAssetId},
				{10, permissions.Permission_OrganizationOwner, orgAssetId},
				{20, permissions.Permission_OrganizationOwner, orgAssetId},
			},
		},
		{
			name:          "evicted_users_lose_all_repo_permissions",
			owners:        []int64{1, 2},
			members:       []int64{101, 102},
			orgRepos:      []uint64{500, 501},
			numberOfSeats: 1,
			// evicted members also lose WriteRepo + ReadRepo on every org repo
			wantRevoked: []revokeCall{
				// Member 101
				{101, permissions.Permission_OrganizationMember, orgAssetId},
				{101, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{101, permissions.Permission_ReadRepo, permissions.RepoAssetId(500)},
				{101, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
				{101, permissions.Permission_ReadRepo, permissions.RepoAssetId(501)},
				// Member 102
				{102, permissions.Permission_OrganizationMember, orgAssetId},
				{102, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{102, permissions.Permission_ReadRepo, permissions.RepoAssetId(500)},
				{102, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
				{102, permissions.Permission_ReadRepo, permissions.RepoAssetId(501)},
				// Owner 1
				{1, permissions.Permission_OrganizationOwner, orgAssetId},
				{1, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{1, permissions.Permission_ReadRepo, permissions.RepoAssetId(500)},
				{1, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
				{1, permissions.Permission_ReadRepo, permissions.RepoAssetId(501)},
			},
		},
		{
			name:          "zero_seats_returns_error",
			numberOfSeats: 0,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotRevoked []revokeCall

			mock := mockPermissionsService{
				countUsersWithPermission: func(assetId string, p permissions.Permission) (int64, error) {
					if assetId != orgAssetId {
						t.Fatalf("expected assetId, got: %s", assetId)
					}
					if p == permissions.Permission_OrganizationOwner {
						return int64(len(tc.owners)), nil
					}
					return int64(len(tc.members)), nil
				},
				getUsersWithPermission: func(assetId string, p permissions.Permission) (iterator.I[int64], error) {
					if assetId != orgAssetId {
						t.Fatalf("expected assetId, got: %s", assetId)
					}
					if p == permissions.Permission_OrganizationOwner {
						return iterator.NewIterFromSlice(tc.owners), nil
					}
					if p == permissions.Permission_OrganizationMember {
						return iterator.NewIterFromSlice(tc.members), nil
					}
					t.Fatalf("expected permission, got: %d", p)
					return nil, nil
				},
				revokePermissionIfExists: func(userId int64, p permissions.Permission, assetId string) error {
					gotRevoked = append(gotRevoked, revokeCall{userId, p, assetId})
					return nil
				},
			}

			mockRepos := mockOrgReposSrv{
				getAllOrgRepoIds: func(assetId string) ([]uint64, error) {
					if assetId != orgAssetId {
						t.Fatalf("expected assetId, got: %s", assetId)
					}
					return tc.orgRepos, nil
				},
			}

			h := NewHelper(mock, mockRepos)
			err := h.EnforceOrgSeatLimit(t.Context(), orgAssetId, tc.numberOfSeats)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			checkIsEqualIgnoreOrder(t, gotRevoked, tc.wantRevoked)
		})
	}
}

type grantCall struct {
	userId     int64
	permission permissions.Permission
	assetId    string
}

func TestGrantUserPermissionToOrgIfNotExist(t *testing.T) {
	orgAssetId := permissions.OrganizationAssetId(1_000)

	tests := []struct {
		name               string
		userId             int64
		permission         permissions.Permission
		orgRepos           []uint64
		grantAlreadyExists bool
		wantAlreadyExists  bool
		wantGranted        []grantCall
		wantErr            bool
	}{
		{
			name:       "invalid_permission_returns_error",
			userId:     1,
			permission: permissions.Permission_WriteRepo,
			wantErr:    true,
		},
		{
			name:               "already_exists_as_member_returns_early_no_repo_grants",
			userId:             1,
			permission:         permissions.Permission_OrganizationMember,
			grantAlreadyExists: true,
			wantAlreadyExists:  true,
			wantGranted: []grantCall{
				{1, permissions.Permission_OrganizationMember, orgAssetId},
			},
		},
		{
			name:               "already_exists_as_owner_returns_early_no_repo_grants",
			userId:             1,
			permission:         permissions.Permission_OrganizationOwner,
			grantAlreadyExists: true,
			wantAlreadyExists:  true,
			wantGranted: []grantCall{
				{1, permissions.Permission_OrganizationOwner, orgAssetId},
			},
		},
		{
			name:       "new_member_no_repos",
			userId:     1,
			permission: permissions.Permission_OrganizationMember,
			wantGranted: []grantCall{
				{1, permissions.Permission_OrganizationMember, orgAssetId},
			},
		},
		{
			name:       "new_member_grants_write_repo_on_all_repos",
			userId:     42,
			permission: permissions.Permission_OrganizationMember,
			orgRepos:   []uint64{500, 501},
			wantGranted: []grantCall{
				{42, permissions.Permission_OrganizationMember, orgAssetId},
				{42, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{42, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
			},
		},
		{
			name:       "new_owner_grants_write_repo_on_all_repos",
			userId:     42,
			permission: permissions.Permission_OrganizationOwner,
			orgRepos:   []uint64{500, 501},
			wantGranted: []grantCall{
				{42, permissions.Permission_OrganizationOwner, orgAssetId},
				{42, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{42, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotGranted []grantCall

			mock := mockPermissionsService{
				grantPermissionIfNotExists: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
					gotGranted = append(gotGranted, grantCall{userId, p, assetId})
					return tc.grantAlreadyExists, nil
				},
			}
			mockRepos := mockOrgReposSrv{
				getAllOrgRepoIds: func(assetId string) ([]uint64, error) {
					if assetId != orgAssetId {
						t.Fatalf("expected assetId, got: %s", assetId)
					}
					return tc.orgRepos, nil
				},
			}

			h := NewHelper(mock, mockRepos)
			alreadyExists, err := h.GrantUserPermissionToOrgIfNotExist(t.Context(), tc.userId, orgAssetId, tc.permission)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if alreadyExists != tc.wantAlreadyExists {
				t.Fatalf("alreadyExists: got %v, want %v", alreadyExists, tc.wantAlreadyExists)
			}
			checkIsEqualIgnoreOrder(t, gotGranted, tc.wantGranted)
		})
	}
}

func TestRevokeUserPermissionFromOrgIfExist(t *testing.T) {
	orgAssetId := permissions.OrganizationAssetId(1_000)

	tests := []struct {
		name                string
		userId              int64
		permission          permissions.Permission
		owners              []int64
		orgRepos            []uint64
		wantOneOwnerLeftErr bool
		wantRevoked         []revokeCall
		wantErr             bool
	}{
		{
			name:       "invalid_permission_returns_error",
			userId:     1,
			permission: permissions.Permission_WriteRepo,
			wantErr:    true,
		},
		{
			name:                "only_owner_cannot_be_removed",
			userId:              1,
			permission:          permissions.Permission_OrganizationOwner,
			owners:              []int64{1},
			wantOneOwnerLeftErr: true,
			wantErr:             true,
		},
		{
			name:       "owner_removed_when_other_owners_exist",
			userId:     1,
			permission: permissions.Permission_OrganizationOwner,
			owners:     []int64{1, 2},
			wantRevoked: []revokeCall{
				{1, permissions.Permission_OrganizationOwner, orgAssetId},
			},
		},
		{
			name:       "owner_removed_also_loses_repo_perms",
			userId:     1,
			permission: permissions.Permission_OrganizationOwner,
			owners:     []int64{1, 2},
			orgRepos:   []uint64{500, 501},
			wantRevoked: []revokeCall{
				{1, permissions.Permission_OrganizationOwner, orgAssetId},
				{1, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{1, permissions.Permission_ReadRepo, permissions.RepoAssetId(500)},
				{1, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
				{1, permissions.Permission_ReadRepo, permissions.RepoAssetId(501)},
			},
		},
		{
			name:       "member_removed",
			userId:     1,
			permission: permissions.Permission_OrganizationMember,
			wantRevoked: []revokeCall{
				{1, permissions.Permission_OrganizationMember, orgAssetId},
			},
		},
		{
			name:       "member_removed_also_loses_repo_perms",
			userId:     1,
			permission: permissions.Permission_OrganizationMember,
			orgRepos:   []uint64{500, 501},
			wantRevoked: []revokeCall{
				{1, permissions.Permission_OrganizationMember, orgAssetId},
				{1, permissions.Permission_WriteRepo, permissions.RepoAssetId(500)},
				{1, permissions.Permission_ReadRepo, permissions.RepoAssetId(500)},
				{1, permissions.Permission_WriteRepo, permissions.RepoAssetId(501)},
				{1, permissions.Permission_ReadRepo, permissions.RepoAssetId(501)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotRevoked []revokeCall

			mock := mockPermissionsService{
				getUsersWithPermission: func(assetId string, p permissions.Permission) (iterator.I[int64], error) {
					if assetId != orgAssetId {
						t.Fatalf("expected assetId, got: %s", assetId)
					}
					return iterator.NewIterFromSlice(tc.owners), nil
				},
				revokePermissionIfExists: func(userId int64, p permissions.Permission, assetId string) error {
					gotRevoked = append(gotRevoked, revokeCall{userId, p, assetId})
					return nil
				},
			}
			mockRepos := mockOrgReposSrv{
				getAllOrgRepoIds: func(assetId string) ([]uint64, error) {
					if assetId != orgAssetId {
						t.Fatalf("expected assetId, got: %s", assetId)
					}
					return tc.orgRepos, nil
				},
			}

			h := NewHelper(mock, mockRepos)
			oneOwnerLeftErr, err := h.RevokeUserPermissionFromOrgIfExist(t.Context(), tc.userId, orgAssetId, tc.permission)

			if oneOwnerLeftErr != tc.wantOneOwnerLeftErr {
				t.Fatalf("oneOwnerLeftErr: got %v, want %v", oneOwnerLeftErr, tc.wantOneOwnerLeftErr)
			}
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			checkIsEqualIgnoreOrder(t, gotRevoked, tc.wantRevoked)
		})
	}
}

func checkIsEqualIgnoreOrder[T comparable](t *testing.T, s1 []T, s2 []T) {
	if len(s1) != len(s2) {
		t.Fatalf("calls: got %d, want %d\ngot:  %v\nwant: %v", len(s1), len(s2), s1, s2)
	}
	appearances := make(map[T]int)
	for _, r := range s1 {
		appearances[r]++
	}
	for _, r := range s2 {
		appearances[r]++
	}
	for r, appearance := range appearances {
		if appearance > 2 {
			t.Errorf("missing call: %+v", r)
		} else if appearance < 2 {
			t.Errorf("unexpected call: %+v", r)
		}
	}
}

type mockOrgReposSrv struct {
	getAllOrgRepoIds func(orgAssetId string) ([]uint64, error)
}

func (m mockOrgReposSrv) GetAllOrgRepoIds(wl context.Context, orgAssetId string) ([]uint64, error) {
	return m.getAllOrgRepoIds(orgAssetId)
}

type mockPermissionsService struct {
	getUsersWithPermission     func(assetId string, p permissions.Permission) (iterator.I[int64], error)
	revokePermissionIfExists   func(userId int64, p permissions.Permission, assetId string) error
	countUsersWithPermission   func(assetId string, p permissions.Permission) (int64, error)
	grantPermissionIfNotExists func(userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error)
}

func (m mockPermissionsService) GetUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error) {
	return m.getUsersWithPermission(assetId, p)
}
func (m mockPermissionsService) RevokePermissionIfExists(wl context.Context, userId int64, p permissions.Permission, assetId string) error {
	return m.revokePermissionIfExists(userId, p, assetId)
}
func (m mockPermissionsService) CountUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (int64, error) {
	return m.countUsersWithPermission(assetId, p)
}
func (m mockPermissionsService) GrantPermissionIfNotExists(wl context.Context, userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error) {
	return m.grantPermissionIfNotExists(userId, p, assetId)
}
