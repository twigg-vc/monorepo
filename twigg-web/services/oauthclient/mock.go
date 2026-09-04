package oauthclient

import (
	"errors"
	"monorepo/base/queue"
	"net/http"
)

type oAuthMock struct {
	redirectUrl string
	userInfos   queue.Queue[UserInfo]
	lastState   string
}

func (m oAuthMock) GetRedirectUrl() string {
	return m.redirectUrl
}
func (m *oAuthMock) SetRedirectUrl(url string) {
	m.redirectUrl = url
}
func (m *oAuthMock) AuthCodeURL(state string) string {
	m.lastState = state
	return m.redirectUrl + "?state=" + state
}
func (m oAuthMock) GetUserInfo(r *http.Request, code string) (UserInfo, error) {
	if m.userInfos.IsEmpty() {
		return UserInfo{}, errors.New("AddGetUserInfo was not called")
	}
	return m.userInfos.Pop(), nil
}

func (m oAuthMock) GetLastAuthCodeURLState() string {
	return m.lastState
}
func (m oAuthMock) AddGetUserInfo(userInfo UserInfo) {
	m.userInfos.Push(userInfo)
}
