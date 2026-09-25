package bugpermissions

import (
	"context"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/user"
)

// Decides who can do what with the bugs of a repo.
type Service struct {
	s service
}

func NewService(db Db) Service {
	return Service{s: service{db: db}}
}

// u is nil for anonymous users.
func (s Service) CanCreateBugs(r context.Context, u *user.User, rp repo.Repo) (bool, error) {
	return s.s.canCreateBugs(r, u, rp)
}

// Whether u can comment on, change the status of and edit b. u is nil for
// anonymous users.
func (s Service) CanWriteBug(r context.Context, u *user.User, rp repo.Repo, b bug.Bug) (bool, error) {
	return s.s.canWriteBug(r, u, rp, b)
}

type Db interface {
	HasPermission(r context.Context, userId int64, p permissions.Permission, assetId string) (bool, error)
}
