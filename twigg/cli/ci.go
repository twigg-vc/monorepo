package cli

import (
	"monorepo/twigg/commit"
)

func (a *app) ciList(args commandArgs) {
	targetCommit := a.s.Current
	if args.commit0 != "" {
		var parseCommitOk bool
		targetCommit, parseCommitOk = a.parseCommit(args.commit0)
		if !parseCommitOk {
			return
		}
	}
	if targetCommit.L == commit.RootCommitId {
		a.logSuccess(noCiWillRun)
		return
	}
	parent, err := a.ag.GetVersion(targetCommit.ParentL, targetCommit.ParentV, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	const ciFilename = "CI.json"
	files, err := a.ag.SearchFileInChangedDirs(targetCommit, parent, a.wl, ciFilename)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if !files.CanGet() {
		a.logSuccess(noCiWillRun)
		return
	}
	a.logInfo(ciListHeader(targetCommit, a.supportsHyperlinks, a.s.ServerUrl))
	for files.CanGet() {
		_, _, isDeleted, path, _, _, _, _ := files.GetFile()
		if !isDeleted {
			a.logSuccess(path)
		}
		err = files.Next()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
}
