package bugs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

const bugsPageSize = 25

type handler struct {
	db Db
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
	frontendBugs, ok := newUsernames(h.db, dbRead, w).getFrontendBugs(bugs)
	if !ok {
		return
	}

	respJson, err := json.Marshal(GetBugsResponse{Bugs: frontendBugs, NextCursor: nextCursor})
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the bugs: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the bugs: %s", err)
	}
}
