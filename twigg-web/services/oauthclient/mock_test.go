package oauthclient

import "testing"

func TestGoogleMock(t *testing.T) {
	m := NewGoogleMock()

	m.SetRedirectUrl("/g")
	if m.AuthCodeURL("123") != "/g?state=123" {
		t.Fatal("unexpected AuthCodeURL")
	}

	expected := UserInfo{Email: "test"}
	m.AddGetUserInfo(expected)
	info, err := m.GetUserInfo(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if info != expected {
		t.Fatal("unexpected info")
	}
	_, err = m.GetUserInfo(nil, "")
	if err == nil {
		t.Fatal("expected err bc only one userInfo was added")
	}
}

func TestMicrosoftMock(t *testing.T) {
	m := NewMicrosoftMock()

	m.SetRedirectUrl("/ms")
	if m.AuthCodeURL("123") != "/ms?state=123" {
		t.Fatal("unexpected AuthCodeURL")
	}

	expected := UserInfo{Email: "test"}
	m.AddGetUserInfo(expected)
	info, err := m.GetUserInfo(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if info != expected {
		t.Fatal("unexpected info")
	}
	_, err = m.GetUserInfo(nil, "")
	if err == nil {
		t.Fatal("expected err bc only one userInfo was added")
	}
}
