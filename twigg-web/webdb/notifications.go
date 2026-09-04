package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"monorepo/base/iterator"
	"monorepo/twigg-web/notification"
	"strings"
)

func (db webDb) CreateNotification(writeCtx context.Context, userId int64, message string, assetPath string) error {
	if userId == 0 {
		return fmt.Errorf("missing userId")
	}
	if message == "" {
		return fmt.Errorf("missing message")
	}
	if assetPath == "" {
		// Keep it strict for now (helps debugging / linking notifications)
		return fmt.Errorf("missing assetPath")
	}
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO notifications (
			userId, message, assetPath
		) VALUES (?, ?, ?)
	`, userId, message, assetPath)
	if err != nil {
		return fmt.Errorf("failed to create notification: %s", err)
	}
	_, err = db.s.Exec(writeCtx, `
		INSERT INTO user_unseen_notify_count (userId, unseenCount) VALUES (?, 1)
		ON CONFLICT(userId) DO UPDATE SET unseenCount = unseenCount + 1
	`, userId)
	if err != nil {
		return fmt.Errorf("failed to increment unseen count: %w", err)
	}
	return nil
}

func (db webDb) MarkNotificationRead(writeCtx context.Context, userId int64, notificationId int64) error {
	if userId == 0 {
		return fmt.Errorf("missing userId")
	}
	if notificationId == 0 {
		return fmt.Errorf("missing notificationId")
	}

	res, err := db.s.Exec(writeCtx, `
		UPDATE notifications
		SET
			readAt = CURRENT_TIMESTAMP,
			seenAt = CASE WHEN seenAt = '' THEN CURRENT_TIMESTAMP ELSE seenAt END
		WHERE
			id = ? AND userId = ? AND readAt = ''
	`, notificationId, userId)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %s", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("notification not updated (not found / not owned / already read)")
	}
	return nil
}

func (db webDb) MarkNotificationSeen(writeCtx context.Context, userId int64, notificationIds []int64) error {
	if len(notificationIds) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(notificationIds))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	args := make([]any, 0, len(notificationIds)+1) // args = { arg1, arg2, ... , userId)
	for _, id := range notificationIds {
		args = append(args, id)
	}
	args = append(args, userId)

	res, err := db.s.Exec(writeCtx, fmt.Sprintf(`
		UPDATE notifications
		SET seenAt = CURRENT_TIMESTAMP
		WHERE id IN (%s) AND userId = ? AND seenAt = ''
	`, placeholders), args...)
	if err != nil {
		return fmt.Errorf("failed to mark notifications as seen: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected > 0 {
		_, err = db.s.Exec(writeCtx, `
			UPDATE user_unseen_notify_count
			SET unseenCount = MAX(0, unseenCount - ?)
			WHERE userId = ?
		`, affected, userId)
		if err != nil {
			return fmt.Errorf("failed to decrement unseen count: %w", err)
		}
	}
	return nil
}

func (db webDb) MarkAllNotificationsSeen(writeCtx context.Context, userId int64) error {
	if userId == 0 {
		return fmt.Errorf("missing userId")
	}
	_, err := db.s.Exec(writeCtx, `
		UPDATE notifications
		SET seenAt = CURRENT_TIMESTAMP
		WHERE userId = ? AND seenAt = ''
	`, userId)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as seen: %w", err)
	}
	_, err = db.s.Exec(writeCtx, `
		UPDATE user_unseen_notify_count SET unseenCount = 0 WHERE userId = ?
	`, userId)
	if err != nil {
		return fmt.Errorf("failed to reset unseen count: %w", err)
	}
	return nil
}

type notificationIterWrapper struct {
	rows *sql.Rows
}

func (it notificationIterWrapper) Get() (notification.Notification, error) {
	var n notification.Notification
	err := it.rows.Scan(
		&n.Id,
		&n.UserId,
		&n.Message,
		&n.AssetPath,
		&n.CreatedAt,
		&n.SeenAt,
		&n.ReadAt,
	)
	if err != nil {
		return notification.Notification{}, fmt.Errorf("failed to get notification from iter: %s", err)
	}
	return n, nil
}
func (it notificationIterWrapper) Next() bool { return it.rows.Next() }
func (it notificationIterWrapper) Err() error { return it.rows.Err() }

func (db webDb) GetUserNotifications(ctx context.Context, userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error) {
	if lastReadNotificationId <= 0 {
		lastReadNotificationId = math.MaxInt64
	}
	rows, err := db.s.Query(ctx, `
		SELECT
			id, userId, message, assetPath, createdAt, seenAt, readAt
		FROM notifications
		WHERE 
			userId = ? AND id < ?
		ORDER BY id DESC
	`, userId, lastReadNotificationId)

	if err != nil {
		return nil, fmt.Errorf("failed to get user notifications: %w", err)
	}

	return notificationIterWrapper{rows}, nil
}

func (db webDb) GetUnseenNotificationCount(ctx context.Context, userId int64) (int64, error) {
	var count int64
	err := db.s.QueryRow(ctx, `
		SELECT unseenCount FROM user_unseen_notify_count WHERE userId = ?
	`, userId).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get unseen count: %w", err)
	}
	return count, nil
}
