package webdb_test

import (
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/notification"
	"monorepo/twigg-web/webdb"
	"reflect"
	"testing"
)

func TestCreateAndGetUserNotifications(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const userId = 10
	// Create notifications for userId
	err = b.CreateNotification(w, userId, "msg-1", "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, userId, "msg-2", "asset-2")
	if err != nil {
		t.Fatal(err)
	}

	// Create notification for another user (must not appear)
	err = b.CreateNotification(w, 999, "other-user", "asset-x")
	if err != nil {
		t.Fatal(err)
	}

	it, err := b.GetUserNotifications(w, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	got, err := iterator.GetFirstN(10, it)
	if err != nil {
		t.Fatal(err)
	}

	for i := range got {
		if got[i].Id == 0 {
			t.Fatal("expected non-zero notification id")
		}
		if got[i].CreatedAt == "" {
			t.Fatal("expected createdAt to be set")
		}
		got[i].Id = int64(i + 1)
		got[i].CreatedAt = "X"
	}

	expected := []notification.Notification{
		{
			Id:        1,
			UserId:    userId,
			Message:   "msg-2",
			AssetPath: "asset-2",
			CreatedAt: "X",
			SeenAt:    "",
			ReadAt:    "",
		},
		{
			Id:        2,
			UserId:    userId,
			Message:   "msg-1",
			AssetPath: "asset-1",
			CreatedAt: "X",
			SeenAt:    "",
			ReadAt:    "",
		},
	}

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("unexpected notifications\nexpected: %#v\ngot:      %#v", expected, got)
	}
}
func TestMarkRead(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const userId = 42

	// Create notification
	err = b.CreateNotification(w, userId, "hello", "asset-9")
	if err != nil {
		t.Fatal(err)
	}

	// Get it (newest first)
	it, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Next() {
		t.Fatal("expected at least 1 notification")
	}
	n, err := it.Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}

	if n.ReadAt != "" {
		t.Fatal("expected unread before MarkRead")
	}
	if n.SeenAt != "" {
		t.Fatal("expected unseen before MarkRead")
	}

	// Mark it as read
	err = b.MarkNotificationRead(w, userId, n.Id)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it is read and seen now
	it2, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !it2.Next() {
		t.Fatal("expected notification after MarkRead")
	}
	n2, err := it2.Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := it2.Err(); err != nil {
		t.Fatal(err)
	}

	if n2.Id != n.Id {
		t.Fatalf("unexpected id: got %d want %d", n2.Id, n.Id)
	}
	if n2.ReadAt == "" {
		t.Fatal("expected readAt to be set after MarkRead")
	}
	if n2.SeenAt == "" {
		t.Fatal("expected seenAt to be auto-populated by MarkRead when previously empty")
	}
}

