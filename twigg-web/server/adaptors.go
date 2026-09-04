package server

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	reposervice "monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/user"
	"time"
)

// Simple adaptor to add some methods with different signatures
type RepoServiceAdaptor struct {
	reposervice.Service
}

func (r RepoServiceAdaptor) GetRepoOwnerId(rl context.Context, repoId uint64) (int64, error) {
	rp, err := r.Service.GetById(rl, repoId)
	return rp.OwnerId, err
}

// Simple adaptor for user service
type UserServiceAdaptor struct {
	user.Service
}

func (u UserServiceAdaptor) GetMaxAllowedTimeout(repoOwnerId int64, repoId uint64, tx context.Context) (time.Duration, error) {
	usr, _, err := u.Service.Get(tx, repoOwnerId)
	if err != nil {
		return 0, err
	}
	switch usr.SelfPaidSubscription {
	case user.Subscription_Trial:
		return user.TrialMaxJobTimeout, nil
	case user.Subscription_Solo:
		return user.SoloMaxJobTimeout, nil
	case user.Subscription_Team:
		return user.TeamMaxJobTimeout, nil
	default:
		return 0, nil
	}
}

type userCanCreateRepoAdaptor struct {
	repoService reposervice.Service
	userService user.Service
}

func (a userCanCreateRepoAdaptor) CanCreateRepo(u user.User, r context.Context) (bool, error) {
	userHasTwoOrMoreNonArchivedRepos, err := a.repoService.NonArchivedRepoCountIsGreaterThan(
		r, u.Id, 1)
	if err != nil {
		return false, err
	}
	return u.CanCreateRepo(userHasTwoOrMoreNonArchivedRepos), nil
}

type getAllOrgRepoIdsAdaptor struct {
	repoService reposervice.Service
}

func (a getAllOrgRepoIdsAdaptor) GetAllOrgRepoIds(wl context.Context, orgAssetId string) ([]uint64, error) {
	ownerId := permissions.ParseOrganizationAssetIdOrDie(orgAssetId)
	it, err := a.repoService.GetAllByOwnerId(wl, ownerId)
	if err != nil {
		return nil, err
	}
	// TODO: Check if ids is bigger then max number of repos alloed
	return iterator.GetFirstNWithMapFunc(10_000, it, func(r repo.Repo) (uint64, error) {
		return r.Id, nil
	})
}