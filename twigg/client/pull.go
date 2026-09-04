package client

import (
	"errors"
	"fmt"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/xchange"
	"net/http"
	"net/url"
)

func (a *tw) PullAllSubmittedAfter(base commit.Commit,
	url_ string, apiKey string,
	onPull onPullCallback, l Write) (isBadApiKeyErr bool, isOldProtocolErr bool, err error) {
	if !base.IsOnServer() {
		// Base must be fully known on the server
		panic("passed a commit as pase that is not known on the server")
	}
	if onPull == nil {
		onPull = func(pulledCommit commit.Commit, hasLocalCommit bool,
			localCommit commit.Commit) error {
			return nil
		}
	}

	params := url.Values{}
	params.Add(BaseCommitServerIdQueryParam, fmt.Sprintf("%d", base.ServerL))
	params.Add(BaseCommitServerVersionQueryParam, fmt.Sprintf("%d", base.ServerV))
	urlWithParams := url_ + PullEndpoint + "?" + params.Encode()
	req, err := http.NewRequest("GET", urlWithParams, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create GET request to pull: %s", err))
	}
	xchange.SetTwiggHeaderInRequest(req)
	xchange.SetApiKeyHeader(apiKey, req)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, fmt.Errorf("%w: %s", ErrFailedToReachServer, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, false, fmt.Errorf("%w: %s", ErrFailedToReachServer, resp.Status)
	}
	if !xchange.MightBeTwiggResponse(resp) {
		return false, false, ErrNotTwiggServer
	}

	originalNextLocalId := a.NextLocalId
	defer func() {
		if err != nil {
			a.NextLocalId = originalNextLocalId
			return
		}
		if a.NextLocalId == originalNextLocalId {
			return
		}
		err = a.save(l)
		if err != nil {
			a.NextLocalId = originalNextLocalId
			return
		}
	}()

	cr, cl, err := xchange.NewCommitReader(resp.Body)
	defer cl()
	if err != nil {
		isOldProtocolErr = errors.Is(err, xchange.ErrOldProtocol)
		return
	}

	err = a.handlePulledCommit(onPull, l, cr)
	if err == io.EOF {
		err = ErrNothingToPull
		return
	}
	if err != nil && err.Error() == xchange.BadApiKeyErrMsg {
		isBadApiKeyErr = true
		return
	}
	// Keep pulling after the first one if there are more
	for err == nil {
		err = a.handlePulledCommit(onPull, l, cr)
	}
	if err == io.EOF {
		err = nil
	}

	return
}

func (a *tw) PullCommit(commitServerId uint64,
	hasServerVersion bool, commitServerVersion uint64,
	base commit.Commit,
	url_ string, apiKey string,
	onPull onPullCallback,
	l Write) (bool, bool, error) {
	commitServerVersionStr := ""
	if hasServerVersion {
		commitServerVersionStr = fmt.Sprintf("%d", commitServerVersion)
	}
	return a.pullCommit(
		fmt.Sprintf("%d", commitServerId),
		commitServerVersionStr,
		base,
		url_, apiKey, onPull, l,
	)
}

func (a *tw) PullTopCommit(base commit.Commit, url_ string, apiKey string,
	onPull func(
		pulledCommit commit.Commit,
		hasLocalCommit bool,
		localCommit commit.Commit) error,
	l Write) (bool, bool, error) {
	return a.pullCommit(
		DetachedLastSubmittedCommitAlias,
		"",
		base,
		url_, apiKey, onPull, l,
	)
}

func (a *tw) pullCommit(commitServerId string, commitServerVersion string,
	base commit.Commit, url_ string, apiKey string,
	onPull onPullCallback,
	l Write) (isBadApiKeyErr bool, isOldProtocolErr bool, err error) {
	if !base.IsOnServer() {
		err = errors.New("base must be on the server")
		return
	}
	if onPull == nil {
		onPull = func(pulledCommit commit.Commit, hasLocalCommit bool,
			localCommit commit.Commit) error {
			return nil
		}
	}
	params := url.Values{}
	params.Add(DetachedCommitServerIdQueryParam, commitServerId)
	params.Add(DetachedCommitServerVersionQueryParam, commitServerVersion)
	params.Add(BaseCommitServerIdQueryParam, fmt.Sprintf("%d", base.ServerL))
	params.Add(BaseCommitServerVersionQueryParam, fmt.Sprintf("%d", base.ServerV))
	params.Add(IsDetachedPullQueryParamName, IsDetachedPullQueryParamValue)
	urlWithParams := url_ + PullEndpoint + "?" + params.Encode()
	req, err := http.NewRequest("GET", urlWithParams, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create GET request to pull detached: %s", err))
	}
	xchange.SetTwiggHeaderInRequest(req)
	xchange.SetApiKeyHeader(apiKey, req)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, fmt.Errorf("%w: %s", ErrFailedToReachServer, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, false, fmt.Errorf("%w: %s", ErrFailedToReachServer, resp.Status)
	}
	if !xchange.MightBeTwiggResponse(resp) {
		return false, false, ErrNotTwiggServer
	}

	originalNextLocalId := a.NextLocalId
	defer func() {
		if err != nil {
			a.NextLocalId = originalNextLocalId
			return
		}
		if a.NextLocalId == originalNextLocalId {
			return
		}
		err = a.save(l)
		if err != nil {
			a.NextLocalId = originalNextLocalId
			return
		}
	}()
	cr, cl, err := xchange.NewCommitReader(resp.Body)
	defer cl()
	if err != nil {
		isOldProtocolErr = errors.Is(err, xchange.ErrOldProtocol)
		return
	}
	err = a.handlePulledCommit(onPull, l, cr)
	if err == io.EOF {
		err = ErrNothingToPull
		return
	}
	if err != nil && err.Error() == xchange.BadApiKeyErrMsg {
		isBadApiKeyErr = true
		return
	}
	_, _, _, _, err = cr.Read()
	if err == io.EOF {
		err = nil
	}
	return
}

