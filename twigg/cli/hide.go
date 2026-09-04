package cli

import (
	"fmt"
	"monorepo/twigg/commit"
)

func (a *app) hide(args commandArgs) {
	c, ok := a.parseCommit(args.commit0)
	if !ok {
		return
	}
	if c.Status == commit.StatusObsolete {
		// It should be impossible to do this because the argument to hide
		// a commit is just the commit id, so it'll always be the latest
		// version of a commit; which is never obsolete
		panic("tried to hide obsolete commit")
	}
	if c.IsHidden {
		a.logError(fmt.Sprintf("%s %v",
			commitStringByLAndV(c.L, c.Version,
				c.HasServerL, c.ServerL,
				c.HasServerV, c.ServerV, true,
				a.supportsHyperlinks, a.s.ServerUrl,
				/*onlyShowServerId*/ false),
			alreadyHidden))
		return
	}
	if c.IsSubmitted {
		a.logError(cantHideSubmitted)
		return
	}
	err := a.ag.Hide(&c, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if c.L == a.s.Current.L {
		a.s.Current = c
		err = a.saveState()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	a.logSuccess(hidCommit(c, a.supportsHyperlinks, a.s.ServerUrl))
}
func (a *app) unhide(args commandArgs) {
	c, ok := a.parseCommit(args.commit0)
	if !ok {
		return
	}
	if !c.IsHidden {
		a.logError(fmt.Sprintf("commit %s %v",
			commitStringByLAndV(c.L, c.Version, c.HasServerL,
				c.ServerL, c.HasServerV, c.ServerV, true,
				a.supportsHyperlinks, a.s.ServerUrl,
				/*onlyShowServerId*/ false),
			notHidden))
		return
	}
	err := a.ag.Unhide(&c, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	if c.L == a.s.Current.L {
		a.s.Current = c
		err = a.saveState()
		if err != nil {
			a.logError(err.Error())
			return
		}
	}
	a.logSuccess(unhideCommit(c, a.supportsHyperlinks, a.s.ServerUrl))
}