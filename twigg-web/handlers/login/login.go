package login

import (
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/oauthclient"
	"monorepo/twigg-web/services/session"
	userservice "monorepo/twigg-web/services/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"net/http"
)

type handler struct {
	allowPasswordLogin bool
	db                 Db
	userService        userservice.Service
	sessionService     session.Service
}

func (hl handler) handleGet(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := hl.sessionService.ReadSessionCookie(r)
	if ok {
		http.Redirect(w, r, routes.Home, http.StatusSeeOther)
		return
	}
	twiggwc.Page( /*hideNavBar=*/ true, featureflags.GetFlags("", "", ""), twiggwc.Login(false)).Render(w)
}

// If authentication is ok, returns a redirect to the main page.
// Else, returns the login page with an error message.
func (h handler) handlePost(w http.ResponseWriter, r *http.Request) {
	// Redirect to login as if the password were wrong if
	// password login is disabled
	if !h.allowPasswordLogin {
		twiggwc.Page( /*hideNavBar=*/ true,
			featureflags.GetFlags("", "", ""),
			/*wrongLoginInfo=*/ twiggwc.Login(true),
		).Render(w)
		return
	}

	rl, cl, err := h.db.BeginRead()
	defer cl()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// `email` and `password` are the name of the field defined in the
	// webcomponent.
	email := r.FormValue(routes.LogInEmailFieldName)
	plainPassword := r.FormValue(routes.LogInPasswordFieldName)
	u, isNotFoundErr, err := h.userService.GetByEmail(rl, email)
	if !isNotFoundErr && err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if isNotFoundErr || !h.userService.PasswordIsCorrect(u, plainPassword) {
		twiggwc.Page( /*hideNavBar=*/ true,
			featureflags.GetFlags("", "", ""),
			/*wrongLoginInfo=*/ twiggwc.Login(true),
		).Render(w)
		return
	}
	// Else, create a session and write a cookie
	h.sessionService.CreateSessionAndWriteCookie(u.Id, u.Username, w)
	http.Redirect(w, r, routes.Home, http.StatusSeeOther)
}

func (h *handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessionService.DeleteSession(r)
	http.Redirect(w, r, routes.LoginPage, http.StatusSeeOther)
}

func (h *handler) handleGoogleOauthCallback(w http.ResponseWriter, r *http.Request) {
	userInfo, ok := h.sessionService.HandleGoogleOAuthCallback(w, r)
	if !ok {
		return
	}
	h.handleOAuthAuthenticatedUser(userInfo, w, r)

}
func (h *handler) handleMicrosoftOAuthCallback(w http.ResponseWriter, r *http.Request) {
	userInfo, ok := h.sessionService.HandleMicrosoftOAuthCallback(w, r)
	if !ok {
		return
	}
	h.handleOAuthAuthenticatedUser(userInfo, w, r)
}
func (h *handler) handleOAuthAuthenticatedUser(userInfo oauthclient.UserInfo,
	w http.ResponseWriter, r *http.Request) {

	wl, cl, commit, err := h.db.BeginWrite()
	defer cl()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authenticatedUsed, isNotFoundErr, err := h.userService.GetByEmail(wl, userInfo.Email)
	if !isNotFoundErr && err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// If not found, create a user and redirect them to the page to set
	// their username
	if isNotFoundErr {
		authenticatedUsed, err = h.userService.RegisterNewUserFromOAuth(wl, userInfo.Email)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		err = commit()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		h.sessionService.CreateSessionAndWriteCookie(
			authenticatedUsed.Id, authenticatedUsed.Username, w)
		http.Redirect(w, r, routes.SetUsernamePath, http.StatusSeeOther)
		return
	}

	// Else just redirect to home
	h.sessionService.CreateSessionAndWriteCookie(
		authenticatedUsed.Id, authenticatedUsed.Username, w)
	http.Redirect(w, r, routes.Home, http.StatusSeeOther)
}
