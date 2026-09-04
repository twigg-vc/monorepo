package cli

import (
	"errors"
	"monorepo/twigg/client"
	"monorepo/twigg/filter"
	"monorepo/twigg/tree"
)

// creates and commits lock
func (a *app) amend(args commandArgs) {
	msg := args.message
	if msg == "" {
		msg = a.s.Current.Message
	}

	if a.s.Current.HasRebaseConflicts && !args.force {
		hasUnresolvedConflict, ok := a.logUnresolvedConflicts(
			/*alsoLogResolved*/ false)
		if !ok {
			return
		}
		if hasUnresolvedConflict {
			a.logError(cantAmendWithConflicts)
			return
		}
	}

	var root tree.Root
	if len(args.pathsToFilter) == 0 {
		root = a.wd
	} else {
		root = filter.Filter(a.wd, args.pathsToFilter, a.ag.GetRoot(a.s.Current, a.wl))
	}
	oldCopy := a.s.Current
	ammended, err := a.ag.Amend(&a.s.Current,
		/*rebaseChildren*/ true, root, msg, a.wl)
	if errors.Is(err, client.ErrNothingToCommit) {
		a.logError(nothingToCommit)
		return
	}
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.s.Current = ammended
	err = a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}

	a.logSuccess(ammendedCommit(
		oldCopy, a.s.Current, a.supportsHyperlinks, a.s.ServerUrl))
}

// Logs all files that have unresolved conflicts with logWarning.
// If `alsoLogResolved`, will also log the resolved conflicts with logSuccess.
// Returns ok=false and logs an error if any happen
func (a *app) logUnresolvedConflicts(alsoLogResolved bool) (hasUnresolvedConflict bool, ok bool) {

	rt := a.ag.GetRoot(a.s.Current, a.wl)
	iter, err := tree.Walk(rt)
	if err != nil {
		a.logError(err.Error())
		return
	}
	for iter.CanGet() {
		path, _, _, tr := iter.Get()
		if !tr.DataIsComplete() {
			panic("current commit root should be fully computed")
		}

		if tr.Data().IsDir {
			// Skip dirs without children with conflicts
			if !tr.Data().HasChildWithConflicts {
				iter.SkipChildrenOnNext()
			}
			err = iter.Next()
			if err != nil {
				a.logError(err.Error())
				return
			}
			continue
		}

		// If it's a file without conflicts, just continue
		if !tr.Data().HasConflicts {
			err = iter.Next()
			if err != nil {
				a.logError(err.Error())
				return
			}
			continue
		}

		// Check if solved. If not, log warning.
		// Continue to log all non resolved cases.
		hasConflicts, err := a.wd.FileHasConflict(path)
		if err != nil {
			a.logError(err.Error())
			return
		}
		if hasConflicts {
			a.logWarning(fileHasUnresolvedConflicts(path))
			hasUnresolvedConflict = true
		} else {
			if alsoLogResolved {
				a.logSuccess(fileHasResolvedConflicts(path))
			}
		}
		err = iter.Next()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	ok = true
	return
}