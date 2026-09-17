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
	if !r.Flags.SearchCommitsUi {
		http.NotFound(w, r.Request)
		return
	}
	params := r.Request.URL.Query()
	f, err := commitsearch.ParseQuery(r.Repo.Id,
		params.Get(routes.CommitSearchQueryParamName))
	if err != nil {
		// The message is what the search bar shows under itself.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ok := replaceTheSearcherUsername(&f, w, r)
	if !ok {
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

// A search says "me" for whoever is searching, which only the request knows.
// On any error, writes an error to the response and returns ok=false.
func replaceTheSearcherUsername(f *commitsearch.Filter, w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest) (ok bool) {
	if f.AuthorUsername != commitsearch.MeUsername &&
		f.ReviewerUsername != commitsearch.MeUsername {
		return true
	}
	if !r.IsLoggedIn {
		http.Error(w, `"me" needs you to be logged in`, http.StatusBadRequest)
		return false
	}
	username := r.MaybeUserWithReadPermission.Username
	if f.AuthorUsername == commitsearch.MeUsername {
		f.AuthorUsername = username
	}
	if f.ReviewerUsername == commitsearch.MeUsername {
		f.ReviewerUsername = username
	}
	return true
}
