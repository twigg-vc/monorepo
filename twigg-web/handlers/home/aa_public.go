package home

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
)

// The storage the home page needs.
type Db interface {
	GetUserAssetIdsWithPermission(ctx context.Context,
		userId int64, p ...permissions.Permission) (iterator.I[string], error)
}

// Registers the handlers required for the home page.
// All handlers will be wrapped with the provided `wrap` if its not nil.
func AddHandlers(
	db Db,
	rt routes.Router,
	rSrv repo.Service,
	uSrv userservice.Service,
	userWithSubMux wrappers.UserWithSubMux) {
	h := newHandler(db, rSrv, rt, uSrv)

	userWithSubMux.HandleFuncR("GET "+routes.Home, h.handleGet)
}
