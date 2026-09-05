package orghelper

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/user"
)

type Helper struct {
	h helper
}

func NewHelper(pSrv PermSrv, reposSrv OrgReposSrv) Helper {
	return Helper{
		h: newHelper(pSrv, reposSrv),
	}
}

// Trims the org down to numberOfSeats (owners + members combined).
// Members are evicted first; owners only after all members are gone.
// The last owner is never removed, so the effective minimum is 1 seat.
// Also revokes Permission_WriteRepo and Permission_ReadRepo on all org repos for every evicted user.
// No-op when the org is already within the limit.
func (h Helper) EnforceOrgSeatLimit(wl context.Context, orgAssetId string, numberOfSeats int64) error {
	return h.h.EnforceOrgSeatLimit(wl, orgAssetId, numberOfSeats)
}

func (h Helper) GrantUserPermissionToOrgIfNotExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error) {
	return h.h.grantUserPermissionToOrgIfNotExist(wl, userId, orgAssetId, p)
}

func (h Helper) RevokeUserPermissionFromOrgIfExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error) {
	return h.h.revokeUserPermissionFromOrgIfExist(wl, userId, orgAssetId, p)
}

func (h Helper) OrgCanAddOwnerOrMember(u user.User, numberOfOwners int64, numberOfMembers int64) (bool, error) {
	return h.h.orgCanAddOwnerOrMember(u, numberOfOwners, numberOfMembers)
}

type PermSrv interface {
	GetUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error)
	RevokePermissionIfExists(wl context.Context, userId int64, p permissions.Permission, assetId string) error
	CountUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (int64, error)
	GrantPermissionIfNotExists(wl context.Context, userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error)
}

type OrgReposSrv interface {
	GetAllOrgRepoIds(wl context.Context, orgAssetId string) ([]uint64, error)
}