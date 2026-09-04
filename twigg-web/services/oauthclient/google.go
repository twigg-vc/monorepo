package oauthclient

import (
	"encoding/json"
	"errors"
	"monorepo/base/queue"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type googleOAuthClient struct {
	cfg *oauth2.Config
}

func newGoogle(redirectUrl string, googleClientId, googleClientSecret string) Google {
	return googleOAuthClient{
		cfg: &oauth2.Config{
			ClientID:     googleClientId,
			ClientSecret: googleClientSecret,
			RedirectURL:  redirectUrl,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"email"},
		},
	}
}

func (g googleOAuthClient) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
}
func (g googleOAuthClient) GetUserInfo(r *http.Request, code string) (uInfo UserInfo, err error) {
	// Exchange code for access token
	token, err := g.cfg.Exchange(r.Context(), code)
	if err != nil {
		return
	}

	// Exchange token for userinfo
	c := g.cfg.Client(r.Context(), token)
	resp, err := c.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("failed to get user info")
		return
	}
	var info googleUserInfo
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return
	}
	if !info.EmailVerified {
		err = errors.New("google email is not verified")
		return
	}
	uInfo = UserInfo{
		Email: info.GoogleEmail,
	}
	return
}

type googleUserInfo struct {
	GoogleEmail   string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

func newGoogleMock() GoogleMock {
	return &oAuthMock{
		redirectUrl: "/mock-google-login",
		userInfos:   queue.New[UserInfo](),
		lastState:   "",
	}
}