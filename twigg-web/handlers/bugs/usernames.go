package bugs

import (
	"context"
	"log"
	"monorepo/twigg-web/bug"
	"monorepo/twigg/client"
	"net/http"
)

// Fetches each username once per request. On any error, writes an error to
// the response and returns ok=false.
type usernameResolver struct {
	db     Db
	dbRead context.Context
	w      http.ResponseWriter
	byId   map[int64]string
}

func newUsernames(db Db, dbRead context.Context, w http.ResponseWriter) *usernameResolver {
	return &usernameResolver{db: db, dbRead: dbRead, w: w, byId: map[int64]string{}}
}

func (u *usernameResolver) getUsername(userId int64) (username string, ok bool) {
	username, ok = u.byId[userId]
	if ok {
		return username, true
	}
	username, isNotFoundErr, err := u.db.GetUsername(u.dbRead, userId)
	if isNotFoundErr {
		http.Error(u.w, "user not found", http.StatusNotFound)
		return "", false
	}
	if err != nil {
		log.Printf("failed to get username of user %d: %s", userId, err)
		http.Error(u.w, "failed to get username", http.StatusInternalServerError)
		return "", false
	}
	u.byId[userId] = username
	return username, true
}

func (u *usernameResolver) getFrontendBug(b bug.Bug) (client.FrontendBug, bool) {
	author, ok := u.getUsername(b.AuthorUserId)
	if !ok {
		return client.FrontendBug{}, false
	}
	assignee := ""
	if b.AssigneeUserId != 0 {
		assignee, ok = u.getUsername(b.AssigneeUserId)
		if !ok {
			return client.FrontendBug{}, false
		}
	}
	return client.BugToFrontend(b, author, assignee), true
}

func (u *usernameResolver) getFrontendBugEvent(e bug.Event) (client.FrontendBugEvent, bool) {
	author, ok := u.getUsername(e.AuthorUserId)
	if !ok {
		return client.FrontendBugEvent{}, false
	}
	newAssignee := ""
	if e.Kind == bug.EventKind_Assignment && e.Assignment.NewAssigneeUserId != 0 {
		newAssignee, ok = u.getUsername(e.Assignment.NewAssigneeUserId)
		if !ok {
			return client.FrontendBugEvent{}, false
		}
	}
	return client.BugEventToFrontend(e, author, newAssignee), true
}

func (u *usernameResolver) getFrontendBugEvents(es []bug.Event) ([]client.FrontendBugEvent, bool) {
	fes := make([]client.FrontendBugEvent, 0, len(es))
	for _, e := range es {
		fe, ok := u.getFrontendBugEvent(e)
		if !ok {
			return nil, false
		}
		fes = append(fes, fe)
	}
	return fes, true
}

func (u *usernameResolver) getFrontendBugs(bs []bug.Bug) ([]client.FrontendBug, bool) {
	fbs := make([]client.FrontendBug, 0, len(bs))
	for _, b := range bs {
		fb, ok := u.getFrontendBug(b)
		if !ok {
			return nil, false
		}
		fbs = append(fbs, fb)
	}
	return fbs, true
}
