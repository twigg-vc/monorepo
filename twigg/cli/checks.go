package cli

import (
	"monorepo/twigg/tree"
	"strings"
)

// Returns true if args are ok. Else, logs an error and returns false.
func (a *app) checkCommandArgs() (ok bool) {
	if strings.HasPrefix(a.args.message, "-") {
		a.logError(messageCantHaveFlagPrefix)
		return
	}
	ok = true
	return
}

// Checks if the workdir is clean. If not, logs an error and returns false.
// On errors, logs the error and returns false.
func (a *app) checkCleanWorkdir() (ok bool) {
	isClean, _, err := a.workdirIsClean()
	if err != nil {
		a.logError(err.Error())
		return false
	}
	if !isClean {
		a.logError(workdirIsDirty)
		return false
	}
	return true
}
func (a *app) workdirIsClean() (isClean bool, hash [32]byte, err error) {
	diffs, err := a.ag.DiffWorkdir(a.wd, a.s.Current.TreeVersion, a.wl)
	if err != nil {
		return
	}
	var dif tree.Diff
	for diffs.CanGet() {
		dif = diffs.GetDiff()
		if dif.Data.Depth == 0 && dif.Type != tree.DiffTypeUndefined {
			hash = dif.Data.ContentHash
			break
		}
		if dif.Type == tree.DiffTypeNoChange {
			diffs.SkipChildrenOnNext()
		}
		err = diffs.Next()
		if err != nil {
			return
		}
	}
	isClean = dif.Type == tree.DiffTypeNoChange
	return
}
