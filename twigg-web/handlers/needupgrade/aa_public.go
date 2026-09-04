package needupgrade

import (
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

func AddHandlers(mux wrappers.AuthMux) {
	mux.HandleFunc("GET "+routes.NeedUpgradePage, handleGetNeedUpgradePage)
}

func handleGetNeedUpgradePage(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	webcomponents.Page( /*hideNavBar*/ true,
		r.Flags,
		webcomponents.NeedUpgradePage(),
	).Render(w)
}
