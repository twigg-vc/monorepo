package session

import (
	"fmt"
	"monorepo/twigg-web/services/oauthclient"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestService(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	gAuth := oauthclient.NewGoogleMock()
	msAuth := oauthclient.NewMicrosoftMock()

	s := NewInsecureCookiesService(time.Duration(3*time.Hour), gAuth, msAuth)

	_, _, _, ok := s.ReadSessionCookie(req)
	if ok {
		t.Fatal("should ErrNotAuthenticated")
	}

	err := s.CreateSessionAndWriteCookie(0, "abc", w)
	if err != nil {
		t.Fatal("should not err")
	}
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("cookie not set")
	}

	// The next request from the browser will come with the cookie set
	reqWithCookies := httptest.NewRequest("GET", "/", nil)
	reqWithCookies.AddCookie(w.Result().Cookies()[0])
	id, usrname, _, ok := s.ReadSessionCookie(reqWithCookies)
	if !ok {
		t.Fatal(err)
	}
	if id != 0 {
		t.Fatal("wrong id")
	}
	if usrname != "abc" {
		t.Fatal("wrong username")
	}

	s.DeleteSession(reqWithCookies)
	_, _, _, ok = s.ReadSessionCookie(reqWithCookies)
	if ok {
		t.Fatal("should no longer be authenticated")
	}

}

func TestMaxEntries(t *testing.T) {
	testMaxEntries := 10
	gAuth := oauthclient.NewGoogleMock()
	msAuth := oauthclient.NewMicrosoftMock()

	s := NewInsecureCookiesService(time.Duration(3*time.Hour), gAuth, msAuth,
		WithSize(testMaxEntries))
	w := httptest.NewRecorder()

	// Exceed maxEntries
	for i := 0; i <= testMaxEntries; i++ {
		err := s.CreateSessionAndWriteCookie(int64(i), fmt.Sprintf("%d", i), w)
		if err != nil {
			t.Fatal("should not err")
		}
	}
	cookiesSet := w.Result().Cookies()
	if len(cookiesSet) != testMaxEntries+1 {
		t.Fatal("expected testMaxEntries+1 cookies set")
	}
	// Expect user 0 to evicted
	user0Req := httptest.NewRequest("GET", "/", nil)
	user0Req.AddCookie(cookiesSet[0])
	_, _, _, ok := s.ReadSessionCookie(user0Req)
	if ok {
		t.Fatal("should err because user 0 should've been evicted")
	}

	// Expect user 1 to not be evicted
	user1Req := httptest.NewRequest("GET", "/", nil)
	user1Req.AddCookie(cookiesSet[1])
	id, usrname, _, ok := s.ReadSessionCookie(user1Req)
	if !ok {
		t.Fatal("should not err")
	}
	if id != 1 {
		t.Fatal("wrong id")
	}
	if usrname != "1" {
		t.Fatal("wrong username")
	}
}

func TestMaxAge(t *testing.T) {
	gAuth := oauthclient.NewGoogleMock()
	w := httptest.NewRecorder()
	msAuth := oauthclient.NewMicrosoftMock()
	s := NewInsecureCookiesService(time.Duration(50*time.Millisecond), gAuth, msAuth)

	err := s.CreateSessionAndWriteCookie(0, "", w)
	if err != nil {
		t.Fatal("should not err")
	}
	reqWithCookies := httptest.NewRequest("GET", "/", nil)
	reqWithCookies.AddCookie(w.Result().Cookies()[0])
	id, _, _, ok := s.ReadSessionCookie(reqWithCookies)
	if id != 0 || !ok {
		t.Fatal("should be authenticated")
	}

	time.Sleep(100 * time.Millisecond)

	_, _, _, ok = s.ReadSessionCookie(reqWithCookies)
	if ok {
		t.Fatal("should not be authenticated")
	}
}

func TestFake(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	f := NewFake(69, "fakeuser")

	// The following calls do nothing
	f.CreateSessionAndWriteCookie(9999999, "fgffhn", w)
	f.DeleteSession(req)

	id, usrname, _, ok := f.ReadSessionCookie(req)
	if !ok || id != 69 || usrname != "fakeuser" {
		t.Fatal("should always be authenticated with the fake id")
	}

}

