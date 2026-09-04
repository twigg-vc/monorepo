package server

import (
	"errors"
	"fmt"
	"io"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/xchange"
	"net/http"
	"strconv"
)

// Not necessarily all commits will be returned on a pull, to prevent cases
// that might cause OOO. The client might need to pull again to split the work.
const maxCommitsReturnedOnHandlePull = 500

// Max number of children sent with the commit of a detached pull
const maxChildrenSentOnDetachedPull = 1000

// Msg sent to the client on unexpected server errors
const serverErrMsg = "unexpected server error"

func (s srv) HandlePull(w http.ResponseWriter, r *http.Request,
	verifier PullVerifier, l Read) bool {
	// Create the writer that is used to write the commits to the client
	w.Header().Set("Content-Type", "application/octet-stream")
	xchange.SetTwiggHeaderInResponse(w)
	cw, cl, err := xchange.NewCommitWriter(w)
	defer cl()
	if err != nil {
		return false
	}
	// Validate the request
	ok, notOkMsg, err := verifier.PullIsOk(r)
	if err != nil {
		_ = cw.WriteErrMsg(serverErrMsg)
		return false
	}
	if !ok {
		_ = cw.WriteErrMsg(notOkMsg)
		return false
	}

	// Read parameters and handle detached/regular pull
	baseIdString := r.URL.Query().Get(client.BaseCommitServerIdQueryParam)
	var baseId uint64
	var baseV uint64
	var hasBaseV bool
	baseId, err = strconv.ParseUint(baseIdString, 10, 64)
	if err != nil {
		_ = cw.WriteErrMsg("bad base id")
		return false
	}
	vString := r.URL.Query().Get(client.BaseCommitServerVersionQueryParam)
	if vString == "" {
		hasBaseV = false
	} else {
		hasBaseV = true
		baseV, err = strconv.ParseUint(vString, 10, 64)
		if err != nil {
			_ = cw.WriteErrMsg("bad base v")
			return false
		}
	}
	isDetachedPull := r.URL.Query().Get(client.IsDetachedPullQueryParamName) ==
		client.IsDetachedPullQueryParamValue
	if isDetachedPull {
		targetIdStr := r.URL.Query().Get(client.DetachedCommitServerIdQueryParam)
		var targetId uint64
		targetV := uint64(0)
		hasTargetV := false
		if targetIdStr == client.DetachedLastSubmittedCommitAlias {
			targetId = s.Top_.ServerL
		} else {
			targetId, err = strconv.ParseUint(targetIdStr, 10, 64)
			if err != nil {
				_ = cw.WriteErrMsg("bad target id")
				return false
			}
			targetVStr := r.URL.Query().Get(client.DetachedCommitServerVersionQueryParam)
			if targetVStr != "" {
				targetV, err = strconv.ParseUint(targetVStr, 10, 64)
				if err != nil {
					_ = cw.WriteErrMsg("bad target v")
					return false
				}
				hasTargetV = true
			}
		}
		return s.handleDetachedPull(targetId, hasTargetV, targetV,
			baseId, hasBaseV, baseV, cw, l)
	}
	// Regular pulls require version
	if !hasBaseV {
		_ = cw.WriteErrMsg("missing base v")
		return false
	}
	return s.handleRegularPull(baseId, baseV, cw, l)
}