func TestGetUnseenCount(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const userId = 88
	const otherUserId = 999

	count, err := b.GetUnseenNotificationCount(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unseen, got %d", count)
	}

	err = b.CreateNotification(w, userId, "msg-1", "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, userId, "msg-2", "asset-2")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, otherUserId, "other", "asset-x")
	if err != nil {
		t.Fatal(err)
	}

	count, err = b.GetUnseenNotificationCount(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unseen, got %d", count)
	}

	it, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	notifs, err := iterator.GetFirstN(2, it)
	if err != nil {
		t.Fatal(err)
	}

	err = b.MarkNotificationSeen(w, userId, []int64{notifs[0].Id})
	if err != nil {
		t.Fatal(err)
	}

	count, err = b.GetUnseenNotificationCount(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unseen after MarkSeen, got %d", count)
	}
}

func TestMarkSeen(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const userId = 77
	const otherUserId = 999

	err = b.CreateNotification(w, userId, "msg-1", "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, userId, "msg-2", "asset-2")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, userId, "msg-3", "asset-3")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, otherUserId, "other", "asset-x")
	if err != nil {
		t.Fatal(err)
	}

	it, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	notifs, err := iterator.GetFirstN(10, it)
	if err != nil {
		t.Fatal(err)
	}

	// Mark first 2 as seen (newest first, so msg-3 and msg-2)
	err = b.MarkNotificationSeen(w, userId, []int64{notifs[0].Id, notifs[1].Id})
	if err != nil {
		t.Fatal(err)
	}

	it2, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	notifs2, err := iterator.GetFirstN(10, it2)
	if err != nil {
		t.Fatal(err)
	}

	if notifs2[0].SeenAt == "" {
		t.Fatal("expected SeenAt to be set on first notification")
	}
	if notifs2[1].SeenAt == "" {
		t.Fatal("expected SeenAt to be set on second notification")
	}
	if notifs2[2].SeenAt != "" {
		t.Fatal("expected SeenAt to be empty on third notification")
	}

	// Idempotent: calling again on already-seen notifications should not error
	err = b.MarkNotificationSeen(w, userId, []int64{notifs[0].Id, notifs[1].Id})
	if err != nil {
		t.Fatal(err)
	}

	// Empty list: no-op
	err = b.MarkNotificationSeen(w, userId, []int64{})
	if err != nil {
		t.Fatal(err)
	}

	// Security: passing another user's notification ID with userId should not mark it
	itOther, err := b.GetUserNotifications(w, otherUserId, 0)
	if err != nil {
		t.Fatal(err)
	}
	otherNotifs, err := iterator.GetFirstN(10, itOther)
	if err != nil {
		t.Fatal(err)
	}
	err = b.MarkNotificationSeen(w, userId, []int64{otherNotifs[0].Id})
	if err != nil {
		t.Fatal(err)
	}
	itOther2, err := b.GetUserNotifications(w, otherUserId, 0)
	if err != nil {
		t.Fatal(err)
	}
	otherNotifs2, err := iterator.GetFirstN(10, itOther2)
	if err != nil {
		t.Fatal(err)
	}
	if otherNotifs2[0].SeenAt != "" {
		t.Fatal("security: other user's notification should not be marked as seen")
	}
}

func TestCreateValidation(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	// missing userId
	err = b.CreateNotification(w, 0, "msg", "asset")
	if err == nil {
		t.Fatal("expected error for missing userId")
	}

	// missing message
	err = b.CreateNotification(w, 10, "", "asset")
	if err == nil {
		t.Fatal("expected error for missing message")
	}

	// missing assetId
	err = b.CreateNotification(w, 10, "msg", "")
	if err == nil {
		t.Fatal("expected error for missing assetId")
	}
}
func TestGetUserNotificationsPagination(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const userId = 55
	const total = 20

	// Create 20 notifications
	for i := 1; i <= total; i++ {
		err := b.CreateNotification(w, userId,
			fmt.Sprintf("msg-%d", i),
			fmt.Sprintf("asset-%d", i),
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Page 0
	it0, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	page0, err := iterator.GetFirstN(10, it0)
	if err != nil {
		t.Fatal(err)
	}

	if len(page0) != 10 {
		t.Fatalf("expected 10 items on first page, got %d", len(page0))
	}

	// Newest first
	if page0[0].Message != "msg-20" {
		t.Fatalf("expected msg-20 first, got %s", page0[0].Message)
	}
	if page0[9].Message != "msg-11" {
		t.Fatalf("expected msg-11 last on first page, got %s", page0[9].Message)
	}

	// Last notification id
	lastSeenId := page0[len(page0)-1].Id

	// Second page
	it1, err := b.GetUserNotifications(w, userId, lastSeenId)
	if err != nil {
		t.Fatal(err)
	}
	page1, err := iterator.GetFirstN(10, it1)
	if err != nil {
		t.Fatal(err)
	}

	if len(page1) != 10 {
		t.Fatalf("expected 10 items on second page, got %d", len(page1))
	}

	if page1[0].Message != "msg-10" {
		t.Fatalf("expected msg-10 first on second page, got %s", page1[0].Message)
	}
	if page1[9].Message != "msg-1" {
		t.Fatalf("expected msg-1 last on second page, got %s", page1[9].Message)
	}

	// Third page (should be empty)
	lastSeenId2 := page1[len(page1)-1].Id

	it2, err := b.GetUserNotifications(w, userId, lastSeenId2)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := iterator.GetFirstN(10, it2)
	if err != nil {
		t.Fatal(err)
	}

	if len(page2) != 0 {
		t.Fatalf("expected empty third page, got %d items", len(page2))
	}
}

func TestMarkAllSeen(t *testing.T) {
	b, cl, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	w, closeW, _, err := b.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}

	const userId = 55
	const otherUserId = 999

	err = b.CreateNotification(w, userId, "msg-1", "asset-1")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, userId, "msg-2", "asset-2")
	if err != nil {
		t.Fatal(err)
	}
	err = b.CreateNotification(w, otherUserId, "other", "asset-x")
	if err != nil {
		t.Fatal(err)
	}

	err = b.MarkAllNotificationsSeen(w, userId)
	if err != nil {
		t.Fatal(err)
	}

	it, err := b.GetUserNotifications(w, userId, 0)
	if err != nil {
		t.Fatal(err)
	}
	notifs, err := iterator.GetFirstN(2, it)
	if err != nil {
		t.Fatal(err)
	}
	if len(notifs) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifs))
	}
	for _, n := range notifs {
		if n.SeenAt == "" {
			t.Fatalf("expected all notifications to be seen, but %d is not", n.Id)
		}
	}

	count, err := b.GetUnseenNotificationCount(w, userId)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unseen after MarkAllSeen, got %d", count)
	}

	// Idempotent: calling again should not error
	err = b.MarkAllNotificationsSeen(w, userId)
	if err != nil {
		t.Fatal(err)
	}

	// Security: other user's notifications must not be affected
	itOther, err := b.GetUserNotifications(w, otherUserId, 0)
	if err != nil {
		t.Fatal(err)
	}
	otherNotifs, err := iterator.GetFirstN(1, itOther)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherNotifs) != 1 {
		t.Fatalf("expected 1 notification for other user, got %d", len(otherNotifs))
	}
	if otherNotifs[0].SeenAt != "" {
		t.Fatal("security: other user's notification should not be marked as seen")
	}
}
