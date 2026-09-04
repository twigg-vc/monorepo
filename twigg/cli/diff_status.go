package cli

import (
	"monorepo/twigg/commit"
	"monorepo/twigg/tree"
)

// Calls onFile for every file changed by the diff, in the order they are
// walked. Logs any error and returns ok=false.
func (a *app) walkDiffs(diffs tree.ParallelIterator,
	onFile func(path string, diffType tree.DiffType)) (ok bool) {
	var diff tree.Diff
	var diffPath string
	var err error
	for diffs.CanGet() {
		diff = diffs.GetDiff()
		var visit tree.VisitStatus
		diffPath, _, visit, _ = diffs.Get()
		if diff.Type == tree.DiffTypeUndefined || visit == tree.SecondVisit {
			err = diffs.Next()
			if err != nil {
				a.logError(err.Error())
				return
			}
			continue
		}
		if diff.Type == tree.DiffTypeNoChange {
			diffs.SkipChildrenOnNext()
			err = diffs.Next()
			if err != nil {
				a.logError(err.Error())
				return
			}
			continue
		}
		if diff.Data.IsDir {
			err = diffs.Next()
			if err != nil {
				a.logError(err.Error())
				return
			}
			continue
		}
		if diff.Type != tree.DiffTypeNoChange {
			onFile(diffPath, diff.Type)
		}
		err = diffs.Next()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	ok = true
	return
}

func (a *app) logDiff(diffs tree.ParallelIterator) {
	loggedSomething := false
	ok := a.walkDiffs(diffs, func(path string, diffType tree.DiffType) {
		a.logInfo(fileStatus(path, diffType))
		loggedSomething = true
	})
	if !ok {
		return
	}
	if !loggedSomething {
		a.logInfo(noChanges)
	}
}

func (a *app) diff(args commandArgs) {
	// Diff will log diff (A-B).
	if a.args.all && a.args.json {
		a.logError(allNotSupportedWithJson)
		return
	}
	A, B, ok := a.parseDiffArgs(args)
	if !ok {
		return
	}
	if A.IsDetached {
		a.logError(instructToPullParent(A))
		return
	}
	if a.args.all {
		err := a.ag.WriteDiffAll(A.TreeVersion, B.TreeVersion, a.out, a.wl)
		if err != nil {
			a.logError(err.Error())
			return
		}
		return
	}
	diffs, err := a.ag.Diff(A.TreeVersion, B.TreeVersion, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if a.args.json {
		a.logDiffAsJson(diffs)
		return
	}
	a.logDiff(diffs)
}

// diff will log (A-B). This method parses A and B.
// On any error, logs it and returns ok=false
func (a *app) parseDiffArgs(args commandArgs) (A commit.Commit, B commit.Commit, ok bool) {
	// If no arg is provided, diff the current commit and its parent
	if args.commit0 == "" {
		A = a.s.Current
		var err error
		B, err = a.ag.GetVersion(A.ParentL, A.ParentV, a.wl)
		if err != nil {
			a.logError(err.Error())
			return
		}
		ok = true
		return
	}
	// If only the first commit is provided, diff it and its parent
	if args.commit1 == "" {
		A, ok = a.parseCommit(args.commit0)
		if !ok {
			return
		}
		var err error
		B, err = a.ag.GetVersion(A.ParentL, A.ParentV, a.wl)
		if err != nil {
			a.logError(err.Error())
			return
		}
		ok = true
		return
	}
	// In the genral case, both commits are provided
	A, ok = a.parseCommit(args.commit0)
	if !ok {
		return
	}
	B, ok = a.parseCommit(args.commit1)
	return
}

func (a *app) status(args commandArgs) {
	if a.s.Current.HasRebaseConflicts {
		a.logInfo(conflictsStatus)
		_, ok := a.logUnresolvedConflicts( /*alsoLogResolved*/ true)
		if !ok {
			return
		}
	}

	diffs, err := a.ag.DiffWorkdir(a.wd, a.s.Current.TreeVersion, a.wl)
	if err != nil {
		a.logError("Err diffing workdir: " + err.Error())
		return
	}
	a.logInfo(workdirStatus)
	a.logDiff(diffs)
	a.logInfo("\n" + commitTreeStatus)
	// Run the "log" command when done.
	// Set number=-1 to log the default number of commits.
	a.log(commandArgs{number: -1})
}