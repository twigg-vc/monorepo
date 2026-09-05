package landing

import (
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

func AddHandlers(
	rt routes.Router,
	mux wrappers.RlMux,
	wrap func(http.HandlerFunc) http.HandlerFunc) {
	if wrap == nil {
		wrap = func(r http.HandlerFunc) http.HandlerFunc {
			return r
		}
	}
	hl := newHandler()
	mux.HandleFunc("GET "+routes.LandingPage, wrap(hl.ServeHTTP))
}
