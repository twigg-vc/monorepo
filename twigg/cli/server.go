package cli

import (
	"errors"
	"fmt"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
)

func (a *app) setServerUrl(args commandArgs) {
	a.s.ServerUrl = a.twiggWebUrl + args.serverPath

	err := a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(setServerUrlOk(a.s.ServerUrl))
}

func (a *app) setUnsafeServerUrl(args commandArgs) {
	if !a.s.EnableUnsafeDevMode {
		a.logError(invalidCommand)
		return
	}
	a.s.ServerUrl = args.unsafeServerUrl

	err := a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(setServerUrlOk(a.s.ServerUrl))
}
func (a *app) setEnableUnsafeDevModeCmd(args commandArgs) {
	a.s.EnableUnsafeDevMode = args.boolInFirstArg
	err := a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(setEnableUnsafeDevModeOk(a.s.EnableUnsafeDevMode))
}

func (a *app) setApiKey(args commandArgs) {
	a.s.ApiKey = args.apiKey
	err := a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(apiKeyConfiguredOk)
}
func (a *app) dumpApiKey(args commandArgs) {
	a.logSuccess(a.s.ApiKey)
}

func (a *app) setServerId(args commandArgs) {
	if a.s.ServerUrl == "" {
		a.logError(serverUrlNotSet)
		return
	}
	if a.s.ApiKey == "" {
		a.logError(apiKeyNotSet)
		return
	}
	notOkMsg, err := a.ag.SetNextServerId(a.s.ServerUrl, a.s.ApiKey,
		args.serverId)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if notOkMsg != "" {
		a.logError(notOkMsg)
		return
	}
	a.logSuccess(setServerIdOk(args.serverId))
}

func (a *app) push(args commandArgs) {
	if a.s.ServerUrl == "" {
		a.logError(serverUrlNotSet)
		return
	}
	if a.s.ApiKey == "" {
		a.logError(apiKeyNotSet)
		return
	}
	if a.s.Current.IsOnServer() {
		a.logError(currentCommitAlreadyPushed)
		return
	}
	isObsParentErr, isBadApiKeyErr, err := a.ag.Push(
		&a.s.Current, a.s.ServerUrl,
		a.s.ApiKey, a.wl)
	if isBadApiKeyErr {
		a.logError(badApiKey)
		return
	}
	if isObsParentErr {
		a.logError(cantPushWithObsParent)
		return
	}
	if err != nil {
		a.logError(err.Error())
		return
	}
	// Update current as it will have changed
	err = a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}

	a.logSuccess(pushOk)
}

// Executed after every commit is pulled
func (a *app) onPull(pulled commit.Commit, hasLocal bool, local commit.Commit) error {
	a.logInfo(pulledCommit(pulled, a.supportsHyperlinks, a.s.ServerUrl))
	return nil
}

