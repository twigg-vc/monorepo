package cli

import (
	"fmt"
	"monorepo/twigg/commit"
)

func (a *app) restore(args commandArgs) {
	if !a.commitHasExplicitVersion(args.commit0) {
		a.logError(commitVersionNotProvided)
		return
	}
	oldVersion, ok := a.parseCommit(args.commit0)
	if !ok {
		return
	}
	if oldVersion.Status == commit.StatusLatest {
		a.logError(fmt.Sprintf("%d is the latest version of %s",
			oldVersion.Version, args.commit0))
		return
	}
	latest, err := a.ag.GetLatest(oldVersion.L, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if latest.IsSubmitted {
		a.logError(cantRestoreSubmitted)
		return
	}
	restored, err := a.ag.Restore(&latest, oldVersion, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}

	// Read back the current commit as it might have changed
	a.s.Current, err = a.ag.GetVersion(a.s.Current.L, a.s.Current.Version, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	err = a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}

	// If the current commit used to be the latest one and the workdir is
	// clean, go-to the restored one as that is now the actual latest version
	wdIsClean, _, err := a.workdirIsClean()
	if err != nil {
		a.logError(err.Error())
		return
	}
	if a.s.Current.L == latest.L &&
		a.s.Current.Version == latest.Version &&
		wdIsClean {
		err = a.gotoCommit(restored)
		if err != nil {
			a.logError(err.Error())
			return
		}
	}

	a.logSuccess(restoredCommit(oldVersion.Version, restored, true, a.s.ServerUrl))
}
