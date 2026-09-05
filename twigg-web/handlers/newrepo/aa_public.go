package newrepo

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
)

// Registers the handlers required for the newrepo page.
// All handlers will be wrapped with the provided `wrap` if its not nil.
func AddHandlers(
	canCreate CanCreateRepo,
	rSrv RepoService,
	permSrv PermissionsService,
	userWithSubMux wrappers.UserWithSubMux) {

	h := newHandler(canCreate, rSrv, permSrv)

	userWithSubMux.HandleFuncR("GET "+routes.NewRepoUrl, h.handleGet)
	userWithSubMux.HandleFuncW("POST "+routes.NewRepoUrl, h.handlePost)
}

type CanCreateRepo interface {
	CanCreateRepo(u user.User, rl context.Context) (bool, error)
}
type RepoService interface {
	CreateNew(wl context.Context, ownerId int64, displayName string, description string) (r repo.Repo, isAlreadyExistsErr bool, err error)
}
type PermissionsService interface {
	GrantPermissionIfNotExists(wl context.Context, userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error)
	GetUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error)
}