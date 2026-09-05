package orghelper

import (
	"context"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
)

type helper struct {
	permSrv  PermSrv
	reposSrv OrgReposSrv
}

func newHelper(pSrv PermSrv, reposSrv OrgReposSrv) helper {
	return helper{pSrv, reposSrv}
}

func (h helper) EnforceOrgSeatLimit(wl context.Context, orgAssetId string, numberOfSeats int64) error {
	if numberOfSeats < 1 {
		return fmt.Errorf("numberOfSeats can not be < 1")
	}
	nMembers, err := h.permSrv.CountUsersWithPermission(wl, orgAssetId, permissions.Permission_OrganizationMember)
	if err != nil {
		return fmt.Errorf("failed to count members with permission in orgAssetId:%s: err:%s", orgAssetId, err)
	}
	nOwners, err := h.permSrv.CountUsersWithPermission(wl, orgAssetId, permissions.Permission_OrganizationOwner)
	if err != nil {
		return fmt.Errorf("failed to count owners with permission in orgAssetId:%s: err:%s", orgAssetId, err)
	}

	totalSeatsInUse := nMembers + nOwners
	if totalSeatsInUse <= numberOfSeats {
		return nil
	}
	toRemove := totalSeatsInUse - numberOfSeats

	orgRepoIds, err := h.reposSrv.GetAllOrgRepoIds(wl, orgAssetId)
	if err != nil {
		return err
	}

	// Remove members
	membersIt, err := h.permSrv.GetUsersWithPermission(wl, orgAssetId, permissions.Permission_OrganizationMember)
	if err != nil {
		return fmt.Errorf("failed to iterate in members: %s", err)
	}
	for membersIt.Next() {
		if toRemove == 0 {
			break
		}
		memberId, err := membersIt.Get()
		if err != nil {
			return err
		}
		err = h.revokeOrgPermAndReposReadAndWritePerm(
			wl,
			memberId,
			orgAssetId,
			permissions.Permission_OrganizationMember,
			orgRepoIds,
		)
		if err != nil {
			return err
		}
		toRemove--
	}
	err = membersIt.Err()
	if err != nil {
		return err
	}

	if toRemove == 0 {
		return nil
	}

	// Remove owners, until toRemove == 0 or there is only one owner remaining
	ownersIt, err := h.permSrv.GetUsersWithPermission(wl, orgAssetId, permissions.Permission_OrganizationOwner)
	if err != nil {
		return fmt.Errorf("failed to iterate in owners: %s", err)
	}

	var i int64 = 0
	for ownersIt.Next() {
		remainingOwners := nOwners - i
		if toRemove == 0 || remainingOwners <= 1 {
			break
		}
		ownerIt, err := ownersIt.Get()
		if err != nil {
			return err
		}
		err = h.revokeOrgPermAndReposReadAndWritePerm(
			wl,
			ownerIt,
			orgAssetId,
			permissions.Permission_OrganizationOwner,
			orgRepoIds,
		)
		if err != nil {
			return err
		}
		toRemove--
	}
	return nil
}

