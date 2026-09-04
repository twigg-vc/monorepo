package webdb_test

import (
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/webdb"
	"reflect"
	"slices"
	"testing"
)

func TestHasPermission(t *testing.T) {
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

	// Check permission not granted
	haveP, err := b.HasPermission(w, 10, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}

	// Grant permission
	_, err = b.GrantPermissionIfNotExists(w, 10, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}

	// Check permission granted
	haveP, err = b.HasPermission(w, 10, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if !haveP {
		t.Fatal("should have permission granted")
	}
	// Check if permission granted just for permissions.Permission_WriteRepo
	haveP, err = b.HasPermission(w, 10, permissions.Permission_ReadRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}
}

func TestGrantPermissionTwice(t *testing.T) {
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

	expFalse, err := b.GrantPermissionIfNotExists(w, 10, 88, "fdzgfzdgzfhxg")
	if err != nil {
		t.Fatal(err)
	}
	if expFalse {
		t.Fatal("shouldn't have permission granted")
	}

	expTrue, err := b.GrantPermissionIfNotExists(w, 10, 88, "fdzgfzdgzfhxg")
	if err != nil {
		t.Fatal(err)
	}
	if !expTrue {
		t.Fatal("should have permission granted")
	}

}
func TestHasPermissionForDifferentRepo(t *testing.T) {
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

	// Grant permission
	_, err = b.GrantPermissionIfNotExists(w, 10, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}

	// Check permission granted
	haveP, err := b.HasPermission(w, 10, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if !haveP {
		t.Fatal("should have permission granted")
	}

	// Check if permission granted just for Repo of id 99
	haveP, err = b.HasPermission(w, 10, permissions.Permission_WriteRepo, permissions.RepoAssetId(80))
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}
}
func TestRevokePermissionIfExists(t *testing.T) {
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

	// Grant permission
	_, err = b.GrantPermissionIfNotExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	// Revoke permission
	err = b.RevokePermissionIfExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	// Check if it was updated
	haveP, err := b.HasPermission(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}
}
func TestRevokePermissionIfExistsMultiple(t *testing.T) {
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

	// Grant permission
	_, err = b.GrantPermissionIfNotExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	// Revoke permission
	err = b.RevokePermissionIfExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	err = b.RevokePermissionIfExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	err = b.RevokePermissionIfExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
}
func TestRevokeJustOnePermission(t *testing.T) {
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

	// Grant Read and Write permissions
	_, err = b.GrantPermissionIfNotExists(w, 100, permissions.Permission_ReadRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GrantPermissionIfNotExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}

	// Revoke Write permission
	err = b.RevokePermissionIfExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}

	// Check still has Read and Write was Revoke
	haveP, err := b.HasPermission(w, 100, permissions.Permission_ReadRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if !haveP {
		t.Fatal("should have permission granted")
	}
	haveP, err = b.HasPermission(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}
}

func TestRevokeAllPermissionsToAsset(t *testing.T) {
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

	b.GrantPermissionIfNotExists(w, 100, permissions.Permission_ReadRepo, "asset1")
	b.GrantPermissionIfNotExists(w, 101, permissions.Permission_ReadRepo, "asset1")
	b.GrantPermissionIfNotExists(w, 102, permissions.Permission_WriteRepo, "asset1")
	b.GrantPermissionIfNotExists(w, 103, permissions.Permission_WriteRepo, "asset2")
	err = b.RevokeAllPermissionsToAsset(w, "asset1")
	if err != nil {
		t.Fatal(err)
	}
	it, err := b.GetUsersWithPermission(w, "asset1", permissions.Permission_ReadRepo)
	if err != nil {
		t.Fatal(err)
	}
	if it.Next() {
		t.Fatal("got non nil iter")
	}
	it, err = b.GetUsersWithPermission(w, "asset1", permissions.Permission_WriteRepo)
	if err != nil {
		t.Fatal(err)
	}
	if it.Next() {
		t.Fatal("got non nil iter")
	}

	it, err = b.GetUsersWithPermission(w, "asset2", permissions.Permission_WriteRepo)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Next() {
		t.Fatal("got nil iter for non deleter asset")
	}
}
func TestGetUserPermissions(t *testing.T) {
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

	// Grant Read and Write permissions
	_, err = b.GrantPermissionIfNotExists(w, 100, permissions.Permission_ReadRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GrantPermissionIfNotExists(w, 100, permissions.Permission_WriteRepo, permissions.RepoAssetId(99))
	if err != nil {
		t.Fatal(err)
	}

	it, err := b.GetUserPermissions(w, 100)
	if err != nil {
		t.Fatal(err)
	}

	haveReadRepoPerm := false
	haveWriteRepoPerm := false
	i := 0
	for it.Next() {
		p, err := it.Get()
		if err != nil {
			t.Fatal(err)
		}
		if p == permissions.Permission_ReadRepo {
			haveReadRepoPerm = true
		}
		if p == permissions.Permission_WriteRepo {
			haveWriteRepoPerm = true
		}
		i++
	}
	if !haveReadRepoPerm || !haveWriteRepoPerm || i != 2 {
		t.Fatal("iteration did not go as plan")
	}
}

func TestGetUsersWithPermission(t *testing.T) {
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

	const userId0 = 99
	const permission0 = 55
	_, err = b.GrantPermissionIfNotExists(w, userId0, permission0, "my-asset-id")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GrantPermissionIfNotExists(w, 999999999999, 99999999999, "9999999999")
	if err != nil {
		t.Fatal(err)
	}

	it, err := b.GetUsersWithPermission(w, "my-asset-id", permission0)
	if err != nil {
		t.Fatal(err)
	}

	if !it.Next() {
		t.Fatal("expected iterator not to be done")
	}
	userid, err := it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if userid != userId0 {
		t.Fatal("iteration did not go as plan")
	}
	if it.Next() {
		t.Fatal("expected iterator to be done")
	}
	err = it.Err()
	if err != nil {
		t.Fatal(err)
	}

	it, err = b.GetUsersWithPermission(w, "non-existing-asset", permission0)
	if err != nil {
		t.Fatal(err)
	}
	if it.Next() {
		t.Fatal("should not exist")
	}
}

func TestGetUserAssetIdsWithPermission(t *testing.T) {
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

	// Grant some dummy permissions for a specific user and some random ones
	const userId = 800
	b.GrantPermissionIfNotExists(w, userId, 100, "asset-1")
	b.GrantPermissionIfNotExists(w, userId, 101, "asset-1")
	b.GrantPermissionIfNotExists(w, userId, 102, "asset-1")
	b.GrantPermissionIfNotExists(w, userId, 100, "asset-2")
	b.GrantPermissionIfNotExists(w, userId, 101, "asset-2")
	b.GrantPermissionIfNotExists(w, userId, 99, "asset-3")
	b.GrantPermissionIfNotExists(w, userId, 105, "asset-3")
	// Other user who has 100/101 permissions to same and other assets
	b.GrantPermissionIfNotExists(w, 1, 100, "asset-1")
	b.GrantPermissionIfNotExists(w, 1, 101, "asset-1")
	b.GrantPermissionIfNotExists(w, 1, 100, "asset-2")
	b.GrantPermissionIfNotExists(w, 1, 101, "asset-2")
	b.GrantPermissionIfNotExists(w, 1, 100, "asset-3")
	b.GrantPermissionIfNotExists(w, 1, 101, "asset-3")
	// Other user with other permissions to same assets
	b.GrantPermissionIfNotExists(w, 2, 77, "asset-1")
	b.GrantPermissionIfNotExists(w, 2, 78, "asset-2")

	// Read all assets for which the user has permissions 100 or 101
	assetsList := []string{}
	assets, err := b.GetUserAssetIdsWithPermission(w, userId, 100, 101)
	if err != nil {
		t.Fatal(err)
	}
	for assets.Next() {
		assetId, err := assets.Get()
		if err != nil {
			t.Fatal(err)
		}
		assetsList = append(assetsList, assetId)
	}
	err = assets.Err()
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(assetsList)
	if !reflect.DeepEqual(assetsList, []string{"asset-1", "asset-2"}) {
		t.Fatalf("unexpected assetsList: %v", assetsList)
	}

	// Test getting an empty iter
	assets, err = b.GetUserAssetIdsWithPermission(w, userId, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if assets.Next() {
		t.Fatal("expected empty assets list")
	}
	err = assets.Err()
	if err != nil {
		t.Fatal(err)
	}
}
func TestHasOrganizationOwnerPermission(t *testing.T) {
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

	var userId int64 = 20
	assetId := permissions.OrganizationAssetId(99)

	// Check permission not granted
	haveP, err := b.HasPermission(w, userId, permissions.Permission_OrganizationOwner, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}

	// Grant permission
	_, err = b.GrantPermissionIfNotExists(w, userId, permissions.Permission_OrganizationOwner, assetId)
	if err != nil {
		t.Fatal(err)
	}

	// Check permission granted
	haveP, err = b.HasPermission(w, userId, permissions.Permission_OrganizationOwner, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if !haveP {
		t.Fatal("should have permission granted")
	}

	// Check member permission was not granted
	haveP, err = b.HasPermission(w, userId, permissions.Permission_OrganizationMember, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}

	// Revoke permission
	err = b.RevokePermissionIfExists(w, userId, permissions.Permission_OrganizationOwner, assetId)
	if err != nil {
		t.Fatal(err)
	}

	// Check permission revoked
	haveP, err = b.HasPermission(w, userId, permissions.Permission_OrganizationOwner, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}
}

func TestHasOrganizationMemberPermission(t *testing.T) {
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

	var userId int64 = 20
	assetId := permissions.OrganizationAssetId(88)

	// Grant permission
	_, err = b.GrantPermissionIfNotExists(w, userId, permissions.Permission_OrganizationMember, assetId)
	if err != nil {
		t.Fatal(err)
	}

	// Check permission granted
	haveP, err := b.HasPermission(w, userId, permissions.Permission_OrganizationMember, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if !haveP {
		t.Fatal("should have permission granted")
	}

	// Check owner permission was not granted
	haveP, err = b.HasPermission(w, userId, permissions.Permission_OrganizationOwner, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}

	// Revoke permission
	err = b.RevokePermissionIfExists(w, userId, permissions.Permission_OrganizationMember, assetId)
	if err != nil {
		t.Fatal(err)
	}

	// Check permission revoked
	haveP, err = b.HasPermission(w, userId, permissions.Permission_OrganizationMember, assetId)
	if err != nil {
		t.Fatal(err)
	}
	if haveP {
		t.Fatal("should not have permission granted")
	}
}
func TestOrganizationAssetId(t *testing.T) {
	assetId := permissions.OrganizationAssetId(1)
	if assetId != "org-1" {
		t.Fatalf("unexpected asset id, got: %q", assetId)
	}

	parsedAssetId := permissions.ParseOrganizationAssetIdOrDie(assetId)
	if parsedAssetId != 1 {
		t.Fatalf("unexpected parsed asset id, got: %d", parsedAssetId)
	}
}

func TestCountUserAssetsWithPermission(t *testing.T) {
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

	const userId = 800

	// Grant 2 permissions
	_, err = b.GrantPermissionIfNotExists(w, userId, permissions.Permission_OrganizationMember, "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.GrantPermissionIfNotExists(w, userId, permissions.Permission_OrganizationMember, "asset-2")
	if err != nil {
		t.Fatal(err)
	}
	// Different user
	_, err = b.GrantPermissionIfNotExists(w, 99999, permissions.Permission_OrganizationMember, "asset-1")
	if err != nil {
		t.Fatal(err)
	}

	// Expects 2 permissions
	count, err := b.CountUserAssetsWithPermission(w, userId, permissions.Permission_OrganizationMember)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("unexpected count: %d", count)
	}

	// Revoke one, expects only 1
	err = b.RevokePermissionIfExists(w, userId, permissions.Permission_OrganizationMember, "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	count, err = b.CountUserAssetsWithPermission(w, userId, permissions.Permission_OrganizationMember)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unexpected count after revoke: %d", count)
	}

	// Test empty result
	count, err = b.CountUserAssetsWithPermission(w, userId, 99999)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 count, got: %d", count)
	}
}

func TestCountUsersWithPermission(t *testing.T) {
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

	orgAssetId := permissions.OrganizationAssetId(12)

	// Grant 2 owners and 1 member to org 12
	b.GrantPermissionIfNotExists(w, 1, permissions.Permission_OrganizationOwner, orgAssetId)
	b.GrantPermissionIfNotExists(w, 2, permissions.Permission_OrganizationOwner, orgAssetId)
	b.GrantPermissionIfNotExists(w, 3, permissions.Permission_OrganizationMember, orgAssetId)
	// User in a different org, must not affect org 12 counts
	b.GrantPermissionIfNotExists(w, 4, permissions.Permission_OrganizationOwner, permissions.OrganizationAssetId(99))

	count, err := b.CountUsersWithPermission(w, orgAssetId, permissions.Permission_OrganizationOwner)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 owners, got %d", count)
	}

	count, err = b.CountUsersWithPermission(w, orgAssetId, permissions.Permission_OrganizationMember)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 member, got %d", count)
	}

	// Revoke one owner and recount
	b.RevokePermissionIfExists(w, 1, permissions.Permission_OrganizationOwner, orgAssetId)
	count, err = b.CountUsersWithPermission(w, orgAssetId, permissions.Permission_OrganizationOwner)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 owner after revoke, got %d", count)
	}

	// Non-existent permission on asset returns 0
	count, err = b.CountUsersWithPermission(w, orgAssetId, permissions.Permission_WriteRepo)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestGrantPermission_MaxOwnersEnforced(t *testing.T) {
	var maxNumberOfOwnersInOrgBackup = permissions.MaxNumberOfOwnersInOrg
	defer func() { permissions.MaxNumberOfOwnersInOrg = maxNumberOfOwnersInOrgBackup }()
	permissions.MaxNumberOfOwnersInOrg = 10

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

	orgAssetId := permissions.OrganizationAssetId(10)
	for i := range permissions.MaxNumberOfOwnersInOrg {
		_, err := b.GrantPermissionIfNotExists(w, int64(i), permissions.Permission_OrganizationOwner, orgAssetId)
		if err != nil {
			t.Fatalf("unexpected error at owner %d: %s", i, err)
		}
	}

	_, err = b.GrantPermissionIfNotExists(w, int64(permissions.MaxNumberOfOwnersInOrg), permissions.Permission_OrganizationOwner, orgAssetId)
	if err == nil {
		t.Fatal("expected error when exceeding max owners")
	}
}

func TestGrantPermission_MaxMembersEnforced(t *testing.T) {
	var maxNumberOfMembersInOrgBackup = permissions.MaxNumberOfMembersInOrg
	defer func() { permissions.MaxNumberOfMembersInOrg = maxNumberOfMembersInOrgBackup }()
	permissions.MaxNumberOfMembersInOrg = 10

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

	orgAssetId := permissions.OrganizationAssetId(11)
	for i := range permissions.MaxNumberOfMembersInOrg {
		_, err := b.GrantPermissionIfNotExists(w, int64(i), permissions.Permission_OrganizationMember, orgAssetId)
		if err != nil {
			t.Fatalf("unexpected error at member %d: %s", i, err)
		}
	}

	_, err = b.GrantPermissionIfNotExists(w, int64(permissions.MaxNumberOfMembersInOrg), permissions.Permission_OrganizationMember, orgAssetId)
	if err == nil {
		t.Fatal("expected error when exceeding max members")
	}
}
