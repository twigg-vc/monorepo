package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

const commitSearchPageSize = 25

func (hl handler) handleCommitSearch(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	params := r.Request.URL.Query()
	f, err := commitsearch.ParseQuery(r.Repo.Id,
		params.Get(routes.CommitSearchQueryParamName))
	if err != nil {
		// The message is what the search bar shows under itself.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	topCommit, err := hl.rSrv.GetRepoTopCommit(dbRead, r.Repo.Id)
	if err != nil {
		log.Printf("failed to get repo id=%d top commit: %s", r.Repo.Id, err)
		http.Error(w, "failed to get repo top commit",
			http.StatusInternalServerError)
		return
	}
	commits, nextCursor, err := hl.searchDb.SearchCommits(dbRead, f,
		params.Get(routes.CommitSearchCursorParamName), commitSearchPageSize)
	if err != nil {
		log.Printf("failed to search the commits of repo id=%d: %s", r.Repo.Id, err)
		http.Error(w, "failed to search the commits",
			http.StatusInternalServerError)
		return
	}
	cr := newCommitRenderer(hl.userSrv, hl.revSrv, r, topCommit.ServerL, dbRead, w)
	frontendCommits, ok := cr.renderAll(commits)
	if !ok {
		return
	}

	resp := CommitSearchResponse{
		Commits:    frontendCommits,
		NextCursor: nextCursor,
	}
	respJson, err := json.Marshal(resp)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal the search results: %s", err))
	}
	_, err = w.Write(respJson)
	if err != nil {
		log.Printf("failed to write the search results: %s", err)
	}
}
