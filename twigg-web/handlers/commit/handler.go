package commit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"monorepo/base/iterator"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/handlers/reposettings"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/cicdpublisher"
	jobsservice "monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/services/repo"
	reviewservice "monorepo/twigg-web/services/review"
	userservice "monorepo/twigg-web/services/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
	"monorepo/twigg/tree"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type handler struct {
	db          Db
	rSrv        repo.Service
	revSrv      reviewservice.Service
	rt          routes.Router
	userS       userservice.Service
	jobsS       jobsservice.Service
	ciq         CiCdQueue
	parser      Parser
	trackClient TrackClient
	userRepoMux wrappers.UserRepoMux
	queue       Queue
	canSubCache CanSubmitCache
	configName  string
}

func quotaOwner(userId int64) string {
	return strconv.FormatInt(userId, 10)
}

func (hl handler) handleGetCommit(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	s, isNotFoundErr, err := hl.rSrv.GetServer(
		dbRead, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	serverRead := hl.rSrv.GetServerRead(dbRead)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusInternalServerError)
		return
	}
	if err != nil {
		log.Printf("error getting srv for ownerId: %d and displayName: %q in handleGetCommit: %q", r.RepoOwnerUsr.Id, r.Repo.DisplayName, err)
		http.Error(w, "internal error getting srv",
			http.StatusInternalServerError)
		return
	}
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	if cI == 0 {
		http.Error(w, "can't get root commit", http.StatusBadRequest)
		return
	}
	// Read the latest commit version, the parent, and then all versions
	latestVersion, err := s.GetLatest(cI, serverRead)
	if err != nil {
		log.Printf("error getting commit latest version for commit id: %d in handleGetCommit: %q", cI, err)
		http.Error(w, "internal error getting commit",
			http.StatusInternalServerError)
		return
	}
	parentOfLatestVersion, err := s.GetVersion(latestVersion.ParentL,
		latestVersion.ParentV, serverRead)
	if err != nil {
		http.Error(w, "internal error getting parent",
			http.StatusInternalServerError)
		return
	}
	latestParent, err := s.GetLatest(latestVersion.ParentL, serverRead)
	if err != nil {
		http.Error(w, "internal error getting latest parent",
			http.StatusInternalServerError)
		return
	}

	// 1 for each version (version starts at zero)
	cVersions := make([]commit.Commit, 0, latestVersion.Version+1)
	cVersionsParents := make([]commit.Commit, 0, latestVersion.Version+1)
	v := uint64(0)
	const maxCommitVersions = 100
	for v < latestVersion.Version {
		if v > maxCommitVersions {
			http.Error(w, "too many commit versions",
				http.StatusInternalServerError)
			return
		}
		var pastVersion commit.Commit
		pastVersion, err = s.GetVersion(cI, v, serverRead)
		if err != nil {
			http.Error(w, "internal error commit version",
				http.StatusInternalServerError)
			return
		}
		var parentOfPastVersion commit.Commit
		parentOfPastVersion, err = s.GetVersion(pastVersion.ParentL,
			pastVersion.ParentV, serverRead)
		if err != nil {
			http.Error(w, "internal error commit version",
				http.StatusInternalServerError)
			return
		}
		cVersions = append(cVersions, pastVersion)
		cVersionsParents = append(cVersionsParents, parentOfPastVersion)
		v++
	}
	cVersions = append(cVersions, latestVersion)
	if uint64(len(cVersions)) != latestVersion.Version+1 {
		panic("I'm bad at math")
	}
	cVersionsParents = append(cVersionsParents, parentOfLatestVersion)
	if uint64(len(cVersionsParents)) != latestVersion.Version+1 {
		panic("I'm bad at math")
	}

	// children are the commits local number whose
	// the latest version has this commit (cI) as parent
	var children []commit.LocalId
	for _, cVersion := range cVersions {
		var cVersionChildrenIds []commit.LocalId

		const maxCommitChildren = 100
		cVersionChildrenIds, _, err = serverRead.GetCommitChildren(r.Repo.Id,
			uint64(cVersion.L), cVersion.Version, maxCommitChildren)
		if err != nil {
			log.Printf("error getting children of commit %dv%d in handleGetCommit: %q", cVersion.L, cVersion.Version, err)
			http.Error(w, "internal error getting commit children", http.StatusInternalServerError)
			return
		}

		for _, childL := range cVersionChildrenIds {
			if len(children) > maxCommitChildren {
				break
			}
			if slices.Contains(children, childL) {
				continue
			}

			// Checks if the childL still have the cI as parent
			var latestChild commit.Commit
			latestChild, err = s.GetLatest(childL, serverRead)
			if err != nil {
				log.Printf("error getting latest version of child %d in handleGetCommit: %q", childL, err)
				http.Error(w, "internal error getting commit child",
					http.StatusInternalServerError)
				return
			}
			if latestChild.ParentL != cI {
				continue
			}

			children = append(children, childL)
		}
	}

	// Get the left and right commit to compute the diff
	hasRight, rightV := hl.rt.GetRightVersionParameter(r.Request)
	var right commit.Commit
	var rightI int
	if !hasRight {
		// Right is, by default, the latest version
		rightI = len(cVersions) - 1
		right = cVersions[rightI]
	} else {
		found := false
		for i, c := range cVersions {
			if c.Version == rightV {
				right = c
				rightI = i
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "invalid right version",
				http.StatusBadRequest)
			return
		}
	}
	hasLeft, leftV := hl.rt.GetLeftVersionParameter(r.Request)
	var left commit.Commit
	if !hasLeft {
		// Left is, by default, the parent of right
		left = cVersionsParents[rightI]
	} else {
		found := false
		for _, c := range cVersions {
			if c.Version == leftV {
				left = c
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "invalid left version",
				http.StatusBadRequest)
			return
		}
	}

	// Compute the submit status
	var submitWouldConflict bool
	submitWouldConflict, err = func() (bool, error) {
		can, reason, err := hl.getAndCacheCanSubmit(latestVersion, serverRead, s)
		if err != nil {
			return false, err
		}
		return !can && reason == server.CantSubmitWouldCauseRebaseConflict, nil
	}()
	if err != nil {
		http.Error(w, "internal err computing submit status: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	// Maps names of files displayed to the URL that will return the diff
	filenameToGetDiffUrl := make(map[string]string)
	// Maps filename to one of the status defined above (c, m, d)
	const createdStatus = "c"
	const deletedStatus = "d"
	const modifiedInAnyWayStatus = "m"
	filenameToStatus := make(map[string]string)
	tooManyDiffs := false
	diffs, err := s.Diff(right, left, serverRead)
	if err != nil {
		http.Error(w, "internal err getting diffs",
			http.StatusInternalServerError)
		return
	}
	const maxDiffs = 100
	for diffs.CanGet() {
		diff := diffs.GetDiff()
		if diff.Type == tree.DiffTypeUndefined {
			err = diffs.Next()
			if err != nil {
				http.Error(w, "internal err getting next diff",
					http.StatusInternalServerError)
				return
			}
			continue
		}

		var tr tree.Tree
		var trPath string
		trPath, _, _, tr = diffs.Get()
		// Data should always be fully known
		if !tr.DataIsComplete() {
			panic("diff tree not known")
		}
		if diff.Type == tree.DiffTypeNoChange {
			diffs.SkipChildrenOnNext()
			err = diffs.Next()
			if err != nil {
				http.Error(w, "internal err getting next diff",
					http.StatusInternalServerError)
				return
			}
			continue
		}
		// Hide directories; just show file diffs
		if diff.Data.IsDir {
			err = diffs.Next()
			if err != nil {
				http.Error(w, "internal err getting next diff",
					http.StatusInternalServerError)
				return
			}
			continue
		}

		// Status is either created, deleted or modified.
		// I.e. all kinds of modifications are put in the "modified" status
		// for the frontend
		if diff.Type != tree.DiffTypeCreated &&
			diff.Type != tree.DiffTypeDeleted {
			filenameToStatus[trPath] = modifiedInAnyWayStatus
		} else {
			switch diff.Type {
			case tree.DiffTypeCreated:
				filenameToStatus[trPath] = createdStatus
			case tree.DiffTypeDeleted:
				filenameToStatus[trPath] = deletedStatus
			default:
				// Cant ever happen due to the if statement above
				panic("unexpected diff type")
			}
		}

		filenameToGetDiffUrl[trPath] = hl.rt.Diff(
			r.RepoOwnerUsr.Username, r.Repo.DisplayName, cI,
			hasLeft, leftV, hasRight, rightV,
			trPath)
		err = diffs.Next()
		if err != nil {
			http.Error(w, "internal err getting next diff",
				http.StatusInternalServerError)
			return
		}

		if len(filenameToGetDiffUrl) > maxDiffs {
			tooManyDiffs = true
			break
		}
	}

	// We must get the threads because files that have comments on them
	// must be displayed, even if they didn't change
	threads, err := hl.revSrv.GetThreads(dbRead, r.Repo.Id, cI,
		/*asennding*/ true)
	if err != nil {
		http.Error(w, "internal err getting threads",
			http.StatusInternalServerError)
		return
	}
	leftIsParent := left.L != right.L
	for threads.Next() {
		var th review.Thread
		th, err = threads.Get()
		if err != nil {
			http.Error(w, "internal err getting thread",
				http.StatusInternalServerError)
			return
		}
		if th.Type != review.ThreadType_CommentsOnFileOnCommitVersion {
			// Only look for threads that are comments on files
			continue
		}

		if (!leftIsParent && th.CommitVersion == left.Version) ||
			(th.CommitVersion == right.Version) {
			filenameToGetDiffUrl[th.Filename] = hl.rt.Diff(
				r.RepoOwnerUsr.Username, r.Repo.DisplayName, cI,
				hasLeft, leftV, hasRight, rightV,
				th.Filename)
		}

		if len(filenameToGetDiffUrl) > maxDiffs {
			tooManyDiffs = true
			break
		}
	}
	err = threads.Err()
	if err != nil {
		http.Error(w, "internal err iterating threads",
			http.StatusInternalServerError)
		return
	}

	type fileGetDiffUrlAndStatus struct {
		filename   string
		getDiffUrl string
		status     string
	}
	filesToShow := make([]fileGetDiffUrlAndStatus, 0, len(filenameToGetDiffUrl))
	for filename, getDiffUrl := range filenameToGetDiffUrl {
		filesToShow = append(filesToShow, fileGetDiffUrlAndStatus{
			filename:   filename,
			getDiffUrl: getDiffUrl,
			status:     filenameToStatus[filename],
		})
	}
	sort.Slice(filesToShow, func(i, j int) bool {
		return filesToShow[i].filename < filesToShow[j].filename
	})
	diffFileNames := make([]string, len(filesToShow))
	diffStatus := make([]string, len(filesToShow))
	diffUrls := make([]string, len(filesToShow))
	for i, f := range filesToShow {
		diffFileNames[i] = f.filename
		diffUrls[i] = f.getDiffUrl
		diffStatus[i] = f.status
	}

	// Get commit aditional data
	supremeLeaders, err := hl.revSrv.ResolveSupremeLeaders(dbRead, r.RepoOwnerUsr)
	if err != nil {
		http.Error(w, "internal err resolving supreme leaders",
			http.StatusInternalServerError)
		return
	}
	commitData, isNotFoundErr, err := hl.revSrv.GetData(dbRead, r.Repo.Id, cI,
		/*checkOwners*/ true, s.Top().ServerL, supremeLeaders)
	if err != nil && !isNotFoundErr {
		http.Error(w, "internal err getting commit data",
			http.StatusInternalServerError)
		return
	}
	if isNotFoundErr {
		commitData = review.Data{
			Description: "",
		}
	}
	tab := r.FormValue("tab")
	if tab != "feed" && tab != "changes" {
		tab = "feed"
	}

	commitAuthorUser, _, err := hl.userS.Get(dbRead, latestVersion.AuthorUserId)
	if err != nil {
		http.Error(w, "internal err getting user",
			http.StatusInternalServerError)
		return
	}
	var hasLgtmFromCurrentUser bool
	if r.IsLoggedIn {
		hasLgtmFromCurrentUser, err = hl.revSrv.HasLgtm(dbRead, r.Repo.Id, cI, r.MaybeUserWithReadPermission.Id)
		if err != nil {
			http.Error(w, "internal err gettting LGTM",
				http.StatusInternalServerError)
			return
		}
	}
	lgtmAuthorsIter, err := hl.revSrv.GetLgtmAuthors(dbRead, r.Repo.Id, cI)
	if err != nil {
		http.Error(w, "internal err getting LGTM authors",
			http.StatusInternalServerError)
		return
	}
	var lgtmAuthorUsernames []string
	const maxLgtmUsers = 100

	for lgtmAuthorsIter.Next() {
		userId, err := lgtmAuthorsIter.Get()
		if err != nil {
			http.Error(w, "internal err reading LGTM authors",
				http.StatusInternalServerError)
			return
		}
		u, _, err := hl.userS.Get(dbRead, userId)
		if err != nil {
			http.Error(w, "internal err getting LGTM author user",
				http.StatusInternalServerError)
			return
		}
		lgtmAuthorUsernames = append(lgtmAuthorUsernames, u.Username)
		if len(lgtmAuthorUsernames) >= maxLgtmUsers {
			http.Error(w, "internal err: too many LGTM authors",
				http.StatusInternalServerError)
			return
		}
	}
	if err := lgtmAuthorsIter.Err(); err != nil {
		http.Error(w, "internal err iterating LGTM authors",
			http.StatusInternalServerError)
		return
	}

	ciStatus, err := hl.ciq.GetCiCdLatestRunStatus(r.Repo.Id,
		latestVersion.L, latestVersion.Version, dbRead)
	if err != nil {
		log.Printf("failed to get analysis status: %s", err)
		http.Error(w, "internal err getting analysis status",
			http.StatusInternalServerError)
		return
	}

	twiggwc.PageWithTitle(
		fmt.Sprintf("c/%d", cI),
		/*hideNavBar=*/ false,
		r.Flags,
		twiggwc.CommitDisplay(
			r.RepoOwnerUsr.Username,
			r.Repo.DisplayName,
			cVersions,
			cVersionsParents,
			children,
			commitData.ReviewStatus,
			latestParent.IsSubmitted,
			submitWouldConflict,
			hasLgtmFromCurrentUser,
			lgtmAuthorUsernames,
			hl.rt.Submit(r.RepoOwnerUsr.Username, r.Repo.DisplayName, cI),
			hasLeft,
			leftV,
			hasRight,
			rightV,
			commitData.Description,
			hl.rt.Commit(r.RepoOwnerUsr.Username,
				r.Repo.DisplayName, cI, hasLeft, leftV, hasRight, rightV),
			diffFileNames, diffStatus, diffUrls, tooManyDiffs, tab,
			commitAuthorUser,
			r.MaybeUserWithReadPermission,
			ciStatus),
	).Render(w)
}
func (hl handler) handleGetCanSubmitCommits(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	s, isNotFoundErr, err := hl.rSrv.GetServer(
		dbRead, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	serverRead := hl.rSrv.GetServerRead(dbRead)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusInternalServerError)
		return
	}
	if err != nil {
		log.Printf("error getting srv for ownerId: %d and displayName: %q in handleGetCanSubmitCommits: %q", r.RepoOwnerUsr.Id, r.Repo.DisplayName, err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}

	commitIdStrs := r.Request.URL.Query()["c"]
	if len(commitIdStrs) == 0 {
		http.Error(w, "no commits in the request", http.StatusBadRequest)
		return
	}
	if len(commitIdStrs) > 20 {
		http.Error(w, "too many commits in the request", http.StatusBadRequest)
		return
	}

	responses := make(HandleGetCanSubmitCommitsResponse, len(commitIdStrs))
	for _, cStr := range commitIdStrs {
		cI, err := strconv.ParseUint(cStr, 10, 64)
		if err != nil {
			http.Error(w, "bad commit id format", http.StatusBadRequest)
			return
		}
		if cI == 0 {
			http.Error(w, "can't get root commit", http.StatusBadRequest)
			return
		}
		latestVersion, err := s.GetLatest(commit.LocalId(cI), serverRead)
		if errors.Is(err, server.ErrNotFound) {
			http.Error(w, "commit not found", http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Printf("error getting commit latest version for commit id: %d in handleGetCanSubmitCommits: %q", cI, err)
			http.Error(w, "internal error getting commit", http.StatusInternalServerError)
			return
		}
		canSub, cantSubReasonEnum, err := hl.getAndCacheCanSubmit(latestVersion, serverRead, s)
		if err != nil {
			log.Printf("internal error checking can submit: %q", err)
			http.Error(w, "internal error checking can submit", http.StatusInternalServerError)
			return
		}
		var cantSubReason string
		switch cantSubReasonEnum {
		case server.CantSubmitAlreadySubmitted:
			cantSubReason = "already-submitted"
		case server.CantSubmitWithConflict:
			cantSubReason = "has-conflict"
		case server.CantSubmitBeforeParent:
			cantSubReason = "parent-not-submitted"
		case server.CantSubmitWouldCauseRebaseConflict:
			cantSubReason = "would-cause-rebase-conflict"
		default:
			cantSubReason = "unknown"
		}
		responses[cStr] = CanSubmitResult{canSub, cantSubReason}
	}

	err = json.NewEncoder(w).Encode(responses)
	if err != nil {
		log.Printf("error encoding can submit commits response: %q", err)
		http.Error(w, "internal error encoding response", http.StatusInternalServerError)
		return
	}
}

func (hl handler) getAndCacheCanSubmit(latestVersion commit.Commit, serverRead server.Read, s server.Server) (canSub bool, cantSubReasonEnum server.CantSubmitReason, err error) {
	cacheFound := false
	if hl.canSubCache != nil {
		canSub, cantSubReasonEnum, cacheFound = hl.canSubCache.GetCanSubmit(latestVersion.ServerL, latestVersion.ServerV, s.Top().ServerL)
		if cacheFound {
			return
		}
	}
	canSub, cantSubReasonEnum, err = s.CanSubmit(latestVersion, serverRead)
	if err != nil {
		return
	}
	if !cacheFound && hl.canSubCache != nil {
		hl.canSubCache.PutCanSubmit(latestVersion.ServerL, latestVersion.ServerV, s.Top().ServerL, canSub, cantSubReasonEnum)
	}
	return
}

func (hl handler) handleGetReviewData(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	s, err := hl.rSrv.GetServerByRepoId(dbRead, r.Repo.Id)
	if err != nil {
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	supremeLeaders, err := hl.revSrv.ResolveSupremeLeaders(dbRead, r.RepoOwnerUsr)
	if err != nil {
		http.Error(w, "internal err resolving supreme leaders", http.StatusInternalServerError)
		return
	}
	d, isNotFoundErr, err := hl.revSrv.GetData(dbRead, r.Repo.Id, cI,
		/*checkOwners*/ true, s.Top().ServerL, supremeLeaders)
	if err != nil && !isNotFoundErr {
		http.Error(w, "err getting commit data", http.StatusInternalServerError)
		return
	}

	type ReviewData struct {
		Description  string
		ReviewStatus string
	}
	rd := ReviewData{
		Description:  d.Description,
		ReviewStatus: twiggwc.ReviewStatusString(d.ReviewStatus),
	}
	encoder := json.NewEncoder(w)
	err = encoder.Encode(rd)
	if err != nil && !isNotFoundErr {
		http.Error(w, "err writing data", http.StatusInternalServerError)
		return
	}
}

// Returns all comments of all threads (in order)
func (hl handler) handleGetComments(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	var err error
	errMsg := ""
	defer func() {
		if err != nil && errMsg == "" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if errMsg != "" {
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}()

	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		errMsg = "bad commit format"
		return
	}

	threads, err := hl.revSrv.GetThreads(dbRead, r.Repo.Id, cI,
		/*asennding*/ true)
	if err != nil {
		errMsg = "err getting all threads"
		return
	}

	_, err = w.Write([]byte("["))
	if err != nil {
		errMsg = "err opening stream"
		return
	}
	encoder := json.NewEncoder(w)
	first := true
	for threads.Next() {
		var thread review.Thread
		thread, err = threads.Get()
		if err != nil {
			errMsg = "err getting thread"
			return
		}
		// Lgtm "threads" don't have comments
		if thread.Type == review.ThreadType_AddLGTM ||
			thread.Type == review.ThreadType_RemoveLGTM {
			continue
		}

		if !first {
			w.Write([]byte(","))
		}
		first = false
		ok := hl.writeCommitsOfThread(dbRead, r.Repo.Id, cI, thread.Id, w, encoder)
		if !ok {
			return
		}
	}
	err = threads.Err()
	if err != nil {
		errMsg = "err iterating on threads"
		return
	}

	_, err = w.Write([]byte("]"))
	if err != nil {
		errMsg = "err closing stream"
		return
	}

}

func (hl handler) writeCommitsOfThread(dbRead context.Context, repoId uint64, cId uint64, threadId int64, w http.ResponseWriter, js *json.Encoder) (ok bool) {
	comments, err := hl.revSrv.GetComments(dbRead, repoId, cId, threadId)
	if err != nil {
		http.Error(w, "err getting comments", http.StatusInternalServerError)
		return
	}
	first := true
	for comments.Next() {
		cm, err := comments.Get()
		if err != nil {
			http.Error(w, "err getting comment", http.StatusInternalServerError)
			return
		}
		if !first {
			w.Write([]byte(","))
		}
		first = false

		u, _, err := hl.userS.Get(dbRead, cm.AuthorUserId)
		if err != nil {
			http.Error(w, "err getting user", http.StatusInternalServerError)
			return
		}
		err = js.Encode(newFrontendComment(cm, u.Username))
		if err != nil {
			http.Error(w, "err writing comment", http.StatusInternalServerError)
			return
		}
	}
	err = comments.Err()
	if err != nil {
		http.Error(w, "err iterating", http.StatusInternalServerError)
		return
	}
	ok = true
	return
}

func (hl handler) handleGetThreads(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {

	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	it, err := hl.revSrv.GetThreads(dbRead, r.Repo.Id, cI,
		/*ascendingOrder*/ true)
	if err != nil {
		log.Printf("error getting threads for repo_id=%d, commit_id=%d in handleGetThreads, err: %v", r.Repo.Id, cI, err)
		http.Error(w, "err getting all threads", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// To avoid loading it all into memory, we'll stream the threads
	_, err = w.Write([]byte("["))
	if err != nil {
		log.Printf("error writing `[` to handleGetThreads ResponseWriter, err: %v", err)
		http.Error(w, "err opening stream", http.StatusInternalServerError)
		return
	}
	encoder := json.NewEncoder(w)
	first := true
	for it.Next() {
		th, err := it.Get()
		if err != nil {
			log.Printf("error getting Thread in handleGetThreads, err: %v", err)
			http.Error(w, "err getting thread", http.StatusInternalServerError)
			return
		}
		if !first {
			// Write commas between elements
			w.Write([]byte(","))
		}
		first = false
		threadAuthorUser, _, err := hl.userS.Get(dbRead, th.AuthorUserId)
		if err != nil {
			log.Printf("error getting thread (id=%d, author_id=%d) author user in handleGetThreads, err: %v",
				th.Id, th.AuthorUserId, err)
			http.Error(w, "failed to get user", http.StatusInternalServerError)
			return
		}
		err = encoder.Encode(newFrontendThread(th, threadAuthorUser.Username))
		if err != nil {
			log.Printf("error writing thread (id=%d, author_username=%q) in handleGetThreads, err: %v",
				th.Id, threadAuthorUser.Username, err)
			http.Error(w, "err writing thread", http.StatusInternalServerError)
			return
		}
	}
	err = it.Err()
	if err != nil {
		log.Printf("error iterating threads for repo_id=%d, commit_id=%d in handleGetThreads, err: %v",
			r.Repo.Id, cI, err)
		http.Error(w, "err iterating threads", http.StatusInternalServerError)
		return
	}
	_, err = w.Write([]byte("]"))
	if err != nil {
		log.Printf("error writing `]` to handleGetThreads ResponseWriter, err: %v", err)
		http.Error(w, "err closing stream", http.StatusInternalServerError)
		return
	}
}

// Returns the unified diff (string)
func (hl handler) handleGetDiff(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit id", http.StatusBadRequest)
		return
	}

	serverRead := hl.rSrv.GetServerRead(dbRead)
	s, err := hl.rSrv.GetServerByRepoId(dbRead, r.Repo.Id)
	if err != nil {
		log.Printf("failed to get server: %s", err)
		http.Error(w, "failed to get server", http.StatusInternalServerError)
		return
	}
	var right commit.Commit
	hasRightV, rightVParam := hl.rt.GetRightVersionParameter(r.Request)
	if hasRightV {
		right, err = s.GetVersion(cI, rightVParam, serverRead)
	} else {
		right, err = s.GetLatest(cI, serverRead)
	}
	if err != nil {
		log.Printf("failed to get right commit: %s", err)
		http.Error(w, "failed to get right commit", http.StatusInternalServerError)
		return
	}
	var left commit.Commit
	hasLeftV, leftVParam := hl.rt.GetLeftVersionParameter(r.Request)
	if hasLeftV {
		left, err = s.GetVersion(cI, leftVParam, serverRead)
	} else {
		left, err = s.GetVersion(right.ParentL, right.ParentV, serverRead)
	}
	if err != nil {
		log.Printf("failed to get left commit: %s", err)
		http.Error(w, "failed to get left commit", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	err = s.WriteDiff(right, left, hl.rt.GetFileParam(r.Request), w, serverRead)
	if err != nil {
		log.Printf("failed to write diff: %s", err)
		return
	}
}

func (hl handler) handleGetFile(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	serverRead := hl.rSrv.GetServerRead(dbRead)

	s, isNotFoundErr, err := hl.rSrv.GetServer(
		dbRead, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		return
	}
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	hasVersion, version := hl.rt.GetVersionParameter(r.Request)
	var c commit.Commit
	if hasVersion {
		c, err = s.GetVersion(cI, version, serverRead)
	} else {
		c, err = s.GetLatest(cI, serverRead)
	}
	if err != nil {
		// TODO: log error. We just need to differentiate between bad req
		// or a server error reading the commit
		http.Error(w, "failed to get commit", http.StatusInternalServerError)
		return
	}
	err = s.WriteFile(c, hl.rt.GetFileParam(r.Request), w, serverRead)
	if err != nil {
		// TODO: log error. We just need to differentiate between bad req
		// or a server error reading the commit
		http.Error(w, "failed to get file", http.StatusInternalServerError)
		return
	}
}

const isResolvedParameterName = "resolved"
const isResolvedTrueParameterValue = "1"
const commentTextParameterName = "comment"

// Returns saves a new thread and return it
func (hl handler) handlePostNewThread(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	var err error
	defer func() {
		if err != nil {
			http.Error(w, fmt.Sprintf("internal error: %s", err),
				http.StatusInternalServerError)
			return
		}
	}()

	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		err = errors.New("bad commit format")
		return
	}
	hasV, v := hl.rt.GetVersionParameter(r.Request)
	if !hasV {
		err = errors.New("commit version not provided")
		return
	}
	isResolved := r.FormValue(isResolvedParameterName) == isResolvedTrueParameterValue
	commentText := r.FormValue(commentTextParameterName)
	if commentText == "" {
		err = errors.New("comment text not provided")
		return
	}
	file := hl.rt.GetFileParam(r.Request)
	// A line without a file has nothing to anchor to.
	hasLine, line := hl.rt.GetLineParameter(r.Request)
	if hasLine && file == "" {
		err = errors.New("line provided without a file")
		return
	}

	var newThread review.Thread
	if file == "" {
		newThread, err = hl.revSrv.CreateDiscussionThread(dbWrite,
			quotaOwner(r.Repo.OwnerId), r.Repo.Id,
			cI, v, r.UserWithWritePermission.Id,
			commentText, isResolved)
		if err != nil {
			return
		}
	} else {
		newThread, err = hl.revSrv.CreateThread(dbWrite,
			quotaOwner(r.Repo.OwnerId), r.Repo.Id, cI, v, file,
			line, r.UserWithWritePermission.Id, commentText, isResolved)
		if err != nil {
			return
		}
	}

	assetPath := hl.rt.Commit(
		r.RepoOwnerUsr.Username,
		r.Repo.DisplayName,
		cI,
		/*hasLeft=*/ false /*left=*/, 0,
		/*hasRight=*/ false /*right=*/, 0,
	)
	msg := fmt.Sprintf("%s commented on c/%d", r.UserWithWritePermission.Username, cI)

	err = hl.notifyCommitAuthor(
		dbWrite,
		r.Repo.Id,
		cI,
		&v,
		r.UserWithWritePermission.Id,
		r.UserWithWritePermission.Username,
		assetPath,
		msg,
	)
	if err != nil {
		return
	}

	encoder := json.NewEncoder(w)
	err = encoder.Encode(newFrontendThread(newThread,
		r.UserWithWritePermission.Username))
	shouldCommit = true
	return
}

func (hl handler) handlePostToThread(w http.ResponseWriter, r wrappers.UserRepoMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	var err error
	defer func() {
		if err != nil {
			http.Error(w, fmt.Sprintf("internal error: %s", err),
				http.StatusInternalServerError)
			return
		}
	}()
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		err = fmt.Errorf("bad commit format")
		return
	}
	thIdS := r.PathValue(routes.ThreadIdParamName)
	if thIdS == "" {
		err = fmt.Errorf("thread id not specified")
		return
	}
	thId, err := strconv.ParseInt(thIdS, 10, 64)
	if err != nil {
		err = fmt.Errorf("bad thread Id format")
		return
	}
	isResolved := r.FormValue(isResolvedParameterName) == isResolvedTrueParameterValue
	commentText := r.FormValue(commentTextParameterName)
	_, err = hl.revSrv.AddToThread(
		dbWrite, quotaOwner(r.Repo.OwnerId),
		r.Repo.Id, cI, thId, r.UserWithWritePermission.Id, commentText, isResolved)
	if err != nil {
		return
	}
	assetPath := hl.rt.Commit(
		r.RepoOwnerUsr.Username,
		r.Repo.DisplayName,
		cI,
		/*hasLeft=*/ false /*left=*/, 0,
		/*hasRight=*/ false /*right=*/, 0,
	)
	msg := ""
	if isResolved {
		msg = fmt.Sprintf("%s resolved a comment on c/%d", r.UserWithWritePermission.Username, cI)
	} else {
		msg = fmt.Sprintf("%s replied to a comment on c/%d", r.UserWithWritePermission.Username, cI)
	}

	participants, err := hl.collectThreadParticipants(
		dbWrite,
		r.Repo.Id,
		cI,
		thId,
	)
	if err != nil {
		return
	}

	err = hl.sendNotifications(
		dbWrite,
		/*actorUserId*/ r.UserWithWritePermission.Id,
		participants,
		assetPath,
		msg,
	)
	if err != nil {
		return
	}
	w.Write([]byte("ok"))
	shouldCommit = true
	return
}

// Returns a HTML fragment of the commit description
func (hl handler) handlePostCommit(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	serverRead := hl.rSrv.GetServerRead(dbWrite)
	hasLeft, left := hl.rt.GetLeftVersionParameter(r.Request)
	hasRight, right := hl.rt.GetRightVersionParameter(r.Request)

	s, isNotFoundErr, err := hl.rSrv.GetServer(dbWrite, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "err getting server", http.StatusInternalServerError)
		return
	}
	c, err := s.GetLatest(cI, serverRead)
	if err != nil {
		http.Error(w, "err getting commit", http.StatusInternalServerError)
		return
	}
	if c.IsSubmitted {
		http.Error(w, "desc of submitted commit cant be changed",
			http.StatusBadRequest)
		return
	}

	desc := twiggwc.ParsePostClDescription(r.Request)
	if len(desc) == 0 {
		http.Error(w, "description cant be empty", http.StatusBadRequest)
		return
	}
	if len(desc) > reviewservice.MaxDescriptionLength {
		http.Error(w,
			fmt.Sprintf("description cant be > %d",
				reviewservice.MaxDescriptionLength), http.StatusBadRequest)
		return
	}
	err = hl.revSrv.SetDescription(dbWrite, quotaOwner(r.Repo.OwnerId),
		r.Repo.Id, cI, desc /*createIfNeeded*/, true)
	if err != nil {
		http.Error(w, "internal error setting desc",
			http.StatusInternalServerError)
		return
	}
	twiggwc.ClDescription(desc, hl.rt.Commit(
		r.RepoOwnerUsr.Username, r.Repo.DisplayName, cI, hasLeft,
		left, hasRight, right)).Render(w)
	shouldCommit = true
	return
}

func (hl handler) handleGetCommitJobs(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	serverRead := hl.rSrv.GetServerRead(dbRead)
	cId, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	hasVersion, version := hl.rt.GetVersionParameter(r.Request)
	s, err := hl.rSrv.GetServerByRepoId(dbRead, r.Repo.Id)
	if err != nil {
		log.Printf("err getting server: %s", err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	var c commit.Commit
	if hasVersion {
		c, err = s.GetVersion(cId, version, serverRead)
	} else {
		c, err = s.GetLatest(cId, serverRead)
	}
	if err != nil {
		log.Printf("err getting commit: %s", err)
		http.Error(w, "internal error getting commit", http.StatusInternalServerError)
		return
	}

	// Job representation on the frontend
	type frontendJob struct {
		InternalId    int64
		RepoId        uint64
		Commit        uint64
		CommitVersion uint64
		Path          string
		Name          string
		RunNumber     int64
		Status        job.JobStatus
		CreatedTime   string
		Id            string
	}
	var afterInternalJobId int64 = 0
	if p := r.URL.Query().Get(routes.AfterInternalJobIdQueryParamName); p != "" {
		afterInternalJobId, err = strconv.ParseInt(p, 10, 64)
		if err != nil {
			log.Printf("err parsing afterJobId int: %s", err)
			http.Error(w, "invalid afterJobId value", http.StatusBadRequest)
			return
		}
	}
	jobsIter, err := hl.jobsS.GetCommitJobs(dbRead, r.Repo.Id, c.ServerL, afterInternalJobId)
	if err != nil {
		log.Printf("failed to get job iterator: %s", err)
		http.Error(w, "internal error gettting jobs iter",
			http.StatusInternalServerError)
		return
	}
	jobs, err := iterator.GetFirstN(CiJobsPageSize, jobsIter)
	if err != nil {
		log.Printf("failed to get read jobs iterator: %s", err)
		http.Error(w, "internal error gettting jobs",
			http.StatusInternalServerError)
		return
	}
	frontendJobs := make([]frontendJob, len(jobs))
	for i := 0; i < len(frontendJobs); i++ {
		frontendJobs[i] = frontendJob{
			InternalId:    jobs[i].InternalId,
			RepoId:        jobs[i].RepoId,
			Commit:        jobs[i].Commit,
			CommitVersion: jobs[i].CommitVersion,
			Path:          jobs[i].Path,
			Name:          jobs[i].Name,
			RunNumber:     jobs[i].RunNumber,
			Status:        jobs[i].Status,
			CreatedTime:   jobs[i].CreatedTime,
			Id:            jobs[i].Id(),
		}
	}
	err = json.NewEncoder(w).Encode(frontendJobs)
	if err != nil {
		log.Printf("failed to serialize jobs: %s", err)
		http.Error(w, "failed to serialize jobs", http.StatusInternalServerError)
		return
	}
}

func (hl handler) handleGetJobCombinedOut(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	jobId := r.Request.URL.Query().Get(routes.JobIdQueryParamName)

	j, err := hl.jobsS.GetJobById(dbRead, jobId)
	if err != nil {
		log.Printf("failed to get jobId %s: %s", jobId, err)
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}
	if j.RepoId != r.Repo.Id {
		// This is a crytical security requirement.
		// The jobId must be of the specified repo.
		http.Error(w, "no access to this job", http.StatusUnauthorized)
		return
	}

	if j.Status == job.JobStatusBadFileSize {
		_, _ = w.Write([]byte("file is too large"))
		return
	}
	if j.Status == job.JobStatusTooManyJobs {
		_, _ = w.Write([]byte("too many jobs were triggered"))
		return
	}
	if j.Status == job.JobStatusExceedsPlanLimits {
		_, _ = w.Write([]byte("job requires more execution time or resources than allowed by your current plan"))
		return
	}
	if j.Status == job.JobStatusBadFileFormat {
		serverRead := hl.rSrv.GetServerRead(dbRead)
		s, err := hl.rSrv.GetServerByRepoId(dbRead, r.Repo.Id)
		if err != nil {
			log.Printf("err getting server: %s", err)
			http.Error(w, "internal error getting srv", http.StatusInternalServerError)
			return
		}
		c, err := s.GetVersion(j.Commit, j.CommitVersion, serverRead)
		if err != nil {
			log.Printf("err getting commit: %s", err)
			http.Error(w, "internal error getting commit", http.StatusInternalServerError)
			return
		}
		buff := bytes.NewBuffer(nil)
		err = s.WriteFile(c, j.Path, buff, serverRead)
		if err != nil {
			log.Printf("err getting job file: %s", err)
			http.Error(w, "internal error getting job file", http.StatusInternalServerError)
			return
		}
		var notOkErrMsg string
		if strings.HasSuffix(j.Path, cicdpublisher.CiFilename) {
			var ok bool
			_, ok, notOkErrMsg = hl.parser.ParseCiFile(buff.Bytes())
			if ok {
				panic(fmt.Sprintf("job %s status is bad format but parse says ok", jobId))
			}
		}
		if strings.HasSuffix(j.Path, cicdpublisher.CdFilename) {
			var ok bool
			_, ok, notOkErrMsg = hl.parser.ParseCdFile(buff.Bytes())
			if ok {
				panic(fmt.Sprintf("job %s status is bad format but parse says ok", jobId))
			}
		}
		if notOkErrMsg == "" {
			panic("file is neither ci nor cd")
		}
		// Write the err message and the file
		message := append([]byte(notOkErrMsg+" :\n"), buff.Bytes()...)
		_, _ = w.Write(message)
		return
	}

	combinedOut, err := hl.trackClient.GetCombinedOutput(jobId)
	if err != nil {
		log.Printf("failed to get combinedOut jobid %s: %s", jobId, err)
		http.Error(w, "internal error getting logs",
			http.StatusInternalServerError)
		return
	}
	defer combinedOut.Close()
	_, _ = io.Copy(w, combinedOut)
}

func (hl handler) handlePostSubmit(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	serverWrite := hl.rSrv.GetServerWrite(dbWrite)

	s, isNotFoundErr, err := hl.rSrv.GetServer(dbWrite, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to get srv: %s", err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	supremeLeaders, err := hl.revSrv.ResolveSupremeLeaders(dbWrite, r.RepoOwnerUsr)
	if err != nil {
		http.Error(w, "internal err resolving supreme leaders", http.StatusInternalServerError)
		return
	}
	d, isNotFoundErr, err := hl.revSrv.GetData(dbWrite, r.Repo.Id, cI,
		/*checkOwners*/ true, s.Top().ServerL, supremeLeaders)
	if err != nil && !isNotFoundErr {
		log.Printf("failed to get review data: %s", err)
		http.Error(w, "internal err getting review data",
			http.StatusInternalServerError)
		return
	}
	if isNotFoundErr || d.ReviewStatus != review.ReviewStatus_Ready {
		http.Error(w, "not approved", http.StatusBadRequest)
		return
	}
	c, err := s.GetLatest(cI, serverWrite)
	if err != nil {
		log.Printf("failed to get commit: %s", err)
		http.Error(w, "internal err getting c", http.StatusInternalServerError)
		return
	}
	if isWipCommit(c.Message) {
		http.Error(w, "can't submit WIP commits", http.StatusBadRequest)
		return
	}
	err = s.Submit(c, serverWrite)
	if err != nil {
		log.Printf("failed to submit: %s", err)
		http.Error(w, "internal err on submit", http.StatusInternalServerError)
		return
	}

	_, err = hl.ciq.EnqueueCiCdRun(r.Repo.Id, c.L, c.Version+1, runnerlib.OnSumit, dbWrite)
	if err != nil {
		log.Printf("failed to enqueue cicd run: %s", err)
		http.Error(w, "internal err enqueing cicd run",
			http.StatusInternalServerError)
		return
	}

	hasLeft, left := hl.rt.GetLeftVersionParameter(r.Request)
	hasRight, right := hl.rt.GetRightVersionParameter(r.Request)
	http.Redirect(w, r.Request, hl.rt.Commit(r.RepoOwnerUsr.Username,
		r.Repo.DisplayName, c.L, hasLeft, left, hasRight, right),
		http.StatusSeeOther)

	gitMirrorUrl, isNotFoundErr, err := hl.rSrv.GetGitMirrorUrl(dbWrite, r.Repo.Id)
	if err != nil && !isNotFoundErr {
		log.Printf("err=%s getting git mirror url in handlePostSubmit", err)
		http.Error(w, "internal err on submit", http.StatusInternalServerError)
		return
	}
	if r.Repo.IsGitMirrorEnabled && !isNotFoundErr {
		payloadType, payload, err := reposettings.PushTopToGitMirrorPayload(
			r.Repo.Id, gitMirrorUrl)
		if err != nil {
			log.Printf("err=%s creating git mirror payload in handlePostSubmit", err)
			http.Error(w, "internal err in git mirror", http.StatusInternalServerError)
			return
		}
		err = hl.queue.Enqueue(payloadType, payload)
		if err != nil {
			log.Printf("err=%s enqueuing git mirror payload in handlePostSubmit", err)
			http.Error(w, "internal err in git mirror", http.StatusInternalServerError)
			return
		}
	}
	assetPath := hl.rt.Commit(
		r.RepoOwnerUsr.Username,
		r.Repo.DisplayName,
		c.L,
		/*hasLeft*/ false /*left*/, 0,
		/*hasRight*/ false /*right*/, 0)
	msg := fmt.Sprintf("c/%d was submitted by %s",
		c.L,
		r.UserWithWritePermission.Username,
	)
	err = hl.notifyCommitAuthor(
		dbWrite,
		r.Repo.Id,
		c.L,
		&c.Version,
		r.UserWithWritePermission.Id,
		r.UserWithWritePermission.Username,
		assetPath,
		msg,
	)
	if err != nil {
		log.Printf("failed to notify submit: %s", err)
		http.Error(w, "failed to notify commit author", http.StatusInternalServerError)
		return
	}

	shouldCommit = true
	return
}

func (hl handler) handlePostRollback(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	serverWrite := hl.rSrv.GetServerWrite(dbWrite)
	s, err := hl.rSrv.GetServerByRepoId(dbWrite, r.Repo.Id)
	if err != nil {
		log.Printf("failed to get server: %s", err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	c, err := s.GetLatest(cI, serverWrite)
	if err != nil {
		log.Printf("failed to get commit: %s", err)
		http.Error(w, "internal err getting c", http.StatusInternalServerError)
		return
	}
	if !c.IsSubmitted {
		http.Error(w, "cant rollback non submitted", http.StatusBadRequest)
		return
	}
	rollback, err := s.CreateRollback(c.L, r.UserWithWritePermission.Id, serverWrite)
	if err != nil {
		log.Printf("failed to create rollback commit: %s", err)
		http.Error(w, "internal err creating rollback",
			http.StatusInternalServerError)
		return
	}
	originalCommitData, isNotFoundErr, err := hl.revSrv.GetData(
		dbWrite, r.Repo.Id, cI,
		/*checkOwners*/ false, 0, []string{})
	if err != nil && !isNotFoundErr {
		log.Printf("failed to get original commit data: %s", err)
		http.Error(w, "internal err getting original commit data",
			http.StatusInternalServerError)
		return
	}
	originalUrl := hl.rt.Commit(r.RepoOwnerUsr.Username,
		r.Repo.DisplayName, cI,
		/*hasLeft*/ false /*left*/, 0,
		/*hasRight*/ false /*right*/, 0)
	rollbackCommitDescription :=
		fmt.Sprintf("\n### Rollback [c/%d](%s)", cI, originalUrl)
	if originalCommitData.Description != "" {
		rollbackCommitDescription = rollbackCommitDescription + ":\n" +
			originalCommitData.Description
	}
	err = hl.revSrv.SetDescription(dbWrite,
		quotaOwner(r.Repo.OwnerId),
		r.Repo.Id, rollback.L, trimEllipsis(
			rollbackCommitDescription, reviewservice.MaxDescriptionLength), true)
	if err != nil {
		log.Printf("failed to set rollback description: %s", err)
		http.Error(w, "internal err setting rollback desc",
			http.StatusInternalServerError)
		return
	}

	assetPath := hl.rt.Commit(
		r.RepoOwnerUsr.Username,
		r.Repo.DisplayName,
		cI,
		/*hasLeft*/ false /*left*/, 0,
		/*hasRight*/ false /*right*/, 0)
	msg := fmt.Sprintf("rollback of c/%d was created by %s", cI, r.UserWithWritePermission.Username)
	err = hl.notifyCommitAuthor(
		dbWrite,
		r.Repo.Id,
		cI,
		&c.Version,
		r.UserWithWritePermission.Id,
		r.UserWithWritePermission.Username,
		assetPath,
		msg,
	)
	if err != nil {
		log.Printf("failed no notify rollback: %s", err)
		http.Error(w, "failed to notify commit author", http.StatusInternalServerError)
		return
	}
	shouldCommit = true
	redirectPath := hl.rt.Commit(r.RepoOwnerUsr.Username,
		r.Repo.DisplayName, rollback.L,
		/*hasLeft*/ false /*left*/, 0,
		/*hasRight*/ false /*right*/, 0)
	_, _ = w.Write([]byte(redirectPath))
	return
}

func (hl handler) handlePostRename(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	serverWrite := hl.rSrv.GetServerWrite(dbWrite)
	s, err := hl.rSrv.GetServerByRepoId(dbWrite, r.Repo.Id)
	if err != nil {
		log.Printf("failed to get server: %s", err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	var body struct {
		Message string
	}
	err = json.NewDecoder(r.Request.Body).Decode(&body)
	if err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	_, err = s.RenameCommit(cI, body.Message, r.UserWithWritePermission.Id, serverWrite)
	if err != nil {
		log.Printf("failed to rename commit: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	shouldCommit = true
	w.WriteHeader(http.StatusNoContent)
	return
}

// isWipCommit returns true if the commit title marks the commit as WIP.
// Same logic in the frontend.
var wipRegex = regexp.MustCompile(`(?i)^wip($|[^a-z0-9])`)

func isWipCommit(message string) bool {
	if strings.TrimSpace(message) == "" {
		return false
	}

	return wipRegex.MatchString(strings.TrimSpace(message))
}

func (hl handler) handlePostAddLgtm(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}

	hasVersion, version := hl.rt.GetVersionParameter(r.Request)
	if !hasVersion {
		http.Error(w, "version not provided", http.StatusBadRequest)
		return
	}
	s, isNotFoundErr, err := hl.rSrv.GetServer(
		dbWrite, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	serverRead := hl.rSrv.GetServerRead(dbWrite)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to get server: %s", err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	commit, err := s.GetVersion(cI, version, serverRead)
	if err != nil {
		log.Printf("failed to get commit: %s", err)
		http.Error(w, "internal error getting commit", http.StatusInternalServerError)
		return
	}
	if commit.IsSubmitted {
		http.Error(w, "can't add lgtm to submitted", http.StatusBadRequest)
		return
	}

	var newThread review.Thread
	newThread, err = hl.revSrv.AddLgtm(dbWrite, quotaOwner(r.Repo.OwnerId), r.Repo.Id, cI, version,
		r.UserWithWritePermission.Id)
	if err != nil {
		log.Printf("failed to add lgtm: %s", err)
		http.Error(w, "failed to add lgtm", http.StatusInternalServerError)
		return
	}
	encoder := json.NewEncoder(w)
	err = encoder.Encode(newFrontendThread(newThread,
		r.UserWithWritePermission.Username))
	if err != nil {
		log.Printf("failed to write encoded thread response: %s", err)
		http.Error(w, "failed to write encoded thread response", http.StatusInternalServerError)
		return
	}

	assetPath := hl.rt.Commit(
		r.RepoOwnerUsr.Username,
		r.Repo.DisplayName,
		cI,
		/*hasLeftVersion*/ false /*leftVersion*/, 0,
		/*hasRightVersion*/ false /*rightVersion*/, 0,
	)
	msg := fmt.Sprintf("%s LGTM'd c/%d", r.UserWithWritePermission.Username, cI)
	err = hl.notifyCommitAuthor(
		dbWrite,
		r.Repo.Id,
		cI,
		&version,
		r.UserWithWritePermission.Id,
		r.UserWithWritePermission.Username,
		assetPath,
		msg,
	)
	if err != nil {
		log.Printf("failed to notify LGTM: %s", err)
		http.Error(w, "failed to notify commit author", http.StatusInternalServerError)
		return
	}

	shouldCommit = true
	return
}

func (hl handler) handlePostRemoveLgtm(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}
	s, isNotFoundErr, err := hl.rSrv.GetServer(
		dbWrite, r.RepoOwnerUsr.Id, r.Repo.DisplayName)
	serverRead := hl.rSrv.GetServerRead(dbWrite)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to get server: %s", err)
		http.Error(w, "internal error getting srv", http.StatusInternalServerError)
		return
	}
	commit, err := s.GetLatest(cI, serverRead)
	if err != nil {
		log.Printf("failed to get commit: %s", err)
		http.Error(w, "internal error getting commit", http.StatusInternalServerError)
		return
	}
	if commit.IsSubmitted {
		http.Error(w, "can't remove lgtm from submitted", http.StatusBadRequest)
		return
	}

	var newThread review.Thread
	newThread, err = hl.revSrv.RemoveLastLgtm(dbWrite, quotaOwner(r.Repo.OwnerId),
		r.Repo.Id, cI,
		r.UserWithWritePermission.Id)
	if err != nil {
		http.Error(w, "failed to remove lgtm", http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	err = encoder.Encode(newFrontendThread(newThread,
		r.UserWithWritePermission.Username))
	if err != nil {
		http.Error(w, "failed to write thread", http.StatusInternalServerError)
		return
	}
	assetPath := hl.rt.Commit(
		r.RepoOwnerUsr.Username,
		r.Repo.DisplayName,
		cI,
		/*hasLeft=*/ false /*left=*/, 0,
		/*hasRight=*/ false /*right=*/, 0,
	)

	msg := fmt.Sprintf("%s removed LGTM from c/%d", r.UserWithWritePermission.Username, cI)
	err = hl.notifyCommitAuthor(
		dbWrite,
		r.Repo.Id,
		cI,
		&newThread.CommitVersion,
		r.UserWithWritePermission.Id,
		r.UserWithWritePermission.Username,
		assetPath,
		msg,
	)
	if err != nil {
		log.Printf("failed no notify LGTM remove: %s", err)
		http.Error(w, "failed to notify commit author", http.StatusInternalServerError)
		return
	}
	shouldCommit = true
	return
}

func newFrontendThread(t review.Thread, authorUsername string) FrontendThread {

	var typeString string
	switch t.Type {
	case review.ThreadType_CommentsOnFileOnCommitVersion:
		typeString = "CommentsOnFileOnCommitVersion"
	case review.ThreadType_CommentsOnCommitVersion:
		typeString = "CommentsOnCommitVersion"
	case review.ThreadType_AddLGTM:
		typeString = "AddLGTM"
	case review.ThreadType_RemoveLGTM:
		typeString = "RemoveLGTM"
	default:
		panic("got unknown type")
	}

	return FrontendThread{
		Id:             t.Id,
		Type:           typeString,
		CommitVersion:  t.CommitVersion,
		IsResolved:     t.IsResolved,
		IsLgtm:         t.IsLgtm,
		Filename:       t.Filename,
		Line:           t.Line,
		AuthorUsername: authorUsername,
		CreatedOn:      t.CreatedOn,
	}
}

// This is the comment that is sent to the frontend
type frontendComment struct {
	ThreadId       int64
	AuthorUsername string
	Text           string
	T              time.Time
}

func newFrontendComment(cm review.Comment,
	authorUsername string) frontendComment {
	return frontendComment{
		ThreadId:       cm.ThreadId,
		AuthorUsername: authorUsername,
		Text:           cm.Text,
		T:              cm.T,
	}
}

func (hl handler) handlePostAddReviewers(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}

	const maxUsernamesPerRequest = 20

	type AddReviewersBody struct {
		Usernames []string
	}
	var addReviewersBody AddReviewersBody
	limitedReader := io.LimitReader(r.Body, 10_000)
	err = json.NewDecoder(limitedReader).Decode(&addReviewersBody)
	if err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	if len(addReviewersBody.Usernames) == 0 {
		http.Error(w, "no usernames provided", http.StatusBadRequest)
		return
	}
	if len(addReviewersBody.Usernames) > maxUsernamesPerRequest {
		http.Error(w, fmt.Sprintf("too many usernames (max %d)", maxUsernamesPerRequest), http.StatusBadRequest)
		return
	}

	for _, username := range addReviewersBody.Usernames {
		addedUser, isNotFound, err := hl.userS.GetByUsername(dbWrite, username)
		if isNotFound || err != nil {
			http.Error(w, fmt.Sprintf("invalid username: %s", username), http.StatusBadRequest)
			return
		}
		err = hl.revSrv.AddReviewer(dbWrite, quotaOwner(r.Repo.OwnerId), r.Repo.Id, cI, addedUser.Id)
		if err != nil {
			log.Printf("failed to AddReviewer: %s", err)
			http.Error(w, "failed to add reviewer", http.StatusInternalServerError)
			return
		}
		assetPath := hl.rt.Commit(
			r.RepoOwnerUsr.Username,
			r.Repo.DisplayName,
			cI,
			/*hasLeft=*/ false /*left=*/, 0,
			/*hasRight=*/ false /*right=*/, 0,
		)
		// Notify the added reviewer (no self-notify).
		if addedUser.Id != r.UserWithWritePermission.Id {
			msg := fmt.Sprintf("%s added you as a reviewer of c/%d", r.UserWithWritePermission.Username, cI)
			if err := hl.db.CreateNotification(dbWrite, addedUser.Id, msg, assetPath); err != nil {
				log.Printf("failed to notify reviewer: %s", err)
				http.Error(w, "failed to notify reviewer", http.StatusInternalServerError)
				return
			}
		}

		// Notify the commit author
		// It does nothing if actor is commit author (no self-notify).
		err = hl.notifyCommitAuthor(
			dbWrite,
			r.Repo.Id,
			cI,
			/*nil is equal least version*/ nil,
			r.UserWithWritePermission.Id,
			r.UserWithWritePermission.Username,
			assetPath,
			fmt.Sprintf("%s added %s as a reviewer of c/%d", r.UserWithWritePermission.Username, username, cI),
		)
		if err != nil {
			log.Printf("failed to notify commit author: %s", err)
			http.Error(w, "failed to notify commit author", http.StatusInternalServerError)
			return
		}
	}

	_, err = w.Write([]byte("ok"))
	if err != nil {
		log.Printf("failed to write ok response in handlePostAddReviewers: %s", err)
	}
	shouldCommit = true
	return
}
func (hl handler) handlePostRemoveReviewers(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}

	const maxUsernamesPerRequest = 20

	type RemoveReviewersBody struct {
		Usernames []string
	}
	var removeReviewersBody RemoveReviewersBody
	limitedReader := io.LimitReader(r.Body, 10_000)
	err = json.NewDecoder(limitedReader).Decode(&removeReviewersBody)
	if err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	if len(removeReviewersBody.Usernames) == 0 {
		http.Error(w, "no usernames provided", http.StatusBadRequest)
		return
	}
	if len(removeReviewersBody.Usernames) > maxUsernamesPerRequest {
		http.Error(w, fmt.Sprintf("too many usernames (max %d)", maxUsernamesPerRequest), http.StatusBadRequest)
		return
	}

	for _, username := range removeReviewersBody.Usernames {
		u, isNotFound, err := hl.userS.GetByUsername(dbWrite, username)
		if isNotFound || err != nil {
			http.Error(w, fmt.Sprintf("invalid username: %s", username), http.StatusBadRequest)
			return
		}
		err = hl.revSrv.RemoveReviewer(dbWrite, quotaOwner(r.Repo.OwnerId), r.Repo.Id, cI, u.Id)
		if err != nil {
			log.Printf("failed to RemoveReviewer: %s", err)
			http.Error(w, "failed to remove reviewer", http.StatusInternalServerError)
			return
		}
		assetPath := hl.rt.Commit(
			r.RepoOwnerUsr.Username,
			r.Repo.DisplayName,
			cI,
			/*hasLeft=*/ false /*left=*/, 0,
			/*hasRight=*/ false /*right=*/, 0,
		)

		// Notify the commit author
		// It does nothing if actor is commit author (no self-notify).
		err = hl.notifyCommitAuthor(
			dbWrite,
			r.Repo.Id,
			cI,
			/*nil is equal latest version*/ nil,
			r.UserWithWritePermission.Id,
			r.UserWithWritePermission.Username,
			assetPath,
			fmt.Sprintf("%s removed %s as a reviewer of c/%d", r.UserWithWritePermission.Username, username, cI),
		)
		if err != nil {
			log.Printf("failed to notify commit author: %s", err)
			http.Error(w, "failed to notify commit author", http.StatusInternalServerError)
			return
		}
	}

	_, err = w.Write([]byte("ok"))
	if err != nil {
		log.Printf("failed to write ok response in handlePostRemoveReviewers: %s", err)
	}
	shouldCommit = true
	return
}

func (hl handler) handleGetReviewers(w http.ResponseWriter,
	r wrappers.UserWithReadPermissionMuxRequest, dbRead context.Context) {

	cI, err := hl.rt.GetCommitParameter(r.Request)
	if err != nil {
		http.Error(w, "bad commit format", http.StatusBadRequest)
		return
	}

	d, isNotFoundErr, err := hl.revSrv.GetData(dbRead, r.Repo.Id, cI,
		/*checkOwners*/ false, 0, []string{})
	if err != nil && !isNotFoundErr {
		http.Error(w, "internal err getting review data", http.StatusInternalServerError)
		return
	}

	reviewerUsernames := make([]string, 0, len(d.ReviewersUserIds))
	for _, userId := range d.ReviewersUserIds {
		u, _, err := hl.userS.Get(dbRead, userId)
		if err != nil {
			log.Printf("failed to get reviewer user id=%d: %s", userId, err)
			http.Error(w, "internal err getting reviewer", http.StatusInternalServerError)
			return
		}
		reviewerUsernames = append(reviewerUsernames, u.Username)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(reviewerUsernames)
	if err != nil {
		log.Printf("failed to encode reviewers: %s", err)
		http.Error(w, "internal err writing response", http.StatusInternalServerError)
		return
	}
}

// Trim string and adds elipsis
func trimEllipsis(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

// notifyCommitAuthor creates a notification to the author of a commit when
// someone comments on it.
//
// If version is nil, it notifies the author of the latest version.
// If actorId == authorId, it does nothing (no self-notify).
func (hl handler) notifyCommitAuthor(
	dbWrite context.Context,
	repoId uint64,
	commitId uint64,
	version *uint64,
	actorId int64,
	actorUsername string,
	assetPath string,
	message string,
) error {
	serverRead := hl.rSrv.GetServerRead(dbWrite)

	s, err := hl.rSrv.GetServerByRepoId(dbWrite, repoId)
	if err != nil {
		return err
	}

	var c commit.Commit
	if version != nil {
		c, err = s.GetVersion(commitId, *version, serverRead)
	} else {
		c, err = s.GetLatest(commitId, serverRead)
	}
	if err != nil {
		return err
	}

	authorId := c.AuthorUserId
	if authorId == actorId {
		return nil
	}
	if message == "" {
		panic("notifyCommitAuthor with no message")
	}

	return hl.db.CreateNotification(dbWrite, authorId, message, assetPath)
}

// sendNotifications creates notifications for the given users.
// It assumes allInvolvedUserIds already contains unique user IDs.
// The actorUserId will never be notified.
func (hl handler) sendNotifications(
	dbWrite context.Context,
	actorUserId int64,
	allInvolvedUserIds []int64,
	assetPath string,
	message string,
) error {
	if message == "" {
		panic("sendNotifications called with empty message")
	}

	for _, userId := range allInvolvedUserIds {

		// Never notify the actor
		if userId == actorUserId {
			continue
		}

		if err := hl.db.CreateNotification(dbWrite, userId, message, assetPath); err != nil {
			return err
		}
	}

	return nil
}

// collectThreadParticipants returns all unique user IDs involved in a thread
// of a given commit. This includes:
//   - The commit author
//   - The thread author
//   - All users who have commented in the thread
//
// The returned slice contains no duplicate user IDs.
func (hl handler) collectThreadParticipants(
	dbRead context.Context,
	repoId uint64,
	commitId uint64,
	threadId int64,
) ([]int64, error) {

	unique := make(map[int64]struct{})

	serverRead := hl.rSrv.GetServerRead(dbRead)

	// Commit author
	s, err := hl.rSrv.GetServerByRepoId(dbRead, repoId)
	if err != nil {
		return nil, err
	}

	c, err := s.GetLatest(commitId, serverRead)
	if err != nil {
		return nil, err
	}
	unique[c.AuthorUserId] = struct{}{}

	// Thread author
	thread, err := hl.revSrv.GetThread(dbRead, threadId)
	if err != nil {
		return nil, err
	}
	unique[thread.AuthorUserId] = struct{}{}

	// All commenter's
	commentsIter, err := hl.revSrv.GetComments(dbRead, repoId, commitId, threadId)
	if err != nil {
		return nil, err
	}
	const maxParticipantsInteraction = 100
	var count = 0
	for commentsIter.Next() {
		if count >= maxParticipantsInteraction {
			return nil, fmt.Errorf("too many comments while collecting thread participants (>%d)", maxParticipantsInteraction)
		}
		cm, err := commentsIter.Get()
		if err != nil {
			return nil, err
		}
		unique[cm.AuthorUserId] = struct{}{}
		count++
	}

	if err := commentsIter.Err(); err != nil {
		return nil, err
	}

	// Convert map to slice
	participants := make([]int64, 0, len(unique))
	for userId := range unique {
		participants = append(participants, userId)
	}

	return participants, nil
}