func (s srv) handleRegularPull(baseId, baseV uint64,
	cw xchange.CommitWriter, l Read) bool {
	base, isNotFoundErr, err := l.GetCommitVersionByServerId(s.RepoId, baseId, baseV)
	if isNotFoundErr {
		_ = cw.WriteErrMsg(fmt.Sprintf("c/%dv%d not found", baseId, baseV))
		return false
	}
	if err != nil {
		_ = cw.WriteErrMsg(serverErrMsg)
		return false
	}
	if !base.IsSubmitted {
		// We might change this but for now the current implementation
		// assumes base is the latest.
		_ = cw.WriteErrMsg(fmt.Sprintf("c/%dv%d is not submitted", baseId, baseV))
		return false
	}
	if base.Status != commit.StatusLatest {
		panic("submitted commit is not latest")
	}
	if len(base.Children) == 0 {
		err = cw.WriteEof()
		return err == nil
	}

	nCommitsWritten := 0
	// When pulling commits, only submitted commits are pulled.
	// Submitted commits only have one child, so we just need to write one
	// at a time
	var latestChild commit.Commit
	if len(base.Children) > 1 {
		panic("more than 1 child on submitted commit")
	}
	latestChild, err = s.GetLatest(base.Children[0], l)
	if err != nil {
		_ = cw.WriteErrMsg(serverErrMsg)
		return false
	}
	writeBaseL := base.L
	writeBaseV := base.Version
	writeBaseTreeV := base.TreeVersion
	for {
		err = cw.Write(latestChild,
			writeBaseL, writeBaseV, writeBaseTreeV, s.r, l)
		if err != nil {
			return false
		}
		nCommitsWritten++
		if nCommitsWritten >= maxCommitsReturnedOnHandlePull {
			err = cw.WriteUnexpectedEof()
			return err == nil
		}
		if len(latestChild.Children) == 0 {
			break
		}
		if len(latestChild.Children) > 1 {
			panic("more than 1 child on submitted commit")
		}
		latestChild, err = s.GetLatest(latestChild.Children[0], l)
		if err != nil {
			_ = cw.WriteErrMsg(serverErrMsg)
			return false
		}
		// Since c was just written, we can use it as base when
		// writing the next commit
		writeBaseL = latestChild.ParentL
		writeBaseV = latestChild.ParentV
		writeBaseTreeV = latestChild.ParentTreeVersion
	}
	err = cw.WriteEof()
	return err == nil
}

func (s srv) handleDetachedPull(targetId uint64, hasTargetV bool, targetV uint64,
	baseId uint64, hasBaseV bool, baseV uint64,
	cw xchange.CommitWriter, l Read) bool {
	var target commit.Commit
	var isNotFoundErr bool
	var err error
	if hasTargetV {
		if targetId == s.Top_.ServerL && targetV == s.Top_.ServerV {
			// avoid reading db if the top is the target
			target = s.Top_
		} else {
			target, isNotFoundErr, err = l.GetCommitVersionByServerId(
				s.RepoId, targetId, targetV)
		}
	} else {
		if targetId == s.Top_.ServerL {
			// avoid reading db if the top is the target
			target = s.Top_
		} else {
			target, isNotFoundErr, err = l.GetLatestCommitByServerId(
				s.RepoId, targetId)
		}
	}
	if isNotFoundErr {
		if hasTargetV {
			_ = cw.WriteErrMsg(fmt.Sprintf("target c/%dv%d not found", targetId, targetV))
		} else {
			_ = cw.WriteErrMsg(fmt.Sprintf("target c/%d not found", targetId))
		}
		return false
	}
	if err != nil {
		_ = cw.WriteErrMsg(serverErrMsg)
		return false
	}
	var base commit.Commit
	if hasBaseV {
		base, isNotFoundErr, err = l.GetCommitVersionByServerId(s.RepoId, baseId, baseV)
	} else {
		base, isNotFoundErr, err = l.GetLatestCommitByServerId(s.RepoId, baseId)
	}
	if isNotFoundErr {
		if hasBaseV {
			_ = cw.WriteErrMsg(fmt.Sprintf("base c/%dv%d not found", baseId, baseV))
		} else {
			_ = cw.WriteErrMsg(fmt.Sprintf("base c/%d not found", baseId))
		}
		return false
	}
	if err != nil {
		_ = cw.WriteErrMsg(serverErrMsg)
		return false
	}
	if target.ServerL == base.ServerL && target.ServerV == base.ServerV {
		// nothing to pull
		err = cw.WriteEof()
		return err == nil
	}
	// Children are not attached to commits when they are pushed, so they are
	// populated here. Only one commit is sent on a detached pull, so it's ok
	// for it to be a bit bigger.
	target.Children, target.ChildrenVersions, err = l.GetCommitChildren(
		s.RepoId, target.L, target.Version, maxChildrenSentOnDetachedPull)
	if err != nil {
		_ = cw.WriteErrMsg(serverErrMsg)
		return false
	}
	err = cw.Write(target,
		base.L, base.Version, base.TreeVersion, s.r, l)
	if err != nil {
		return false
	}
	err = cw.WriteEof()
	return err == nil
}

