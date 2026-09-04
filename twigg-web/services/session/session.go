package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"monorepo/base/cache"
	"monorepo/twigg-web/services/oauthclient"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Represents a user authentication session
type Session struct {
	// Id of the user authenticated
	// -1 for oauth sessions
	UserId int64
	// Name of the user authenticated
	// "" for oauth sessions
	Username string
	// Time when the session expires
	Expiration time.Time
	// Random string used as an anti-csrf value
	// Empty string when not used
	Csrf string
}

const maxEntries = 10_000
const sessionCookieName = "session"

type serviceConfig struct {
	secureCookies              bool
	enableStrongCsrfProtection bool
	maxSessionCookieAge        time.Duration
	size                       int
}

type service struct {
	sessionStore cache.LRU[string, Session]
	mu           sync.RWMutex
	config       serviceConfig

	googleOAuthClient    oauthclient.Google
	microsoftOAuthClient oauthclient.Microsoft
}

func newService(maxSessionCookieAge time.Duration,
	insecureCookies bool,
	googleOAuthClient oauthclient.Google,
	microsoftOAuthClient oauthclient.Microsoft,
	options ...ServiceOption) Service {

	config := serviceConfig{
		secureCookies:              !insecureCookies,
		size:                       maxEntries,
		maxSessionCookieAge:        maxSessionCookieAge,
		enableStrongCsrfProtection: false,
	}
	for _, opt := range options {
		opt(&config)
	}
	return &service{
		sessionStore:         cache.New[string, Session](config.size),
		googleOAuthClient:    googleOAuthClient,
		microsoftOAuthClient: microsoftOAuthClient,
		config:               config,
	}
}

func (s *service) CreateSessionAndWriteCookie(userId int64, username string, w http.ResponseWriter) error {
	// Create a session and save it to storage
	sessionId := getRandomString()
	var csrf string
	if s.config.enableStrongCsrfProtection {
		csrf = getRandomString()
	}
	s.mu.Lock()
	s.sessionStore.Put(sessionId, Session{
		UserId:     userId,
		Username:   username,
		Expiration: time.Now().Add(s.config.maxSessionCookieAge),
		Csrf:       csrf,
	})
	s.mu.Unlock()
	// Write a cookie to the response
	httpCookie := http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionId,
		Path:     "/",
		MaxAge:   int(s.config.maxSessionCookieAge.Seconds()),
		HttpOnly: true,
		Secure:   s.config.secureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &httpCookie)
	if s.config.enableStrongCsrfProtection {
		httpCookie = http.Cookie{
			Name:     CsrfCookieName,
			Value:    csrf,
			Path:     "/",
			MaxAge:   int(s.config.maxSessionCookieAge.Seconds()),
			HttpOnly: false,
			Secure:   s.config.secureCookies,
			SameSite: http.SameSiteStrictMode,
		}
		http.SetCookie(w, &httpCookie)
	}
	return nil
}

func (s *service) DisableSecureCookies(t *testing.T) {
	s.config.secureCookies = false
}

func (s *service) ReadSessionCookie(r *http.Request) (userId int64, username string, notOkNoCsrf, ok bool) {
	// Read the cookie from the request
	httpCookie, err := r.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		ok = false
		return
	}
	if err != nil {
		panic(fmt.Sprintf("got unexpected err from r.Cookie: %s", err))
	}

	// Read the session from the storage
	s.mu.RLock()
	c, found := s.sessionStore.Get(httpCookie.Value)
	s.mu.RUnlock()
	if !found {
		ok = false
		return
	}

	// Check if the session is expired
	if c.Expiration.Before(time.Now()) {
		// If so, delete it from storage
		s.mu.Lock()
		cToExpire, exist := s.sessionStore.Get(httpCookie.Value)
		if exist && cToExpire.Expiration.Before(time.Now()) {
			s.sessionStore.Remove(httpCookie.Value)
		}
		s.mu.Unlock()
		ok = false
		return
	}

	if s.config.enableStrongCsrfProtection && r.Method != "GET" {
		headerMatches := subtle.ConstantTimeCompare(
			[]byte(r.Header.Get(CsrfHeaderName)), []byte(c.Csrf)) == 1
		formMatches := subtle.ConstantTimeCompare(
			[]byte(r.FormValue(CsrfFormName)), []byte(c.Csrf)) == 1
		if !headerMatches && !formMatches {
			notOkNoCsrf = true
			ok = false
			return
		}
	}

	// If the session was found and is not expired, return it
	ok = true
	userId = c.UserId
	username = c.Username
	return
}