type onPullCallback = func(pulledCommit commit.Commit, hasLocalCommit bool,
	localCommit commit.Commit) error

// The children of a pulled commit might have been pulled before it by a
// detached pull, in which case they are still detached and must now be attached
// to `local` (the local copy of `incoming`)
func (a *tw) attachDetachedChildren(incoming commit.Commit,
	local *commit.Commit, l Write) (attachedAny bool, err error) {
	for i, childServerL := range incoming.Children {
		childServerV := incoming.ChildrenVersions[i]
		localChild, isNotFoundErr, getChildErr := l.GetCommitVersionByServerId(
			a.RepoId, childServerL, childServerV)
		if getChildErr != nil && !isNotFoundErr {
			err = getChildErr
			return
		}
		// No work to do if the child is not here or if it is attached.
		if isNotFoundErr || !localChild.IsDetached {
			continue
		}
		localChild.AttachToLocalParent(local)
		attachedAny = true
		err = l.SetCommit(a.QuotaOwner, a.RepoId, localChild)
		if err != nil {
			return
		}
	}
	return
}

func (a *tw) handlePulledCommit(
	onPull onPullCallback, l Write, cr xchange.CommitReader) (err error) {
	incoming, baseServerId, baseServerV, newCommitIt, err := cr.Read()
	if err != nil {
		return
	}
	if !incoming.HasServerL || !incoming.HasServerV {
		err = errors.New("got commit without server ids")
		return
	}
	base, isNotFoundErr, err := l.GetCommitVersionByServerId(
		a.RepoId, baseServerId, baseServerV)
	if isNotFoundErr {
		err = errors.New("base not found")
		return
	}
	if err != nil {
		return
	}
	hasLocalCopy := false
	_, isNotFoundErr, err = l.GetCommitVersionByServerId(
		a.RepoId, incoming.ServerL, incoming.ServerV)
	if err != nil && !isNotFoundErr {
		return
	}
	hasLocalCopy = !isNotFoundErr
	var hasLatestLocal bool
	var latestLocal commit.Commit
	latestLocal, isNotFoundErr, err = l.GetLatestCommitByServerId(
		a.RepoId, incoming.ServerL)
	if err != nil && !isNotFoundErr {
		return
	}
	hasLatestLocal = !isNotFoundErr
	copyOfLatestLocal := latestLocal
	defer func() {
		if err == nil {
			err = onPull(incoming, hasLatestLocal, copyOfLatestLocal)
		}
	}()
	var incomingParentPtr *commit.Commit
	var incomingParent commit.Commit
	incomingParent, isNotFoundErr, err = l.GetCommitVersionByServerId(
		a.RepoId, incoming.ParentServerL, incoming.ParentServerV)
	if err != nil && !isNotFoundErr {
		return
	}
	if !isNotFoundErr {
		incomingParentPtr = &incomingParent
	}

	// In the current implementation we MUST read all the files being pushed,
	// even if we're pulling a commit we already have.
	newTreeVersion, newTreeHash, err := a.repo.SaveDelta(
		newCommitIt,
		base.TreeVersion,
		l)
	if err != nil && !errors.Is(err, repo.ErrNoChange) {
		return
	}
	if errors.Is(err, repo.ErrNoChange) {
		newTreeVersion = base.TreeVersion
		newTreeHash = base.RootDirHash
	}

	// If simply pulling a commit we already have, there's no more work to
	// do after we already read the iterator
	if hasLocalCopy {
		return
	}
	if hasLatestLocal && latestLocal.IsSubmitted {
		err = errors.New("can't modify submitted commits")
		return
	}
	newLocalCommit, usedNextLocalId := commit.NewLocal(
		/*isOnServer*/ false,
		incoming, newTreeVersion, newTreeHash,
		hasLatestLocal,
		&latestLocal, incomingParentPtr,
		a.NextLocalId,
	)
	if usedNextLocalId {
		a.NextLocalId += 1
	}

	_, err = a.attachDetachedChildren(incoming, &newLocalCommit, l)
	if err != nil {
		return
	}
	err = l.SetCommit(a.QuotaOwner, a.RepoId, newLocalCommit)
	if err != nil {
		return
	}
	if incomingParentPtr != nil {
		err = l.SetCommit(a.QuotaOwner, a.RepoId, *incomingParentPtr)
		if err != nil {
			return
		}
	}
	if hasLatestLocal {
		err = l.SetCommit(a.QuotaOwner, a.RepoId, latestLocal)
		if err != nil {
			return
		}
	}
	return
}