// Doesn't commit the lock.
// Saves the necessary CLs to the lock.
func (s *srv) HandlePush(w http.ResponseWriter, r *http.Request,
	verifier PushVerifier, p PushObserver, l Write) bool {
	xchange.SetTwiggHeaderInResponse(w)
	defer r.Body.Close()
	// Create the writer that will be used to write the response
	cIdWriter, closeCIdWriter, err := xchange.NewCommitIdWriter(w)
	defer closeCIdWriter()
	if err != nil {
		return false
	}
	// Create the reader that is used to read the commits
	cr, cl, err := xchange.NewCommitReader(r.Body)
	defer cl()
	if err != nil {
		_ = cIdWriter.WriteErrMsg(serverErrMsg)
		return false
	}

	// Validate the request
	ok, notOkMsg, err := verifier.PushIsOk(r)
	if err != nil {
		_ = cIdWriter.WriteErrMsg(serverErrMsg)
		return false
	}
	if !ok {
		_ = cIdWriter.WriteErrMsg(notOkMsg)
		return false
	}

	originalNextLocalId := s.NextLocalId
	done := false
	var pushOk bool
	for !done {
		done, pushOk = s.handleSiglePushedCommit(l, cIdWriter, cr, p)
	}

	// Reset the NextLocalId and signal that the tx should not be commited
	// if any error happens
	if !pushOk {
		l.PreventCommit()
		s.NextLocalId = originalNextLocalId
	}
	// If no error happened, save the NextLocalId
	if pushOk && originalNextLocalId != s.NextLocalId {
		saveErr := s.save(l)
		if saveErr != nil {
			s.NextLocalId = originalNextLocalId
			_ = cIdWriter.WriteErrMsg(serverErrMsg)
			pushOk = false
		}
	}

	return pushOk
}

