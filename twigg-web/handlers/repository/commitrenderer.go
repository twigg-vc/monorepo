package repository

import (
	"context"
	"log"
	"monorepo/base/iterator"
	"monorepo/twigg-web/review"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"net/http"
	"strings"
)

// Helper that enriches the commits of one request into what the frontend
// expects. Not thread safe.
type commitRenderer struct {
	userSrv                 UserService
	revSrv                  ReviewService
	r                       wrappers.UserWithReadPermissionMuxRequest
	repoTopServerId         commit.LocalId
	dbRead                  context.Context
	w                       http.ResponseWriter
	cachedUsernamesById     map[int64]string
	cachedSupremeLeaders    []string
	hasCachedSupremeLeaders bool
}

func newCommitRenderer(userSrv UserService, revSrv ReviewService,
	r wrappers.UserWithReadPermissionMuxRequest,
	repoTopServerId commit.LocalId, dbRead context.Context,
	w http.ResponseWriter) *commitRenderer {
	return &commitRenderer{
		userSrv:             userSrv,
		revSrv:              revSrv,
		r:                   r,
		repoTopServerId:     repoTopServerId,
		dbRead:              dbRead,
		w:                   w,
		cachedUsernamesById: map[int64]string{},
	}
}

func commitIsArchived(c commit.Commit) bool {
	return strings.HasPrefix(c.Message, msgPrefixToHidePendingCommit)
}

// Renders at most pageSize commits of the iterator, skipping the ones for
// which filterOutFunc=true (filterOutFunc can be nil).
// hasMore is true when the iterator holds more than
// pageSize commits. On any error, writes an error to the response and
// returns ok=false.
func (cr *commitRenderer) renderCommits(commits iterator.I[commit.Commit],
	pageSize int, filterOutFunc func(c commit.Commit) bool) (
	fcs []twiggwc.FrontendCommit, hasMore bool, ok bool) {
	fcs = make([]twiggwc.FrontendCommit, 0, pageSize)
	for commits.Next() {
		if len(fcs) >= pageSize {
			hasMore = true
			break
		}
		c, err := commits.Get()
		if err != nil {
			log.Printf("failed to get a commit of the page: %s", err)
			http.Error(cr.w, "failed to get commit", http.StatusInternalServerError)
			return nil, false, false
		}
		if filterOutFunc != nil && filterOutFunc(c) {
			continue
		}
		fc, ok := cr.render(c)
		if !ok {
			return nil, false, false
		}
		fcs = append(fcs, fc)
	}
	err := commits.Err()
	if err != nil {
		log.Printf("failed to iterate on the commits of the page: %s", err)
		http.Error(cr.w, "failed to iterate on commits",
			http.StatusInternalServerError)
		return nil, false, false
	}
	return fcs, hasMore, true
}

// The root commit has no author and submitted commits are always
// ReviewStatus_Ready.
// On any error, writes an error to the response and returns ok=false.
func (cr *commitRenderer) render(c commit.Commit) (fc twiggwc.FrontendCommit, ok bool) {
	var authorUsername string
	if c.L != 0 {
		authorUsername, ok = cr.getAuthorUsername(c.AuthorUserId)
		if !ok {
			return fc, false
		}
	}
	reviewStatus := review.ReviewStatus_Ready
	if !c.IsSubmitted {
		reviewStatus, ok = cr.getReviewStatus(c.L)
		if !ok {
			return fc, false
		}
	}
	return twiggwc.CommitToFrontend(c, authorUsername, reviewStatus), true
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

// On any error, writes an error to the response and returns ok=false.
func (cr *commitRenderer) getReviewStatus(cId commit.LocalId) (
	s review.ReviewStatus, ok bool) {
	supremeLeaders, ok := cr.getSupremeLeaders()
	if !ok {
		return s, false
	}
	// isNotFound errors are ignored bc they mean the data was not saved yet.
	// The returned reviewData will have a valid review status.
	d, isNotFoundErr, err := cr.revSrv.GetData(
		cr.dbRead, cr.r.Repo.Id, cId,
		/*checkOwners=*/ true,
		/*cIdToReadOwners=*/ cr.repoTopServerId,
		supremeLeaders)
	if err != nil && !isNotFoundErr {
		log.Printf("failed to get the review data of commit %d: %s", cId, err)
		http.Error(cr.w, "failed to get review data",
			http.StatusInternalServerError)
		return s, false
	}
	return d.ReviewStatus, true
}
