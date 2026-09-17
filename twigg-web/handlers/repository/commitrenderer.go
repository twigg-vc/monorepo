package repository

import (
	"context"
	"log"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

// Helper that enriches the commits of one request into what the frontend
// expects. Not thread safe.
type commitRenderer struct {
	userSrv                 UserService
	revSrv                  ReviewService
	r                       wrappers.UserWithReadPermissionMuxRequest
	dbRead                  context.Context
	w                       http.ResponseWriter
	cachedUsernamesById     map[int64]string
	cachedSupremeLeaders    []string
	hasCachedSupremeLeaders bool
}

func newCommitRenderer(userSrv UserService, revSrv ReviewService,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context,
	w http.ResponseWriter) *commitRenderer {
	return &commitRenderer{
		userSrv:             userSrv,
		revSrv:              revSrv,
		r:                   r,
		dbRead:              dbRead,
		w:                   w,
		cachedUsernamesById: map[int64]string{},
	}
}

// On any error, writes an error to the response and returns ok=false.
func (cr *commitRenderer) getAuthorUsername(authorId int64) (username string, ok bool) {
	username, ok = cr.cachedUsernamesById[authorId]
	if ok {
		return username, true
	}
	author, isNotFoundErr, err := cr.userSrv.Get(cr.dbRead, authorId)
	if isNotFoundErr {
		http.Error(cr.w, "commit author not found", http.StatusNotFound)
		return "", false
	}
	if err != nil {
		log.Printf("failed to get commit author %d: %s", authorId, err)
		http.Error(cr.w, "failed to get commit author",
			http.StatusInternalServerError)
		return "", false
	}
	cr.cachedUsernamesById[authorId] = author.Username
	return author.Username, true
}

// On any error, writes an error to the response and returns ok=false.
func (cr *commitRenderer) getSupremeLeaders() (supremeLeaders []string, ok bool) {
	if cr.hasCachedSupremeLeaders {
		return cr.cachedSupremeLeaders, true
	}
	supremeLeaders, err := cr.revSrv.ResolveSupremeLeaders(cr.dbRead, cr.r.RepoOwnerUsr)
	if err != nil {
		log.Printf("failed to resolve the supreme leaders of %s: %s",
			cr.r.RepoOwnerUsr.Username, err)
		http.Error(cr.w, "internal err resolving supreme leaders",
			http.StatusInternalServerError)
		return nil, false
	}
	cr.cachedSupremeLeaders = supremeLeaders
	cr.hasCachedSupremeLeaders = true
	return supremeLeaders, true
}
