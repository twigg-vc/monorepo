package bugs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/bugsearch"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

const bugSearchPageSize = 25

func (h handler) handleSearchBugs(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	if !r.Flags.SearchBugsUi {
		http.NotFound(w, r.Request)
		return
	}
	params := r.Request.URL.Query()
	f, err := bugsearch.ParseQuery(r.Repo.Id,
		params.Get(routes.BugSearchQueryParamName))
	if err != nil {
		// The message is what the search bar shows under itself.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ok := replaceAssigneeUsernameIfEqualToMe(&f, w, r)
	if !ok {
		return
	}
	bugs, nextCursor, err := h.db.SearchBugs(dbRead, f,
		params.Get(routes.BugSearchCursorParamName), bugSearchPageSize)
	if err != nil {
		log.Printf("failed to search the bugs of repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to search the bugs", http.StatusInternalServerError)
		return
	}
	frontendBugs, ok := newUsernames(h.db, dbRead, w).getFrontendBugs(bugs)
	if !ok {
		return
	}

	respJson, err := json.Marshal(BugSearchResponse{
		Bugs:       frontendBugs,
		NextCursor: nextCursor,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the search results: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the search results: %s", err)
	}
}

// If AssigneeUsername is "me" and the searcher is logged in, changes
// AssigneeUsername to r.MaybeUserWithReadPermission.Username.
func replaceAssigneeUsernameIfEqualToMe(f *bugsearch.Filter, w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest) (ok bool) {
	if f.AssigneeUsername != bugsearch.MeUsername {
		return true
	}
	if !r.IsLoggedIn {
		http.Error(w, `"me" needs you to be logged in`, http.StatusBadRequest)
		return false
	}
	f.AssigneeUsername = r.MaybeUserWithReadPermission.Username
	return true
}