func (s *service) DeleteSession(r *http.Request) {
	httpCookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.sessionStore.Remove(httpCookie.Value)
	s.mu.Unlock()
}

const oauthCookieName string = "oauthstate"
const stateCookieTTL = 5 * time.Minute

func (s *service) StartGoogleOAuthSession(w http.ResponseWriter, r *http.Request) {
	state := getRandomString()
	s.createOauthSessionAndSetCookie(state, w)

	// Get the url that we must tell the user to go-to
	url := s.googleOAuthClient.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}
func (s *service) StartMicrosoftOAuthSession(w http.ResponseWriter, r *http.Request) {
	state := getRandomString()
	s.createOauthSessionAndSetCookie(state, w)

	// Get the url that we must tell the user to go-to
	url := s.microsoftOAuthClient.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *service) createOauthSessionAndSetCookie(state string, w http.ResponseWriter) {
	// Crate an oauth session and save it to storage
	expiry := time.Now().Add(stateCookieTTL)
	s.mu.Lock()
	s.sessionStore.Put(state, Session{
		UserId:     -1,
		Username:   "",
		Csrf:       state,
		Expiration: expiry,
	})
	s.mu.Unlock()
	// Write a cookie to the response
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookieName,
		Value:    state,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		Secure:   s.config.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *service) HandleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) (oauthclient.UserInfo, bool) {
	return s.handleOAuthCallback(
		w,
		r,
		s.googleOAuthClient.GetUserInfo,
	)
}
func (s *service) HandleMicrosoftOAuthCallback(w http.ResponseWriter, r *http.Request) (oauthclient.UserInfo, bool) {
	return s.handleOAuthCallback(
		w,
		r,
		s.microsoftOAuthClient.GetUserInfo,
	)
}

func (s *service) handleOAuthCallback(
	w http.ResponseWriter,
	r *http.Request,
	getUserInfo func(*http.Request, string) (oauthclient.UserInfo, error),
) (oauthclient.UserInfo, bool) {
	// Start by checking the cookie which was sent to the user before
	// redirecting them to the authentication URL
	cookie, err := r.Cookie(oauthCookieName)
	if err != nil {
		http.Error(w, "missing oauth cookie", http.StatusUnauthorized)
		return oauthclient.UserInfo{}, false
	}
	s.mu.Lock()
	oauthSession, found := s.sessionStore.Get(cookie.Value)
	s.mu.Unlock()
	if !found {
		http.Error(w, "session not found", http.StatusUnauthorized)
		return oauthclient.UserInfo{}, false
	}
	if oauthSession.Expiration.Before(time.Now()) {
		http.Error(w, "session expired", http.StatusUnauthorized)
		return oauthclient.UserInfo{}, false
	}
	if r.URL.Query().Get("state") != oauthSession.Csrf {
		http.Error(w, "wrong csrf", http.StatusUnauthorized)
		return oauthclient.UserInfo{}, false
	}
	// The state has been validated, so it must not be usable again.
	s.mu.Lock()
	s.sessionStore.Remove(cookie.Value)
	s.mu.Unlock()
	code := r.URL.Query().Get("code")

	// Now get the code and use it to get the user info
	userInfo, err := getUserInfo(r, code)
	if err != nil {
		log.Printf("failed to get OAuth user info: %s", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return oauthclient.UserInfo{}, false
	}

	return userInfo, true
}

type fake struct {
	userId   int64
	username string
}

func newFakeService(uId int64, username string) fake {
	return fake{uId, username}
}

func (f fake) CreateSessionAndWriteCookie(id int64, username string, w http.ResponseWriter) error {
	return nil
}
func (f fake) ReadSessionCookie(r *http.Request) (int64, string, bool, bool) {
	return f.userId, f.username, false, true
}
func (f fake) DeleteSession(r *http.Request) {}

func (f fake) StartGoogleOAuthSession(w http.ResponseWriter, r *http.Request) {}

func (f fake) StartMicrosoftOAuthSession(w http.ResponseWriter, r *http.Request) {}

func (s fake) HandleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) (oauthclient.UserInfo, bool) {
	return oauthclient.UserInfo{}, true
}
func (s fake) HandleMicrosoftOAuthCallback(w http.ResponseWriter, r *http.Request) (oauthclient.UserInfo, bool) {
	return oauthclient.UserInfo{}, true
}
func (s fake) DisableSecureCookies(t *testing.T) {}

// generate a secure random state
func getRandomString() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}