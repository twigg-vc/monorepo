package bugs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const bugsPageSize = 25
const bugEventsPageSize = 100

// JSON escapes can make the body take more bytes than its text.
const maxRequestBytes = 2*bug.MaxBodyLen + 1024

type handler struct {
	db    Db
	perms Permissions
}

func (h handler) handleGetBugs(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	if !r.Flags.ShowBugs {
		http.NotFound(w, r.Request)
		return
	}
	params := r.Request.URL.Query()
	status := bug.Status(params.Get(routes.BugsStatusQueryParamName))
	if status != "" && !status.IsValid() {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	bugs, nextCursor, err := h.db.GetBugsPage(dbRead, r.Repo.Id, status,
		params.Get(routes.BugsCursorQueryParamName), bugsPageSize)
	if err != nil {
		log.Printf("failed to get the bugs of repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to get the bugs", http.StatusInternalServerError)
		return
	}
	open, closed, err := h.db.CountRepoBugs(dbRead, r.Repo.Id)
	if err != nil {
		log.Printf("failed to count the bugs of repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to count the bugs", http.StatusInternalServerError)
		return
	}
	canCreate, err := h.perms.CanCreateBugs(dbRead, viewer(r), r.Repo)
	if err != nil {
		log.Printf("failed to check if bugs can be created in repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to check the permission", http.StatusInternalServerError)
		return
	}
	frontendBugs, ok := newUsernames(h.db, dbRead, w).getFrontendBugs(bugs)
	if !ok {
		return
	}

	respJson, err := json.Marshal(GetBugsResponse{
		Bugs:        frontendBugs,
		NextCursor:  nextCursor,
		OpenCount:   open,
		ClosedCount: closed,
		CanCreate:   canCreate,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the bugs: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the bugs: %s", err)
	}
}

func (h handler) handleGetBug(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	if !r.Flags.ShowBugs {
		http.NotFound(w, r.Request)
		return
	}
	number, ok := parseBugNumber(w, r.Request)
	if !ok {
		return
	}
	b, isNotFoundErr, err := h.db.GetBug(dbRead, r.Repo.Id, number)
	if isNotFoundErr {
		http.Error(w, "bug not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("failed to get bug b/%d of repo id=%d: %s", number, r.Repo.Id, err)
		http.Error(w, "failed to get the bug", http.StatusInternalServerError)
		return
	}
	events, _, err := h.db.GetBugEvents(dbRead, r.Repo.Id, number, "", bugEventsPageSize)
	if err != nil {
		log.Printf("failed to get the events of b/%d of repo id=%d: %s", number, r.Repo.Id, err)
		http.Error(w, "failed to get the bug events", http.StatusInternalServerError)
		return
	}
	canWrite, err := h.perms.CanWriteBug(dbRead, viewer(r), r.Repo, b)
	if err != nil {
		log.Printf("failed to check if b/%d of repo id=%d can be written: %s", number, r.Repo.Id, err)
		http.Error(w, "failed to check the permission", http.StatusInternalServerError)
		return
	}
	u := newUsernames(h.db, dbRead, w)
	frontendBug, ok := u.getFrontendBug(b)
	if !ok {
		return
	}
	frontendEvents, ok := u.getFrontendBugEvents(events)
	if !ok {
		return
	}
	twiggwc.PageWithTitle(
		fmt.Sprintf("b/%d", number),
		/*hideNavBar=*/ false,
		r.Flags,
		twiggwc.BugPage(r.RepoOwnerUsr.Username, r.Repo.DisplayName, frontendBug, frontendEvents, canWrite),
	).Render(w)
}

func (h handler) handlePostBug(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.ShowBugs {
		http.NotFound(w, r.Request)
		return false
	}
	canCreate, err := h.perms.CanCreateBugs(dbWrite, &r.UserWithWritePermission, r.Repo)
	if err != nil {
		log.Printf("failed to check if bugs can be created in repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to check the permission", http.StatusInternalServerError)
		return false
	}
	if !canCreate {
		http.Error(w, "you can not create bugs in this repo", http.StatusForbidden)
		return false
	}
	var req PostBugRequest
	err = json.NewDecoder(http.MaxBytesReader(w, r.Request.Body, maxRequestBytes)).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		http.Error(w, "the title is required", http.StatusBadRequest)
		return false
	}
	if utf8.RuneCountInString(req.Title) > bug.MaxTitleLen {
		http.Error(w, "the title is too long", http.StatusBadRequest)
		return false
	}
	if len(req.Body) > bug.MaxBodyLen {
		http.Error(w, "the body is too long", http.StatusBadRequest)
		return false
	}
	b, err := h.db.CreateBug(dbWrite, r.Repo.Id, r.UserWithWritePermission.Id, req.Title, req.Body)
	if err != nil {
		log.Printf("failed to create a bug in repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to create the bug", http.StatusInternalServerError)
		return false
	}
	frontendBug, ok := newUsernames(h.db, dbWrite, w).getFrontendBug(b)
	if !ok {
		return false
	}

	respJson, err := json.Marshal(frontendBug)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the bug: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the bug: %s", err)
	}
	return true
}

func (h handler) handlePostComment(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.ShowBugs {
		http.NotFound(w, r.Request)
		return false
	}
	b, ok := h.getWritableBug(w, r, dbWrite)
	if !ok {
		return false
	}
	var req PostCommentRequest
	err := json.NewDecoder(http.MaxBytesReader(w, r.Request.Body, maxRequestBytes)).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	if strings.TrimSpace(req.Body) == "" {
		http.Error(w, "the comment is required", http.StatusBadRequest)
		return false
	}
	if len(req.Body) > bug.MaxBodyLen {
		http.Error(w, "the comment is too long", http.StatusBadRequest)
		return false
	}
	e, _, err := h.db.AddBugComment(dbWrite, r.Repo.Id, b.Number, r.UserWithWritePermission.Id, req.Body)
	if err != nil {
		log.Printf("failed to comment on b/%d of repo id=%d: %s", b.Number, r.Repo.Id, err)
		http.Error(w, "failed to add the comment", http.StatusInternalServerError)
		return false
	}

	respJson, err := json.Marshal(twiggwc.BugEventToFrontend(e, r.UserWithWritePermission.Username))
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the comment: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the comment: %s", err)
	}
	return true
}