// Reads one commit that was pushed.
// Returns `done=true` on any error or when there are no more commits to read.
func (s *srv) handleSiglePushedCommit(
	l Write, cId xchange.CommitIdWriter, cr xchange.CommitReader,
	pushObs PushObserver) (done bool, ok bool) {
	pushedCommit, baseCommitServerId, baseCommitServerV,
		pushedCommitIt, err := cr.Read()
	if err == io.EOF {
		done = true
		ok = true
		return
	}
	if err != nil && err != io.EOF {
		_ = cId.WriteErrMsg(serverErrMsg)
		done = true
		ok = false
		return
	}
	// Perform some sanity checks just to make the server more robust
	if pushedCommit.Status != commit.StatusLatest {
		_ = cId.WriteErrMsg("got obsolete commit")
		done = true
		ok = false
		return
	}
	if !pushedCommit.HasParentServerL || !pushedCommit.HasParentServerV {
		_ = cId.WriteErrMsg("parent of commit is not on server")
		done = true
		ok = false
		return
	}
	const maxMsgLen = 500
	if len(pushedCommit.Message) > maxMsgLen {
		_ = cId.WriteErrMsg("got too large message")
		done = true
		ok = false
		return
	}
	if len(pushedCommit.Children) != len(pushedCommit.ChildrenVersions) {
		_ = cId.WriteErrMsg("mismatch between children and childrenVersions len")
		done = true
		ok = false
		return
	}
	const maxChildren = 200
	if len(pushedCommit.Children) > maxChildren {
		_ = cId.WriteErrMsg("too many children")
		done = true
		ok = false
		return
	}

	// Get local commit
	localCommitExists := pushedCommit.HasServerL
	var latestLocalCommit commit.Commit
	if localCommitExists {
		latestLocalCommit, _, err = l.GetLatestCommitByLocalId(
			s.RepoId, pushedCommit.ServerL)
		if err != nil {
			_ = cId.WriteErrMsg(serverErrMsg)
			done = true
			ok = false
			return
		}
	}
	if localCommitExists && latestLocalCommit.IsSubmitted {
		errMsg := fmt.Sprintf(
			"commit %d is already submitted and can't be modified",
			latestLocalCommit.L)
		_ = cId.WriteErrMsg(errMsg)
		done = true
		ok = false
		return
	}
	localParentOfPushedCommit, isNotFoundErr, err := l.GetCommitVersionByServerId(
		s.RepoId, pushedCommit.ParentServerL, pushedCommit.ParentServerV)
	if isNotFoundErr {
		_ = cId.WriteErrMsg("parent not found")
		done = true
		ok = false
		return
	}
	if err != nil {
		_ = cId.WriteErrMsg(serverErrMsg)
		done = true
		ok = false
		return
	}
	baseCommit, isNotFoundErr, err := l.GetCommitVersionByServerId(
		s.RepoId, baseCommitServerId, baseCommitServerV)
	if isNotFoundErr {
		_ = cId.WriteErrMsg("base not found")
		done = true
		ok = false
		return
	}
	if err != nil {
		_ = cId.WriteErrMsg(serverErrMsg)
		done = true
		ok = false
		return
	}

	// In the current implementation, we must read all the files trasnfered
	// even if they are equivalent to a tree already present here.
	// We might change this in the future
	newTreeVersion, newTreeHash, err := s.r.SaveDelta(
		pushedCommitIt,
		baseCommit.TreeVersion,
		l)
	if err != nil && !errors.Is(err, repo.ErrNoChange) {
		_ = cId.WriteErrMsg(serverErrMsg)
		done = true
		ok = false
		return
	}
	if errors.Is(err, repo.ErrNoChange) {
		newTreeVersion = baseCommit.TreeVersion
		newTreeHash = baseCommit.RootDirHash
	}

	newLocalCommit, usedNextLocalId := commit.NewLocal(
		/*isOnServer*/ true,
		pushedCommit,
		newTreeVersion,
		newTreeHash,
		localCommitExists,
		&latestLocalCommit,
		&localParentOfPushedCommit,
		s.NextLocalId,
	)
	if usedNextLocalId {
		s.NextLocalId++
	}

	// Call the observer before saving the commit
	if pushObs != nil {
		pushObs.OnPush(&newLocalCommit, localCommitExists, latestLocalCommit)
	}

	// The local commit must be updated first, because the pending commits
	// only uses the commit id as the key (not the version). I.e. SetCommit
	// must be called for `latestLocalCommit` before it's called for
	// `newLocalCommit`
	if localCommitExists {
		err = l.SetCommit(s.QuotaOwner, s.RepoId, latestLocalCommit)
		if err != nil {
			_ = cId.WriteErrMsg(serverErrMsg)
			done = true
			ok = false
			return
		}
	}
	err = l.SetCommit(s.QuotaOwner, s.RepoId, newLocalCommit)
	if err != nil {
		_ = cId.WriteErrMsg(serverErrMsg)
		done = true
		ok = false
		return
	}

	// Note that we purposefully don't save the parent (which now would have
	// the new commit attached) because children are only attached on submit.

	// Write the id of the new commit back to the client
	err = cId.Write(newLocalCommit.L, newLocalCommit.Version)
	if err != nil {
		done = true
		ok = false
		return
	}

	done = false
	ok = true
	return
}