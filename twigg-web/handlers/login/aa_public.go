package login

import (
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/session"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
)

// Add handlers and return 2 wrap functions
// logInWrap use it to wrap handler function so that
// if user not authenticated it redirects to the login page.
// logInAndIsPayingWrap same as logInWrap but If user
// is not paying, redirects to the plans page.
func AddHandlers(
	allowPasswordLogin bool,
	rt routes.Router,
	db webdb.WebDb,
	userService userservice.Service,
	sessionService session.Service,
	mux wrappers.RlMux) {

	h := handler{
		allowPasswordLogin: allowPasswordLogin,
		db:                 db,
		userService:        userService,
		sessionService:     sessionService,
	}

	mux.HandleFunc("GET "+routes.LoginPage, h.handleGet)
	mux.HandleFunc("POST "+routes.LoginPage, h.handlePost)
	mux.HandleFunc("POST "+routes.Logout, h.HandleLogout)

	// Google Oauth
	mux.HandleFunc("GET "+routes.StartLoginWithGoogleOAuth,
		h.sessionService.StartGoogleOAuthSession)
	mux.HandleFunc("GET "+routes.CallbackLoginWithGoogleOAuth,
		h.handleGoogleOauthCallback)

	// Microsoft Oauth
	mux.HandleFunc("GET "+routes.StartLoginWithMicrosoftOAuth,
		h.sessionService.StartMicrosoftOAuthSession)
	mux.HandleFunc("GET "+routes.CallbackLoginWithMicrosoftOAuth,
		h.handleMicrosoftOAuthCallback)

}