func (h handler) handlePostStatus(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.ShowBugs {
		http.NotFound(w, r.Request)
		return false
	}
	b, ok := h.getWritableBug(w, r, dbWrite)
	if !ok {
		return false
	}
	var req PostStatusRequest
	err := json.NewDecoder(http.MaxBytesReader(w, r.Request.Body, maxRequestBytes)).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	if !req.Status.IsValid() {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return false
	}
	if b.Status == req.Status {
		http.Error(w, "the bug is already "+string(req.Status), http.StatusConflict)
		return false
	}
	e, _, err := h.db.SetBugStatus(dbWrite, r.Repo.Id, b.Number, r.UserWithWritePermission.Id, req.Status)
	if err != nil {
		log.Printf("failed to set the status of b/%d of repo id=%d: %s", b.Number, r.Repo.Id, err)
		http.Error(w, "failed to set the status", http.StatusInternalServerError)
		return false
	}
	number := b.Number
	b, _, err = h.db.GetBug(dbWrite, r.Repo.Id, number)
	if err != nil {
		log.Printf("failed to get bug b/%d of repo id=%d: %s", number, r.Repo.Id, err)
		http.Error(w, "failed to get the bug", http.StatusInternalServerError)
		return false
	}
	frontendBug, ok := newUsernames(h.db, dbWrite, w).getFrontendBug(b)
	if !ok {
		return false
	}

	respJson, err := json.Marshal(PostStatusResponse{
		Bug:    frontendBug,
		Events: []twiggwc.FrontendBugEvent{twiggwc.BugEventToFrontend(e, r.UserWithWritePermission.Username)},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the status change: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the status change: %s", err)
	}
	return true
}

// Loads the bug of the request and checks the user can write it. On any
// error, writes an error to the response and returns ok=false.
func (h handler) getWritableBug(w http.ResponseWriter, r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (b bug.Bug, ok bool) {
	number, ok := parseBugNumber(w, r.Request)
	if !ok {
		return bug.Bug{}, false
	}
	b, isNotFoundErr, err := h.db.GetBug(dbWrite, r.Repo.Id, number)
	if isNotFoundErr {
		http.Error(w, "bug not found", http.StatusNotFound)
		return bug.Bug{}, false
	}
	if err != nil {
		log.Printf("failed to get bug b/%d of repo id=%d: %s", number, r.Repo.Id, err)
		http.Error(w, "failed to get the bug", http.StatusInternalServerError)
		return bug.Bug{}, false
	}
	canWrite, err := h.perms.CanWriteBug(dbWrite, &r.UserWithWritePermission, r.Repo, b)
	if err != nil {
		log.Printf("failed to check if b/%d of repo id=%d can be written: %s", number, r.Repo.Id, err)
		http.Error(w, "failed to check the permission", http.StatusInternalServerError)
		return bug.Bug{}, false
	}
	if !canWrite {
		http.Error(w, "you can not change this bug", http.StatusForbidden)
		return bug.Bug{}, false
	}
	return b, true
}

func parseBugNumber(w http.ResponseWriter, r *http.Request) (number uint64, ok bool) {
	number, err := strconv.ParseUint(r.PathValue(routes.BugNumberParamName), 10, 64)
	if err != nil {
		http.Error(w, "invalid bug number", http.StatusBadRequest)
		return 0, false
	}
	return number, true
}

// Returns nil for anonymous users.
func viewer(r wrappers.UserWithReadPermissionMuxRequest) *user.User {
	if !r.IsLoggedIn {
		return nil
	}
	return r.MaybeUserWithReadPermission
}
