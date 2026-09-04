package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"monorepo/base/iterator"
	"monorepo/twigg-web/notification"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
)

func TestHandleGetUnseenCount(t *testing.T) {
	svc := newMockedNotificationService()
	var captureUserId int64
	svc.getUnseenCount = func(userId int64) (int64, error) {
		captureUserId = userId
		return 7, nil
	}

	h := newHandler(nil, svc)
	r := newRequestForHandleGetUnseenCount(42)
	w := httptest.NewRecorder()

	h.handleGetUnseenCount(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if captureUserId != 42 {
		t.Fatalf("GetUnseenCount called with unexpected user id, got: %d", captureUserId)
	}

	var resp GetUnseenCountResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Count != 7 {
		t.Fatalf("expected Count=7, got %d", resp.Count)
	}
}

func TestHandleGetUnseenCountServiceError(t *testing.T) {
	svc := newMockedNotificationService()
	svc.getUnseenCount = func(userId int64) (int64, error) {
		return 0, errors.New("db exploded")
	}

	h := newHandler(nil, svc)
	r := newRequestForHandleGetUnseenCount(99)
	w := httptest.NewRecorder()

	h.handleGetUnseenCount(w, r, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestHandleMarkAllSeen(t *testing.T) {
	svc := newMockedNotificationService()
	var captureUserId int64
	svc.markAllSeen = func(userId int64) error {
		captureUserId = userId
		return nil
	}

	h := newHandler(nil, svc)
	r := newRequestForHandleMarkAllSeen(42)
	w := httptest.NewRecorder()

	shouldCommit := h.handleMarkAllSeen(w, r, nil)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if captureUserId != 42 {
		t.Fatalf("MarkAllSeen called with unexpected user id, got: %d", captureUserId)
	}
	if !shouldCommit {
		t.Fatalf("expected shouldCommit=true")
	}
}

func TestHandleGet(t *testing.T) {
	svc := newMockedNotificationService()

	notif1 := notification.Notification{Id: 10, UserId: 42, Message: "msg1", AssetPath: "/a/b"}
	notif2 := notification.Notification{Id: 11, UserId: 42, Message: "msg2", AssetPath: "/a/c"}

	svc.getUserNotifications = func(userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error) {
		return iterator.NewIterFromSlice([]notification.Notification{notif1, notif2}), nil
	}
	var captureSeenIds []int64
	svc.markSeen = func(userId int64, notificationIds []int64) error {
		captureSeenIds = notificationIds
		return nil
	}
	svc.getUnseenCount = func(userId int64) (int64, error) {
		return 3, nil
	}

	h := newHandler(nil, svc)
	r := newRequestForHandleGet(42)
	w := httptest.NewRecorder()

	shouldCommit := h.handleGet(w, r, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !shouldCommit {
		t.Fatalf("expected shouldCommit=true")
	}

	var resp GetNotificationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(resp.Notifications))
	}
	if resp.UnseenCount != 3 {
		t.Fatalf("expected UnseenCount=3, got %d", resp.UnseenCount)
	}
	if len(captureSeenIds) != 2 || captureSeenIds[0] != 10 || captureSeenIds[1] != 11 {
		t.Fatalf("MarkSeen called with unexpected ids: %v", captureSeenIds)
	}
}

func TestHandleGetGetNotificationsError(t *testing.T) {
	svc := newMockedNotificationService()
	svc.getUserNotifications = func(userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error) {
		return nil, errors.New("db exploded")
	}

	h := newHandler(nil, svc)
	r := newRequestForHandleGet(42)
	w := httptest.NewRecorder()

	shouldCommit := h.handleGet(w, r, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
}

func TestHandleGetMarkSeenError(t *testing.T) {
	svc := newMockedNotificationService()
	svc.getUserNotifications = func(userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error) {
		return iterator.NewIterFromSlice([]notification.Notification{{Id: 1}}), nil
	}
	svc.markSeen = func(userId int64, notificationIds []int64) error {
		return errors.New("db exploded")
	}

	h := newHandler(nil, svc)
	r := newRequestForHandleGet(42)
	w := httptest.NewRecorder()

	shouldCommit := h.handleGet(w, r, nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
	if shouldCommit {
		t.Fatalf("expected shouldCommit=false")
	}
}

func newRequestForHandleGet(userId int64) wrappers.UserWithSubMuxRequest {
	req := httptest.NewRequest("GET", "/notifications", nil)
	return wrappers.UserWithSubMuxRequest{
		Request:     req,
		UserWithSub: user.User{Id: userId},
	}
}

func newRequestForHandleMarkAllSeen(userId int64) wrappers.UserWithSubMuxRequest {
	req := httptest.NewRequest("POST", "/notifications/mark-all-seen", nil)
	return wrappers.UserWithSubMuxRequest{
		Request:     req,
		UserWithSub: user.User{Id: userId},
	}
}

func newRequestForHandleGetUnseenCount(userId int64) wrappers.UserWithSubMuxRequest {
	req := httptest.NewRequest("GET", "/notifications/unseen-count", nil)
	return wrappers.UserWithSubMuxRequest{
		Request:     req,
		UserWithSub: user.User{Id: userId},
	}
}

func newMockedNotificationService() *mockedNotificationService {
	return &mockedNotificationService{}
}

type mockedNotificationService struct {
	markAllSeen          func(userId int64) error
	getUnseenCount       func(userId int64) (int64, error)
	getUserNotifications func(userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error)
	markSeen             func(userId int64, notificationIds []int64) error
}

func (m *mockedNotificationService) GetUnseenNotificationCount(_ context.Context, userId int64) (int64, error) {
	return m.getUnseenCount(userId)
}

func (m *mockedNotificationService) CreateNotification(_ context.Context, _ int64, _ string, _ string) error {
	panic("unexpected call to Create")
}

func (m *mockedNotificationService) MarkNotificationRead(_ context.Context, _ int64, _ int64) error {
	panic("unexpected call to MarkRead")
}

func (m *mockedNotificationService) MarkNotificationSeen(_ context.Context, userId int64, notificationIds []int64) error {
	return m.markSeen(userId, notificationIds)
}

func (m *mockedNotificationService) GetUserNotifications(_ context.Context, userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error) {
	return m.getUserNotifications(userId, lastReadNotificationId)
}

func (m *mockedNotificationService) MarkAllNotificationsSeen(_ context.Context, userId int64) error {
	return m.markAllSeen(userId)
}
