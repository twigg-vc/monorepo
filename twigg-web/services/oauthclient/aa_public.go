package oauthclient

import (
	"monorepo/twigg-web/routes"
	"net/http"
)

type UserInfo struct {
	Email string
}

// Client to get OAuth data from google
type Google interface {
	// Returns the URL for the user to sign in given a random state string
	AuthCodeURL(state string) string
	// Given a code that is sent by the user after sigining in, get the
	// user info from Google. An http request is used as an agument just because
	// its context is used to potentially cancel the request to GetUserInfo
	GetUserInfo(r *http.Request, code string) (UserInfo, error)
}

// Returns OAuth client that sends redirects to server running locally
func NewLocalGoogle(serverRootUrl string, googleClientId, googleClientSecret string) Google {
	return newGoogle(serverRootUrl+routes.CallbackLoginWithGoogleOAuth, googleClientId, googleClientSecret)
}

// Mock a google oauth client
type GoogleMock interface {
	Google
	// Returns the base url used for redirects
	GetRedirectUrl() string
	// Set the base url used for redirects
	SetRedirectUrl(url string)
	// Returns the last state passed to AuthCodeURL
	GetLastAuthCodeURLState() string
	// Adds a userInfo to be returned by the next calls of AuthCodeURL
	AddGetUserInfo(userInfo UserInfo)
}

func NewGoogleMock() GoogleMock {
	return newGoogleMock()
}

// Client to get OAuth data from Microsoft
type Microsoft interface {
	// Returns the URL for the user to sign in given a random state string
	AuthCodeURL(state string) string
	// Given a code that is sent by the user after sigining in, get the
	// user info from Microsoft. An http request is used as an agument just
	// because its context is used to potentially cancel the request to
	// GetUserInfo
	GetUserInfo(r *http.Request, code string) (UserInfo, error)
}

// Returns OAuth client that sends redirects to server running locally
func NewLocalMicrosoft(serverRootUrl string, msAzureOAuthClientId, msAzureOAuthClientSecret string) Microsoft {
	return newMicrosoft(serverRootUrl+routes.CallbackLoginWithMicrosoftOAuth, msAzureOAuthClientId, msAzureOAuthClientSecret)
}

// Mock a Microsoft oauth client
type MicrosoftMock interface {
	Microsoft
	// Returns the base url used for redirects
	GetRedirectUrl() string
	// Set the base url used for redirects
	SetRedirectUrl(url string)
	// Returns the last state passed to AuthCodeURL
	GetLastAuthCodeURLState() string
	// Adds a userInfo to be returned by the next calls of AuthCodeURL
	AddGetUserInfo(userInfo UserInfo)
}

func NewMicrosoftMock() MicrosoftMock {
	return newMicrosoftMock()
}