package home

import (
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
)

// Registers the handlers required for the home page.
// All handlers will be wrapped with the provided `wrap` if its not nil.
func AddHandlers(
	db webdb.WebDb,
	rt routes.Router,
	rSrv repo.Service,
	uSrv userservice.Service,
	userWithSubMux wrappers.UserWithSubMux) {
	h := newHandler(db, rSrv, rt, uSrv)

	userWithSubMux.HandleFuncR("GET "+routes.Home, h.handleGet)
}