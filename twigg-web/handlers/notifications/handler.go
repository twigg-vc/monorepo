package notifications

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"monorepo/base/iterator"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
)

type handler struct {
	rt routes.Router
	db Db
}

func newHandler(
	rt routes.Router,
	db Db,
) handler {
	return handler{
		rt: rt,
		db: db,
	}
}

func (hl handler) handleGet(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbWrite context.Context,
) (shouldCommit bool) {
	var lastReadNotificationId int64

	if s := r.URL.Query().Get("LastReadNotificationId"); s != "" {
		var err error
		lastReadNotificationId, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			http.Error(w, "bad LastReadNotificationId", http.StatusBadRequest)
			return false
		}
	}

	it, err := hl.db.GetUserNotifications(
		dbWrite,
		r.UserWithSub.Id,
		lastReadNotificationId,
	)
	if err != nil {
		http.Error(w, "internal error getting user notifications", http.StatusInternalServerError)
		return false
	}

	const maxItems = 5
	notifs, err := iterator.GetFirstN(maxItems, it)
	if err != nil {
		log.Printf("failed to get notifications: %v", err)
		http.Error(w, "internal error getting notifications", http.StatusInternalServerError)
		return false
	}

	// Mark the returned notifications as seen so the frontend doesn't need a
	// separate POST /notifications/seen call after opening the panel.
	notifIds := make([]int64, len(notifs))
	for i, n := range notifs {
		notifIds[i] = n.Id
	}
	if err := hl.db.MarkNotificationSeen(dbWrite, r.UserWithSub.Id, notifIds); err != nil {
		log.Printf("failed to mark notifications as seen: %v", err)
		http.Error(w, "internal error marking notifications as seen", http.StatusInternalServerError)
		return false
	}

	unseenCount, err := hl.db.GetUnseenNotificationCount(dbWrite, r.UserWithSub.Id)
	if err != nil {
		log.Printf("failed to get unseen count: %v", err)
		http.Error(w, "internal error getting unseen count", http.StatusInternalServerError)
		return false
	}

	resp := GetNotificationsResponse{
		Notifications: notifs,
		UnseenCount:   unseenCount,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
	return true
}

func (hl handler) handleGetUnseenCount(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbRead context.Context,
) {
	count, err := hl.db.GetUnseenNotificationCount(dbRead, r.UserWithSub.Id)
	if err != nil {
		log.Printf("failed to get unseen count: %v", err)
		http.Error(w, "internal error getting unseen count", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GetUnseenCountResponse{Count: count})
}

func (hl handler) handleMarkSeen(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbWrite context.Context,
) (shouldCommit bool) {
	type markSeenReq struct {
		NotificationIds []int64
	}
	var req markSeenReq

	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&req)
	if err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return false
	}

	const maxNotificationIds = 500
	if len(req.NotificationIds) > maxNotificationIds {
		http.Error(w, "too many notification ids", http.StatusBadRequest)
		return false
	}

	err = hl.db.MarkNotificationSeen(
		dbWrite,
		r.UserWithSub.Id,
		req.NotificationIds,
	)
	if err != nil {
		log.Printf("failed to mark notifications as seen: %q", err)
		http.Error(w, "failed to mark notifications as seen", http.StatusInternalServerError)
		return false
	}

	w.WriteHeader(http.StatusNoContent)
	return true
}

func (hl handler) handleMarkAllSeen(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbWrite context.Context,
) (shouldCommit bool) {
	err := hl.db.MarkAllNotificationsSeen(dbWrite, r.UserWithSub.Id)
	if err != nil {
		log.Printf("failed to mark all notifications as seen: %q", err)
		http.Error(w, "failed to mark all notifications as seen", http.StatusInternalServerError)
		return false
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (hl handler) handleMarkRead(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbWrite context.Context,
) (shouldCommit bool) {
	type markReadReq struct {
		NotificationId int64
	}
	var req markReadReq

	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return false
	}

	err := hl.db.MarkNotificationRead(
		dbWrite,
		r.UserWithSub.Id,
		req.NotificationId,
	)
	if err != nil {
		log.Printf("failed to mark as read: %q:", err)
		http.Error(w, "failed to mark as read", http.StatusNotFound)
		return false
	}

	w.WriteHeader(http.StatusNoContent)
	return true
}
