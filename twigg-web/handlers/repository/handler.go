package repository

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/cacheheaders"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/routes"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
)

//go:embed docsify
var docsify embed.FS

type handler struct {
	rSrv    RepoService
	revSrv  ReviewService
	userSrv UserService
}

const msgPrefixToHidePendingCommit = "#ARCHIVED"
const maxPendingCommitsPageSize = 20

func (hl handler) handleGet(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	topComit, err := hl.rSrv.GetRepoTopCommit(dbRead, r.Repo.Id)
	if err != nil {
		http.Error(w, "failed to get repo top commit",
			http.StatusInternalServerError)
		return
	}
	submittedFrontendCommits, ok := hl.getSubmittedCommits(
		topComit.ServerL, topComit, w, r, dbRead)
	if !ok {
		return
	}

	pending, err := hl.rSrv.GetRepoPendingCommits(dbRead, r.Repo.Id /*ascending=*/, false)
	if err != nil {
		http.Error(w, "failed to get pending commits",
			http.StatusInternalServerError)
		return
	}
	pendingFrontendCommit := []twiggwc.FrontendCommit{}
	var haveMorePendingCommitsToFetch bool
	for pending.Next() {
		if len(pendingFrontendCommit) >= maxPendingCommitsPageSize {
			haveMorePendingCommitsToFetch = true
			break
		}
		c, err := pending.Get()
		if err != nil {
			http.Error(w, "failed to get pending commit",
				http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(c.Message, msgPrefixToHidePendingCommit) {
			continue
		}
		fc, ok := hl.getFrontendCommit(topComit.ServerL, c, dbRead, w, r)
		if !ok {
			return
		}
		pendingFrontendCommit = append(pendingFrontendCommit, fc)
	}
	err = pending.Err()
	if err != nil {
		http.Error(w, "failed to iterate on pending commits",
			http.StatusInternalServerError)
		return
	}

	twiggwc.Page(
		/*hideNavBar=*/ false,
		r.Flags,
		twiggwc.RepoDisplay(
			r.RepoOwnerUsr.Username, r.Repo.DisplayName, r.Repo.Description,
			pendingFrontendCommit, submittedFrontendCommits,
			haveMorePendingCommitsToFetch)).Render(w)

}

func (hl handler) handleGetMoreSubmitted(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	const startAtCommitParameterName = "starting-at"
	startS := r.Request.URL.Query().Get(startAtCommitParameterName)
	start, err := strconv.ParseUint(startS, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad commit %s", startS), http.StatusBadRequest)
		return
	}
	topComit, err := hl.rSrv.GetRepoTopCommit(dbRead, r.Repo.Id)
	if err != nil {
		http.Error(w, "failed to get repo top commit",
			http.StatusInternalServerError)
		return
	}

	c, err := hl.rSrv.GetRepoCommit(dbRead, r.Repo.Id, start)
	if err != nil {
		log.Printf("failed to get commit %d: %s", start, err)
		http.Error(w, "failed to read commit", http.StatusBadRequest)
		return
	}
	submitted, ok := hl.getSubmittedCommits(topComit.ServerL, c, w, r, dbRead)
	if !ok {
		return
	}
	submittedJson, err := json.Marshal(submitted)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal submitted:%s", err))
	}
	_, err = w.Write(submittedJson)
	if err != nil {
		log.Printf("failed to write more submitted commits: %s", err)
		http.Error(w, "failed to write commits", http.StatusBadRequest)
		return
	}
}

func (hl handler) handleGetMorePending(w http.ResponseWriter, r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	const afterCommitParameterName = "after-commit"
	afterCommitStr := r.Request.URL.Query().Get(afterCommitParameterName)
	afterCommit, err := strconv.ParseUint(afterCommitStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad after-commit param %s", afterCommitStr), http.StatusBadRequest)
		return
	}
	topCommit, err := hl.rSrv.GetRepoTopCommit(dbRead, r.Repo.Id)
	if err != nil {
		log.Printf("failed to get repo id=%d top commit: %s", r.Repo.Id, err)
		http.Error(w, "failed to get repo top commit", http.StatusInternalServerError)
		return
	}
	pending, err := hl.rSrv.GetRepoPendingCommitsAfter(dbRead, r.Repo.Id, afterCommit)
	if err != nil {
		log.Printf("failed to get pending commits: %s", err)
		http.Error(w, "failed to get pending commits", http.StatusInternalServerError)
		return
	}
	pendingFrontendCommits := make([]twiggwc.FrontendCommit, 0, maxPendingCommitsPageSize)
	var haveMorePendingCommitsToFetch bool
	for pending.Next() {
		if len(pendingFrontendCommits) >= maxPendingCommitsPageSize {
			haveMorePendingCommitsToFetch = true
			break
		}
		c, err := pending.Get()
		if err != nil {
			log.Printf("failed to get pending commit: %s", err)
			http.Error(w, "failed to get pending commit", http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(c.Message, msgPrefixToHidePendingCommit) {
			continue
		}
		fc, ok := hl.getFrontendCommit(topCommit.ServerL, c, dbRead, w, r)
		if !ok {
			return
		}
		pendingFrontendCommits = append(pendingFrontendCommits, fc)
	}
	err = pending.Err()
	if err != nil {
		log.Printf("failed to iterate on pending commits: %s", err)
		http.Error(w, "failed to iterate on pending commits", http.StatusInternalServerError)
		return
	}

	resp := getMorePendingResponse{
		PendingFrontendCommits:        pendingFrontendCommits,
		HaveMorePendingCommitsToFetch: haveMorePendingCommitsToFetch,
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal response: %s", err))
	}

	_, err = w.Write(respJSON)
	if err != nil {
		log.Printf("failed to write more pending commits: %s", err)
		http.Error(w, "failed to write commits", http.StatusInternalServerError)
		return
	}
}

type getMorePendingResponse struct {
	PendingFrontendCommits        []twiggwc.FrontendCommit
	HaveMorePendingCommitsToFetch bool
}

func (hl handler) handleGetTwiggDoc(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	if !strings.HasSuffix(r.URL.Path, ".md") {
		cacheheaders.LongCache(w)
		http.ServeFileFS(w, r.Request, docsify, "docsify/index.html")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] != "docs" {
		http.NotFound(w, r.Request)
		return
	}
	filePath := path.Join(parts[4:]...)

	var targetCommit commit.Commit
	c := r.URL.Query().Get("c")
	if c != "" {
		cId, err := strconv.ParseUint(c, 10, 64)
		if err != nil {
			http.Error(w, "bad commit id", http.StatusBadRequest)
			return
		}
		targetCommit, err = hl.rSrv.GetRepoCommit(dbRead, r.Repo.Id, cId)
		if err != nil {
			// TODO: differentiate error and err not found
			http.Error(w, "failed to get commit",
				http.StatusInternalServerError)
			return
		}
	} else {
		var err error
		targetCommit, err = hl.rSrv.GetRepoTopCommit(dbRead, r.Repo.Id)
		if err != nil {
			http.Error(w, "failed to get commit", http.StatusInternalServerError)
			return
		}
	}
	_ = hl.rSrv.GetRepoFile(dbRead, r.Repo.Id, targetCommit, filePath, w)
}

func (hl handler) HandleSearchCommits(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	hasCid, cId, hasCv, cV, ok := hl.parseHandleSearchCommitsQuery(w, r)
	if !ok {
		return
	}

	topComit, err := hl.rSrv.GetRepoTopCommit(dbRead, r.Repo.Id)
	if err != nil {
		http.Error(w, "failed to get repo top commit", http.StatusInternalServerError)
		return
	}
	var commits []twiggwc.FrontendCommit
	if hasCid {
		var c commit.Commit
		if hasCv {
			c, err = hl.rSrv.GetRepoCommitVersion(dbRead, r.Repo.Id, cId, cV)
			if err != nil {
				log.Printf("failed to get commit verssion: %s", err)
				http.Error(w, "failed to get commit", http.StatusInternalServerError)
				return
			}
		} else {
			c, err = hl.rSrv.GetRepoCommit(dbRead, r.Repo.Id, cId)
			if err != nil {
				log.Printf("failed to get commit commit: %s", err)
				http.Error(w, "failed to get commit version", http.StatusInternalServerError)
				return
			}
		}
		fc, ok := hl.getFrontendCommit(topComit.ServerL, c, dbRead, w, r)
		if !ok {
			return
		}
		commits = []twiggwc.FrontendCommit{fc}
	} else {
		var ok bool
		commits, ok = hl.getSubmittedCommits(topComit.ServerL, topComit, w, r, dbRead)
		if !ok {
			return
		}
	}
	commitsJson, err := json.Marshal(commits)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal commits:%s", err))
	}
	_, err = w.Write(commitsJson)
	if err != nil {
		log.Printf("failed to write more searched commits: %s", err)
		http.Error(w, "failed to write commits", http.StatusBadRequest)
		return
	}
}

// On any error, writes an error response and returns ok=false
// Regex breakdown:
// ^(?:c/|c)?    -> Optional prefix 'c/' or 'c'
// (?P<id>\d+)   -> Required digits for Commit ID
// (?:v(?P<v>\d+))? -> Optional 'v' followed by digits for Version
// $             -> End of string
var searchCommitQueryRegex = regexp.MustCompile(`^(?:c/|c)?(?P<id>\d+)(?:v(?P<v>\d+))?$`)

func (hl handler) parseHandleSearchCommitsQuery(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest) (
	hasCommitId bool, commitId uint64, hasCommitVersion bool, commitVersion uint64, ok bool) {

	query := r.Request.URL.Query().Get(routes.RepoSearchCommitsSeachQueryQueryParamName)
	if query == "" {
		return false, 0, false, 0, true
	}
	matches := searchCommitQueryRegex.FindStringSubmatch(query)
	if matches == nil {
		http.Error(w, fmt.Sprintf("invalid search format: %s", query), http.StatusBadRequest)
		return false, 0, false, 0, false
	}
	// Parse Commit ID (Captured in group 1)
	id, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		// This case is unlikely if the regex passes, but good for safety
		http.Error(w, "invalid commit id", http.StatusBadRequest)
		return false, 0, false, 0, false
	}
	// Parse Version (Captured in group 2) if it exists
	if matches[2] != "" {
		v, err := strconv.ParseUint(matches[2], 10, 64)
		if err != nil {
			http.Error(w, "invalid version", http.StatusBadRequest)
			return false, 0, false, 0, false
		}
		return true, id, true, v, true
	}
	return true, id, false, 0, true
}

// Helper to read the additional data required by the frontend commit.
// Always returns ReviewStatus_Ready status for submitted commits.
// On any error, writes an error to the response and returns ok=false.
func (hl handler) getFrontendCommit(repoTopServerId commit.LocalId,
	c commit.Commit,
	dbRead context.Context, w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest) (fc twiggwc.FrontendCommit, ok bool) {
	var commitAuthorUsername string
	if c.L == 0 {
		commitAuthorUsername = ""
	} else {
		commitAuthor, _, err := hl.userSrv.Get(dbRead, c.AuthorUserId)
		if err != nil {
			http.Error(w, "failed to get commit author",
				http.StatusInternalServerError)
			return
		}
		commitAuthorUsername = commitAuthor.Username
	}
	var revStatus review.ReviewStatus
	if c.IsSubmitted {
		revStatus = review.ReviewStatus_Ready
	} else {
		// isNotFound errors are ignored bc they mean the data was not saved yet.
		// The returned reviewData will have a valid review status.
		supremeLeaders, err := hl.revSrv.ResolveSupremeLeaders(dbRead, r.RepoOwnerUsr)
		if err != nil {
			http.Error(w, "internal err resolving supreme leaders", http.StatusInternalServerError)
			return
		}
		latestCommitReviewData, isNotFoundErr, err := hl.revSrv.GetData(
			dbRead, r.Repo.Id, c.L,
			/*checkOwners=*/ true,
			/*cIdToCheckOwners*/ repoTopServerId,
			supremeLeaders)
		if err != nil && !isNotFoundErr {
			http.Error(w, "failed to get review data", http.StatusInternalServerError)
			return
		}
		revStatus = latestCommitReviewData.ReviewStatus
	}
	ok = true
	return twiggwc.CommitToFrontend(c, commitAuthorUsername, revStatus), true
}

// Helper to get submitted commits startingAt (inclusive)
func (hl handler) getSubmittedCommits(repoTopServerId commit.LocalId, startingAt commit.Commit, w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) (submittedFrontendCommits []twiggwc.FrontendCommit, ok bool) {
	const maxSubmittedCommitsToShow = 10
	fc, ok := hl.getFrontendCommit(repoTopServerId, startingAt, dbRead, w, r)
	if !ok {
		return
	}
	submittedFrontendCommits = []twiggwc.FrontendCommit{fc}

	for len(submittedFrontendCommits) <= maxSubmittedCommitsToShow &&
		submittedFrontendCommits[len(submittedFrontendCommits)-1].L > 0 {
		var c commit.Commit
		var err error
		c, err = hl.rSrv.GetRepoCommit(
			dbRead,
			r.Repo.Id,
			submittedFrontendCommits[len(submittedFrontendCommits)-1].ParentL,
		)
		if err != nil {
			http.Error(w, "failed to get submitted commit",
				http.StatusInternalServerError)
			return
		}
		fc, ok = hl.getFrontendCommit(repoTopServerId, c, dbRead, w, r)
		if !ok {
			return
		}
		submittedFrontendCommits = append(submittedFrontendCommits, fc)
	}
	ok = true
	return
}