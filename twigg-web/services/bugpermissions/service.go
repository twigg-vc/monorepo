package bugpermissions

import (
	"context"
	"fmt"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/user"
)

type service struct {
	db Db
}

func (s service) canCreateBugs(r context.Context, u *user.User, rp repo.Repo) (bool, error) {
	return s.canWriteRepo(r, u, rp)
}

func (s service) canWriteBug(r context.Context, u *user.User, rp repo.Repo, b bug.Bug) (bool, error) {
	return s.canWriteRepo(r, u, rp)
}

func (s service) canWriteRepo(r context.Context, u *user.User, rp repo.Repo) (bool, error) {
	if u == nil {
		return false, nil
	}
	if u.Id == rp.OwnerId {
		return true, nil
	}
	canWrite, err := s.db.HasPermission(r, u.Id, permissions.Permission_WriteRepo,
		permissions.RepoAssetId(rp.Id))
	if err != nil {
		return false, fmt.Errorf("failed checking write permission (userId=%v repoId=%v): %w", u.Id, rp.Id, err)
	}
	return canWrite, nil
}
