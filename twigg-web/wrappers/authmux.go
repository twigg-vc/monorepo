package wrappers

import (
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/session"
	"net/http"
)

type authMux struct {
	sessionService session.Service
	configName     string
	mux            RlMux
}

func (m authMux) HandleFunc(
	pattern string, handler func(w http.ResponseWriter, r AuthMuxRequest)) {
	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		userId, username, notOkDueToNoCsrf, ok := m.sessionService.ReadSessionCookie(r)
		if notOkDueToNoCsrf {
			http.Error(w, "missing csrf", http.StatusForbidden)
			return
		}
		if !ok {
			http.Redirect(w, r, routes.LoginPage, http.StatusSeeOther)
			return
		}
		handler(w, AuthMuxRequest{Request: r, Username: username, UserId: userId,
			Flags: featureflags.GetFlags(m.configName, "", username)})
	})
}
