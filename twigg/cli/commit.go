package cli

import (
	"errors"
	"monorepo/twigg/client"
	"monorepo/twigg/filter"
	"monorepo/twigg/tree"
)

func (a *app) commit(args commandArgs) {
	if a.s.Current.HasRebaseConflicts {
		a.logError(cantCommitOnTopOfConflicts)
		return
	}

	var root tree.Root
	if len(args.pathsToFilter) == 0 {
		root = a.wd
	} else {
		root = filter.Filter(a.wd, args.pathsToFilter, a.ag.GetRoot(a.s.Current, a.wl))
	}
	newCommit, err := a.ag.Commit(root, args.message, &a.s.Current, a.wl)

	if errors.Is(err, client.ErrNothingToCommit) {
		a.logInfo(nothingToCommit)
		return
	}

	if err != nil {
		a.logError(err.Error())
		return
	}

	a.s.Current = newCommit
	err = a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}

	a.logSuccess(createdCommit(newCommit, a.supportsHyperlinks, a.s.ServerUrl))
}