func TestConcurrency(t *testing.T) {
	gAuth := oauthclient.NewGoogleMock()
	msAuth := oauthclient.NewMicrosoftMock()
	s := NewInsecureCookiesService(5*time.Minute, gAuth, msAuth)

	const goroutines int = 50
	const iterations int = 100

	var wg sync.WaitGroup
	var start sync.WaitGroup // for simultaneous start
	start.Add(1)

	// Define the work each goroutine will do
	worker := func(gId int) {
		defer wg.Done()
		start.Wait() // wait until all goroutines are ready

		for i := 0; i < iterations; i++ {
			w := httptest.NewRecorder()

			if err := s.CreateSessionAndWriteCookie(int64(gId), fmt.Sprintf("%d", gId), w); err != nil {
				t.Errorf("SetCookie failed: %v", err)
			}

			// Grab the cookie from the response
			resp := w.Result()
			c := resp.Cookies()[0]

			// Simulate a request carrying the cookie
			req := httptest.NewRequest("GET", "/", nil)
			req.AddCookie(c)

			userID, _, _, ok := s.ReadSessionCookie(req)
			if !ok {
				t.Errorf("ReadSessionCookie failed")
			}
			if userID != int64(gId) {
				t.Errorf(
					"ReadSessionCookie returned wrong userID: got %d, want %d", userID, gId)
			}

			s.DeleteSession(req)
			_, _, _, ok = s.ReadSessionCookie(req)
			if ok {
				t.Errorf("ReadSessionCookie retured ok after session deleted")
			}
		}
	}

	// Launch goroutines
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go worker(g)
	}

	// Trigger all goroutines to start at the same time
	start.Done()

	wg.Wait()
}

func TestCookie(t *testing.T) {
	w := httptest.NewRecorder()
	duration := time.Duration(3 * time.Hour)
	gAuth := oauthclient.NewGoogleMock()
	msAuth := oauthclient.NewMicrosoftMock()
	s := NewService(duration, gAuth, msAuth)

	err := s.CreateSessionAndWriteCookie(0, "", w)
	if err != nil {
		t.Fatal("should not err")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("wrong number of cookies")
	}
	cookie := cookies[0]

	if cookie.Name != sessionCookieName {
		t.Fatal("wrong cookie name")
	}
	if !cookie.HttpOnly {
		t.Fatal("wrong HttpOnly")
	}
	if !cookie.Secure {
		t.Fatal("wrong Secure")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("expected lax mode")
	}
	if cookie.MaxAge != int(duration.Seconds()) {
		t.Fatal("wrong max age")
	}
}

func TestInsecureCookies(t *testing.T) {
	w := httptest.NewRecorder()
	s := NewInsecureCookiesService(
		time.Duration(3*time.Hour),
		oauthclient.NewGoogleMock(),
		oauthclient.NewMicrosoftMock(),
	)

	err := s.CreateSessionAndWriteCookie(0, "", w)
	if err != nil {
		t.Fatal("should not err")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("wrong number of cookies")
	}
	if cookies[0].Secure {
		t.Fatal("expected insecure cookie")
	}
}

func TestCsrfRequiredHeader(t *testing.T) {
	w := httptest.NewRecorder()
	s := NewInsecureCookiesService(
		time.Duration(3*time.Hour),
		oauthclient.NewGoogleMock(),
		oauthclient.NewMicrosoftMock(),
		WithStrongCsrfProtection(),
	)

	s.CreateSessionAndWriteCookie(1, "user1", w)

	getReqWithCookie := httptest.NewRequest("GET", "/", nil)
	getReqWithCookie.AddCookie(w.Result().Cookies()[0])
	_, _, _, ok := s.ReadSessionCookie(getReqWithCookie)
	if !ok {
		t.Fatal("expected ok bc GET requires no strong csrf header")
	}

	postReqWithCookie := httptest.NewRequest("POST", "/", nil)
	postReqWithCookie.AddCookie(w.Result().Cookies()[0])
	_, _, isNotOkBecauseNoCsrf, ok := s.ReadSessionCookie(postReqWithCookie)
	if ok || !isNotOkBecauseNoCsrf {
		t.Fatal("expected not ok bc its missing csrf header")
	}

	postReqWithCookieAndHeader := httptest.NewRequest("POST", "/", nil)
	postReqWithCookieAndHeader.AddCookie(w.Result().Cookies()[0])
	postReqWithCookieAndHeader.Header.Set(CsrfHeaderName,
		w.Result().Cookies()[1].Value)
	_, _, _, ok = s.ReadSessionCookie(postReqWithCookieAndHeader)
	if !ok {
		t.Fatal("expected ok")
	}
}

