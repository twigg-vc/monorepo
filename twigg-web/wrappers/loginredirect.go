package wrappers

import (
	"monorepo/twigg-web/routes"
	"net/http"
)

// GET requests are redirected to the login page. Any other method
// gets a plain 401 error.
func redirectToLoginOrUnauthorized(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.Redirect(w, r, routes.LoginPage, http.StatusSeeOther)
		return
	}
	http.Error(w, "not authenticated", http.StatusUnauthorized)
}
