package usereducation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"monorepo/twigg-web/education"
	"monorepo/twigg-web/handlers/usereducation"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
)

func TestHandleGetUserEducation(t *testing.T) {
	db := newMockedDb()
	var captureUserId int64
	db.getUserEducation = func(userId int64) (education.UserEducation, error) {
		captureUserId = userId
		ed := education.NewUserEducation(userId)
		ed.WelcomeWasShown = true
		return ed, nil
	}

	h := usereducation.NewHandler(db)
	r := newRequestForHandleGetUserEducation(42)
	w := httptest.NewRecorder()

	h.HandleGetUserEducation(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if captureUserId != 42 {
		t.Fatalf("GetUserEducation called with unexpected user id, got: %d", captureUserId)
	}

	var resp usereducation.GetUserEducationResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.WelcomeWasShown == false {
		t.Fatalf("expected WelcomeWasShown == true")
	}
}

func TestHandleGetUserEducationDbError(t *testing.T) {
	db := newMockedDb()
	db.getUserEducation = func(userId int64) (education.UserEducation, error) {
		return education.UserEducation{}, errors.New("db exploded")
	}

	h := usereducation.NewHandler(db)
	r := newRequestForHandleGetUserEducation(99)
	w := httptest.NewRecorder()

	h.HandleGetUserEducation(w, r, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestHandlePutWelcomeWasShown(t *testing.T) {
	db := newMockedDb()
	var captureUserId int64
	var captureWelcomeWasShown bool
	db.setWelcomeWasShown = func(userId int64, welcomeWasShown bool) error {
		captureUserId = userId
		captureWelcomeWasShown = welcomeWasShown
		return nil
	}

	const reqUserId = 42
	h := usereducation.NewHandler(db)
	r := newRequestForHandlePut(reqUserId)
	w := httptest.NewRecorder()

	shouldCommit := h.HandlePutWelcomeWasShown(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !shouldCommit {
		t.Fatalf("expected shouldCommit=true")
	}
	if captureUserId != reqUserId {
		t.Fatalf("SetWelcomeWasShown called with unexpected user id, got: %d", captureUserId)
	}
	if captureWelcomeWasShown == false {
		t.Fatalf("expected SetWelcomeWasShown to be called with true")
	}
}

func TestHandlePutWelcomeWasShownDbError(t *testing.T) {
	db := newMockedDb()
	db.setWelcomeWasShown = func(userId int64, welcomeWasShown bool) error {
		return errors.New("db exploded")
	}

	const reqUserId = 99
	h := usereducation.NewHandler(db)
	r := newRequestForHandlePut(reqUserId)
	w := httptest.NewRecorder()

	shouldCommit := h.HandlePutWelcomeWasShown(w, r, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
}

func newRequestForHandlePut(userId int64) wrappers.UserMuxRequest {
	req := httptest.NewRequest("PUT", "/user-education/welcome-was-shown", nil)
	return wrappers.UserMuxRequest{
		Request: req,
		User:    user.User{Id: userId},
	}
}

func newRequestForHandleGetUserEducation(userId int64) wrappers.UserMuxRequest {
	req := httptest.NewRequest("GET", "/user-education", nil)
	return wrappers.UserMuxRequest{
		Request: req,
		User:    user.User{Id: userId},
	}
}

func newMockedDb() *mockedDb {
	return &mockedDb{}
}

type mockedDb struct {
	getUserEducation   func(userId int64) (education.UserEducation, error)
	setWelcomeWasShown func(userId int64, welcomeWasShown bool) error
}

func (m *mockedDb) GetUserEducation(_ context.Context, userId int64) (education.UserEducation, error) {
	return m.getUserEducation(userId)
}

func (m *mockedDb) SetWelcomeWasShown(_ context.Context, userId int64, welcomeWasShown bool) error {
	return m.setWelcomeWasShown(userId, welcomeWasShown)
}