func TestCsrfRequiredForm(t *testing.T) {
	w := httptest.NewRecorder()
	s := NewInsecureCookiesService(
		time.Duration(3*time.Hour),
		oauthclient.NewGoogleMock(),
		oauthclient.NewMicrosoftMock(),
		WithStrongCsrfProtection(),
	)

	s.CreateSessionAndWriteCookie(1, "user1", w)

	// No body request
	postReqWithCookie := httptest.NewRequest("POST", "/", nil)
	postReqWithCookie.AddCookie(w.Result().Cookies()[0])
	_, _, isNotOkBecauseNoCsrf, ok := s.ReadSessionCookie(postReqWithCookie)
	if ok || !isNotOkBecauseNoCsrf {
		t.Fatal("expected not ok bc its missing csrf in form")
	}

	// POST with form value should also pass.
	postReqWithCookie.Form.Add(CsrfFormName, w.Result().Cookies()[1].Value)
	// Set header like a real HTML form would.
	postReqWithCookie.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, _, _, ok = s.ReadSessionCookie(postReqWithCookie)
	if !ok {
		t.Fatal("expected ok when CSRF token is in form")
	}
}

func TestOAuthCallback(t *testing.T) {
	gAuth := oauthclient.NewGoogleMock()
	msAuth := oauthclient.NewMicrosoftMock()
	s := NewInsecureCookiesService(time.Duration(3*time.Hour), gAuth, msAuth)

	startW := httptest.NewRecorder()
	s.StartGoogleOAuthSession(startW, httptest.NewRequest("GET", "/login/google", nil))
	stateCookie := startW.Result().Cookies()[0]

	gAuth.AddGetUserInfo(oauthclient.UserInfo{Email: "user@example.com"})
	callbackReq := httptest.NewRequest(
		"GET", "/oauth/google/callback?state="+stateCookie.Value+"&code=abc", nil)
	callbackReq.AddCookie(stateCookie)
	info, ok := s.HandleGoogleOAuthCallback(httptest.NewRecorder(), callbackReq)
	if !ok {
		t.Fatal("expected successful callback")
	}
	if info.Email != "user@example.com" {
		t.Fatalf("wrong email: %s", info.Email)
	}

	// Replaying the same state (e.g. reloading the callback URL, or an
	// attacker replaying an intercepted URL) must not succeed: the state
	// must be single-use.
	gAuth.AddGetUserInfo(oauthclient.UserInfo{Email: "user@example.com"})
	replayReq := httptest.NewRequest(
		"GET", "/oauth/google/callback?state="+stateCookie.Value+"&code=def", nil)
	replayReq.AddCookie(stateCookie)
	_, ok = s.HandleGoogleOAuthCallback(httptest.NewRecorder(), replayReq)
	if ok {
		t.Fatal("expected state replay to fail")
	}
}

func TestOAuthCallbackWrongState(t *testing.T) {
	gAuth := oauthclient.NewGoogleMock()
	msAuth := oauthclient.NewMicrosoftMock()
	s := NewInsecureCookiesService(time.Duration(3*time.Hour), gAuth, msAuth)

	startW := httptest.NewRecorder()
	s.StartGoogleOAuthSession(startW, httptest.NewRequest("GET", "/login/google", nil))
	stateCookie := startW.Result().Cookies()[0]

	gAuth.AddGetUserInfo(oauthclient.UserInfo{Email: "user@example.com"})
	callbackReq := httptest.NewRequest(
		"GET", "/oauth/google/callback?state=wrong-state&code=abc", nil)
	callbackReq.AddCookie(stateCookie)
	_, ok := s.HandleGoogleOAuthCallback(httptest.NewRecorder(), callbackReq)
	if ok {
		t.Fatal("expected failure due to mismatched state")
	}
}