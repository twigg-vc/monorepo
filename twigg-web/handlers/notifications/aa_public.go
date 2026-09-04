package notifications

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/notification"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
)

// Registers the handlers required for notifications.
// All handlers will be wrapped with the provided `wrap` if its not nil.
func AddHandlers(
	rt routes.Router,
	db Db,
	userWithSubMux wrappers.UserWithSubMux,
) {
	h := newHandler(rt, db)

	userWithSubMux.HandleFuncW("GET "+routes.Notifications, h.handleGet)
	userWithSubMux.HandleFuncR("GET "+routes.NotificationUnseenCount, h.handleGetUnseenCount)
	userWithSubMux.HandleFuncW("POST "+routes.NotificationMarkRead, h.handleMarkRead)
	userWithSubMux.HandleFuncW("POST "+routes.NotificationMarkSeen, h.handleMarkSeen)
	userWithSubMux.HandleFuncW("POST "+routes.NotificationMarkAllSeen, h.handleMarkAllSeen)
}

type Db interface {
	GetUserNotifications(ctx context.Context, userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error)
	GetUnseenNotificationCount(ctx context.Context, userId int64) (int64, error)
	MarkNotificationSeen(writeCtx context.Context, userId int64, notificationIds []int64) error
	MarkNotificationRead(writeCtx context.Context, userId int64, notificationId int64) error
	MarkAllNotificationsSeen(writeCtx context.Context, userId int64) error
}

type GetNotificationsResponse struct {
	Notifications []notification.Notification
	UnseenCount   int64
}

type GetUnseenCountResponse struct {
	Count int64
}
