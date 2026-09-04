package cli

import (
	"monorepo/twigg/commit"
)

func (a *app) rebase(args commandArgs) {
	source, target, ok := a.getRebaseSourceAndTarget(args)
	if !ok {
		return
	}
	if source.IsDetached {
		a.logError(commitIsDetached)
		return
	}
	rebased, err := a.ag.Rebase(&source, &target,
		/*isAutoRebaseOfChildren*/ false,
		/*rebaseChildren*/ true, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}

	if args.dryRun {
		if rebased.HasRebaseConflicts {
			a.logWarning(rebaseWillConflict)
		} else {
			a.logSuccess(rebaseWillSucceed)
		}
		a.db.PreventCommit(a.dbWrite)
		return
	}

	// The current commit might have changed, so goto it
	current, err := a.ag.GetLatest(a.s.Current.L, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	err = a.gotoCommit(current)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if rebased.HasRebaseConflicts {
		a.logWarning(gotConflict)
	} else {
		a.logSuccess(rebaseOk)
	}
}

// On errors, logs err and returns false
func (a *app) getRebaseSourceAndTarget(args commandArgs) (source, target commit.Commit, ok bool) {
	source = a.s.Current
	var sourceOk bool
	var targetOk bool

	// args = [target]
	if args.commit1 == "" {
		target, targetOk = a.parseCommit(args.commit0)
		if !targetOk {
			return
		}
	}
	// args = [source, target]
	if args.commit1 != "" {
		source, sourceOk = a.parseCommit(args.commit0)
		if !sourceOk {
			return
		}
		target, targetOk = a.parseCommit(args.commit1)
		if !targetOk {
			return
		}
	}
	if target.Status != commit.StatusLatest ||
		source.Status != commit.StatusLatest {
		a.logError(rebaseWithObsolete)
		return
	}
	if source.L == 0 {
		a.logError(rebaseRoot)
		return
	}
	if target.HasRebaseConflicts {
		a.logError(cantRebaseIntoConflicts)
		return
	}
	if source.ParentL == target.L && source.ParentV == target.Version {
		a.logError(rebaseIntoParent)
		return
	}
	if source.L == target.L {
		a.logError(rebaseIntoSelf)
		return
	}
	ok = true
	return
}