func (h helper) revokeOrgPermAndReposReadAndWritePerm(wl context.Context, userId int64, orgAssetId string, orgPerm permissions.Permission, orgReposId []uint64) error {
	err := h.permSrv.RevokePermissionIfExists(wl, userId, orgPerm, orgAssetId)
	if err != nil {
		return err
	}
	for _, orgRepoId := range orgReposId {
		repoAssetId := permissions.RepoAssetId(orgRepoId)
		err := h.permSrv.RevokePermissionIfExists(wl, userId, permissions.Permission_WriteRepo, repoAssetId)
		if err != nil {
			return err
		}
		err = h.permSrv.RevokePermissionIfExists(wl, userId, permissions.Permission_ReadRepo, repoAssetId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h helper) grantUserPermissionToOrgIfNotExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error) {
	if p != permissions.Permission_OrganizationOwner &&
		p != permissions.Permission_OrganizationMember {
		alreadyExists = false
		err = errors.New("can only give owner or member permission to org")
		return
	}

	alreadyExists, err = h.permSrv.GrantPermissionIfNotExists(
		wl,
		userId,
		p,
		orgAssetId,
	)
	if err != nil {
		alreadyExists = false
		return
	}
	if alreadyExists {
		return
	}

	orgRepoIds, err := h.reposSrv.GetAllOrgRepoIds(wl, orgAssetId)
	if err != nil {
		return
	}
	for _, orgRepoId := range orgRepoIds {
		_, err = h.permSrv.GrantPermissionIfNotExists(
			wl,
			userId,
			permissions.Permission_WriteRepo,
			permissions.RepoAssetId(orgRepoId),
		)
		if err != nil {
			return
		}
	}
	return
}

func (h helper) revokeUserPermissionFromOrgIfExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (oneOwnerLeftErr bool, err error) {
	if p != permissions.Permission_OrganizationOwner &&
		p != permissions.Permission_OrganizationMember {
		err = fmt.Errorf("can only revoke owner or member permission from org, got:%d", p)
		return
	}

	// To remove Owner we need to mare sure there one more owner
	if p == permissions.Permission_OrganizationOwner {
		var ownersIt iterator.I[int64]
		ownersIt, err = h.permSrv.GetUsersWithPermission(wl, orgAssetId, permissions.Permission_OrganizationOwner)
		if err != nil {
			return
		}

		var hasMoreOwners bool

		for ownersIt.Next() {
			var ownerId int64
			ownerId, err = ownersIt.Get()
			if err != nil {
				err = fmt.Errorf("internal error getting owner")
				return
			}
			if ownerId != userId {
				hasMoreOwners = true
				break
			}
		}
		if err = ownersIt.Err(); err != nil {
			err = fmt.Errorf("gor err: %v in owners iterator", err)
			return
		}

		if !hasMoreOwners {
			oneOwnerLeftErr = true
			err = fmt.Errorf("cant remove the only owner left from with asset id:%s", orgAssetId)
			return
		}
	}

	err = h.permSrv.RevokePermissionIfExists(wl, userId, p, orgAssetId)
	if err != nil {
		err = fmt.Errorf("failed to revoke permission:%d from user id:%d in org asset id:%s", p, userId, orgAssetId)
		return
	}

	// Remove read and write permission from all orgs repos
	orgRepoIds, err := h.reposSrv.GetAllOrgRepoIds(wl, orgAssetId)
	if err != nil {
		return
	}
	for _, orgRepoId := range orgRepoIds {
		err = h.permSrv.RevokePermissionIfExists(
			wl,
			userId,
			permissions.Permission_WriteRepo,
			permissions.RepoAssetId(orgRepoId),
		)
		if err != nil {
			return
		}
		err = h.permSrv.RevokePermissionIfExists(
			wl,
			userId,
			permissions.Permission_ReadRepo,
			permissions.RepoAssetId(orgRepoId),
		)
		if err != nil {
			return
		}
	}
	err = nil
	oneOwnerLeftErr = false
	return
}

func (h helper) orgCanAddOwnerOrMember(u user.User, numberOfOwners int64, numberOfMembers int64) (bool, error) {
	if !u.IsOrganization {
		return false, errors.New("user must be an organization")
	}
	if !u.HasSub() {
		return false, nil
	}
	totalNumberOfContributors := numberOfMembers + numberOfOwners
	if u.SelfPaidSubscription == user.Subscription_Trial {
		return totalNumberOfContributors < userservice.MaxUsersInTrialPlan, nil
	}
	if u.SelfPaidSubscription == user.Subscription_Solo {
		return totalNumberOfContributors < userservice.MaxUsersInSoloPlan, nil
	}
	return totalNumberOfContributors < u.SelfPaidSubscriptionQuantity, nil
}