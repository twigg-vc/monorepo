package session

import (
	"errors"
	"monorepo/twigg-web/services/oauthclient"
	"net/http"
	"testing"
	"time"
)

// If strong CSRF protection is enabled, on authentication, the server
// sends a non-http cookie with a csrf token.
// This token must be included in all non-GET requests as a header or
// a form field.
const CsrfCookieName = "csrf"
const CsrfHeaderName = "X-Twigg-Csrf"
const CsrfFormName = "csrf_token"

type Service interface {
	// Create a session and save it in the service's storage.
	// Then, write a cookie with the session id.
	CreateSessionAndWriteCookie(userId int64, username string, w http.ResponseWriter) error

	// Reads the session id from the request's cookie, then get the session
	// from the service's storage and return it.
	// Returns `ok=false` if the request doesn't contain the cookie, or
	// if the cookie is invalid, or if the session is expired.
	ReadSessionCookie(r *http.Request) (userId int64, username string, notOkDueToNoCsrf, ok bool)

	// Tries reading the session cookie from the request. If found, deletes
	// if from the service's storage.
	DeleteSession(r *http.Request)

	// Create an oauth session and save it in the service's storage.
	// Then, write a cookie with the session id and redirect to Oauth service.
	StartGoogleOAuthSession(w http.ResponseWriter, r *http.Request)
	StartMicrosoftOAuthSession(w http.ResponseWriter, r *http.Request)

	// Reads the oauth session from the request and validates it against the
	// service's storage. Then, delete the session from storage.
	// On any error, writes an error to the response and returns `ok=false`.
	HandleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) (userInfo oauthclient.UserInfo, ok bool)
	HandleMicrosoftOAuthCallback(w http.ResponseWriter, r *http.Request) (userInfo oauthclient.UserInfo, ok bool)

	// Disable secure cookies for testing
	DisableSecureCookies(t *testing.T)
}

func NewService(
	maxSessionCookieAge time.Duration,
	googleOAuthClient oauthclient.Google,
	microsoftOAuthClient oauthclient.Microsoft,
	options ...ServiceOption) Service {
	return newService(maxSessionCookieAge, false, googleOAuthClient,
		microsoftOAuthClient, options...)
}

// Creates service that sets non-secure (non HTTPS only) cookies.
func NewInsecureCookiesService(
	maxSessionCookieAge time.Duration,
	googleOAuthClient oauthclient.Google,
	microsoftOAuthClient oauthclient.Microsoft,
	options ...ServiceOption) Service {
	return newService(maxSessionCookieAge, true, googleOAuthClient,
		microsoftOAuthClient, options...)
}

// Fake service that always returns authenticated.
// It doesn't set/read cookies.
func NewFake(userId int64, username string) Service {
	return newFakeService(userId, username)
}

var ErrNotAuthenticated = errors.New("not authenticated")

type ServiceOption = func(c *serviceConfig)

// Set the number of sessions stored
func WithSize(size int) ServiceOption {
	return func(c *serviceConfig) { c.size = size }
}

// Enables strong CSRF protection
func WithStrongCsrfProtection() ServiceOption {
	return func(c *serviceConfig) { c.enableStrongCsrfProtection = true }
}
