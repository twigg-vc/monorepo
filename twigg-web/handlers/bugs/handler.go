package bugs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strings"
)

const bugsPageSize = 25

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
	err = json.NewDecoder(r.Request.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return false
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		http.Error(w, "the title is required", http.StatusBadRequest)
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

// Returns nil for anonymous users.
func viewer(r wrappers.UserWithReadPermissionMuxRequest) *user.User {
	if !r.IsLoggedIn {
		return nil
	}
	return r.MaybeUserWithReadPermission
}