func (a *app) pull(args commandArgs) {
	if a.s.ServerUrl == "" {
		a.logError(serverUrlNotSet)
		return
	}
	if a.s.ApiKey == "" {
		a.logError(apiKeyNotSet)
		return
	}
	if args.commit0InServerSyntax != "" {
		a.pullCommit(args)
		return
	}

	// Pull everything after the last submitted available locally
	top, err := a.getTopCommit()
	if err != nil {
		a.logError(err.Error())
		return
	}
	isBadApiKeyErr, isOldProtocolErr, err := a.ag.PullAllSubmittedAfter(
		top, a.s.ServerUrl, a.s.ApiKey, a.onPull, a.wl)
	if isBadApiKeyErr {
		a.logError(badApiKey)
		return
	}
	if isOldProtocolErr {
		a.logError(updateRequired)
		return
	}
	if err != nil && !errors.Is(err, client.ErrNothingToPull) {
		a.logError(err.Error())
		return
	}
	if errors.Is(err, client.ErrNothingToPull) {
		a.logSuccess(nothingToPull)
		return
	}

	// If the workdir is clean and !stay, go-to the newest version of the
	// original current commit
	wdIsClean := false
	if !args.stay {
		wdIsClean, _, err = a.workdirIsClean()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	if wdIsClean {
		newC, err := a.ag.GetLatest(a.s.Current.L, a.wl)
		if err != nil {
			a.logError(err.Error())
			return
		}
		err = a.gotoCommit(newC)
		if err != nil {
			a.logError(err.Error())
			return
		}
		a.logSuccess(pullOk)
		return
	}
	ok := a.refreshAndSaveCurrent()
	if !ok {
		return
	}
	a.logSuccess(pullOk)
}
func (a *app) refreshAndSaveCurrent() (ok bool) {
	newC, err := a.ag.GetVersion(
		a.s.Current.L, a.s.Current.Version, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.s.Current = newC
	err = a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}
	return true
}

func (a *app) pullCommit(args commandArgs) {
	var pulledCommitServerId uint64
	onPull := func(pulled commit.Commit, hasLocal bool, local commit.Commit) error {
		pulledCommitServerId = pulled.ServerL
		return a.onPull(pulled, hasLocal, local)
	}

	// Base should be as "close" to the target commit as possible, to reduce
	// the amount of data transfered.
	base := a.s.Current
	var err error
	if !base.IsDetached {
		base, err = a.getTopCommit()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}

	var isBadApiKeyErr bool
	var isOldProtocolErr bool
	if args.commit0InServerSyntax == topCommitAlias {
		isBadApiKeyErr, isOldProtocolErr, err = a.ag.PullTopCommit(base,
			a.s.ServerUrl, a.s.ApiKey, onPull, a.wl)
	} else {
		match, id, hasVersion, version := a.tryParseServerCommitString(
			args.commit0InServerSyntax)
		if !match {
			a.logError(badCommitServerSyntax)
			return
		}
		// If possible, get another version of the specified commit as it'll be
		// closer than the top commit
		latestVersionWithServerId, err2 := a.ag.GetLatestByServerId(
			id, a.wl)
		if err2 != nil && !errors.Is(err2, client.ErrCommitNotFound) {
			a.logError(err2.Error())
			return
		}
		if err2 == nil {
			if hasVersion && latestVersionWithServerId.Version == version {
				a.logSuccess(nothingToPull)
				return
			}
			base = latestVersionWithServerId
			// The latest version with the server ID might not have been pushed
			// yet. Since it's on the server though we know for sure that there
			// is some previous version that is on the server, so we look for it
			// if needed
			for !base.IsOnServer() {
				base, err = a.ag.GetVersion(base.L, base.Version-1, a.wl)
				if err != nil {
					a.logError(err.Error())
					return
				}
			}
		}
		if hasVersion {
			a.logDebug(fmt.Sprintf("will pull c%dv%d with c%dv%d as base",
				id, version, base.ServerL, base.ServerV))
		} else {
			a.logDebug(fmt.Sprintf("will pull c%dv with c%dv%d as base",
				id, base.ServerL, base.ServerV))
		}
		isBadApiKeyErr, isOldProtocolErr, err = a.ag.PullCommit(id, hasVersion, version, base,
			a.s.ServerUrl, a.s.ApiKey, onPull, a.wl)
		a.logDebug("done pulling")
	}
	if isBadApiKeyErr {
		a.logError(badApiKey)
		return
	}
	if isOldProtocolErr {
		a.logError(updateRequired)
		return
	}
	if err != nil && !errors.Is(err, client.ErrNothingToPull) {
		a.logError(err.Error())
		return
	}
	if errors.Is(err, client.ErrNothingToPull) {
		a.logSuccess(nothingToPull)
		return
	}
	pulled, err := a.ag.GetLatestByServerId(pulledCommitServerId, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	// Update the current if we pulled a new version
	if pulledCommitServerId == a.s.Current.ServerL {
		a.s.Current = pulled
		err = a.saveState()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	if args.stay {
		_ = a.refreshAndSaveCurrent()
		return
	}
	// Go-to the pulled commit if the workdir is clean
	a.logDebug("will check if wd is clean")
	wdIsClean, _, err := a.workdirIsClean()
	a.logDebug("done checking if wd is clean")
	if err != nil {
		a.logError(err.Error())
		return
	}
	if wdIsClean {
		a.logDebug("will load pulled commit")
		err = a.gotoCommit(pulled)
		a.logDebug("done loading pulled commit")
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	a.logSuccess(pullOk)
}