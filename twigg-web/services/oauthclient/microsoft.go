package oauthclient

import (
	"encoding/json"
	"errors"
	"monorepo/base/queue"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type microsoftOAuthClient struct {
	cfg *oauth2.Config
}

func newMicrosoft(redirectUrl string, msAzureOAuthClientId, msAzureOAuthClientSecret string) microsoftOAuthClient {
	return microsoftOAuthClient{
		cfg: &oauth2.Config{
			ClientID:     msAzureOAuthClientId,
			ClientSecret: msAzureOAuthClientSecret,
			RedirectURL:  redirectUrl,
			//Use common tenant for public apps
			Endpoint: microsoft.AzureADEndpoint("common"),
			// 1. openid: This is mandatory. Without it, Microsoft doesn't treat the request as an authentication attempt and may reject "identity" scopes like email.
			// 2. profile: While not strictly required for the email, it's best practice when fetching user info to ensure the "me" endpoint has permission to return basic display information.
			// 3. The "Email" Scope: In Microsoft's world, the email scope gives you permission to see the user's primary email address, but it only works correctly when combined with openid.
			// 4. Allows the app to call the Microsoft Graph API (https://graph.microsoft.com/v1.0/me) to read the signed-in user's full profile.
			Scopes: []string{"openid", "profile", "email", "User.Read"},
		},
	}
}

// Returns the URL for the user to sign in given a random state string
func (ms microsoftOAuthClient) AuthCodeURL(state string) string {
	return ms.cfg.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// Given a code that is sent by the user after sigining in, get the
// user info from Microsoft. An http request is used as an agument just because
// its context is used to potentially cancel the request to GetUserInfo
func (ms microsoftOAuthClient) GetUserInfo(r *http.Request, code string) (uInfo UserInfo, err error) {
	// Exchange code for access token
	token, err := ms.cfg.Exchange(r.Context(), code)
	if err != nil {
		return
	}

	// Exchange token for userinfo
	c := ms.cfg.Client(r.Context(), token)
	resp, err := c.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("failed to get user info form microsoft")
		return
	}
	var info microsoftUserInfo
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return
	}
	uInfo = UserInfo{
		Email: info.UserPrincipalName,
	}
	return
}

type microsoftUserInfo struct {
	UserPrincipalName string `json:"userPrincipalName"`
}

func newMicrosoftMock() MicrosoftMock {
	return &oAuthMock{
		redirectUrl: "/mock-microsoft-login",
		userInfos:   queue.New[UserInfo](),
		lastState:   "",
	}
}
