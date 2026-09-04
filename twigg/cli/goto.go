package cli

import (
	"fmt"
	"monorepo/twigg/commit"
)

func (a *app) up(args commandArgs) {
	n := 1
	if args.number != -1 {
		if args.number <= 0 {
			a.logError(fmt.Sprintf("%s %d is <=0", badNumberMsgPrefix, args.number))
			return
		}
		n = args.number
	}
	a.up_(n)
}

// if !gotoChild, just move the current commit and save the sate
func (a *app) up_(n int) {
	target, err := a.getNthChildCommit(n)
	if err != nil {
		a.logError(err.Error())
		return
	}
	err = a.gotoCommit(target)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(
		switchedToCommit(target, a.supportsHyperlinks, a.s.ServerUrl))
}

func (a *app) down(args commandArgs) {
	n := 1
	if args.number != -1 {
		if args.number <= 0 {
			a.logError(fmt.Sprintf("%s %d is <=0", badNumberMsgPrefix, args.number))
			return
		}
		n = args.number
	}
	a.down_(n)
}

func (a *app) down_(n int) {
	current := a.s.Current
	var target commit.Commit
	var err error
	for i := 0; i < n; i++ {
		if current.IsDetachedOrRoot() {
			a.logError(parentNotFound)
			return
		}
		target, err = a.ag.GetVersion(
			current.ParentL,
			current.ParentV, a.wl)
		if err != nil {
			a.logError(err.Error())
			return
		}
		current = target
	}
	err = a.gotoCommit(target)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(
		switchedToCommit(target, a.supportsHyperlinks, a.s.ServerUrl))
}

// commits lock
func (a *app) goto_(args commandArgs) {
	target, ok := a.parseCommit(args.commit0)
	if !ok {
		return
	}
	err := a.gotoCommit(target)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(switchedToCommit(target, a.supportsHyperlinks, a.s.ServerUrl))
}

// Simply moves the current commit, without touching the workdir
func (a *app) warp(args commandArgs) {
	target, ok := a.parseCommit(args.commit0)
	if !ok {
		return
	}
	a.s.Current = target
	err := a.saveState()
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(warpedToCommit(target, a.supportsHyperlinks, a.s.ServerUrl))
}

func (a *app) load(args commandArgs) {
	c, ok := a.parseCommit(args.commit0)
	if !ok {
		return
	}
	err := a.ag.Load(c.TreeVersion, a.wd, a.wl)
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess(loadedCommit(c, a.supportsHyperlinks, a.s.ServerUrl))
}

func (a *app) top(args commandArgs) {
	c, err := a.getTopCommit()
	if err != nil {
		a.logError(err.Error())
		return
	}

	if c.L == a.s.Current.L && c.Version == a.s.Current.Version {
		a.logWarning(alreadyAtTop)
		return
	}

	// Now just go-to c
	err = a.gotoCommit(c)
	if err != nil {
		a.logError(err.Error())
		return
	}

	a.logSuccess(switchedToCommit(c, a.supportsHyperlinks, a.s.ServerUrl))
}

// doesn't commit lock
// goes to commit and saves the state
func (a *app) gotoCommit(c commit.Commit) error {
	err := a.ag.Load(c.TreeVersion, a.wd, a.wl)
	if err != nil {
		return err
	}

	a.s.Current = c
	err = a.saveState()
	if err != nil {
		return err
	}

	return nil
}

func (a *app) getTopCommit() (commit.Commit, error) {
	c := a.s.Current
	var err error

	// Get the first commit that is submitted.
	// That's either the first one (Root commit) or some child of it.
	// If the commit is detached, we can't walk to the parent, but we can
	// keep iterating by getting one with a smaller id. In the worst case
	// we'll reach the root. This can probably be improved by storing some
	// pointer of the "detachement root" or keeping the top cached.
	for c.IsDetached && !c.IsSubmitted {
		c, err = a.ag.GetLatest(c.L-1, a.wl)
		if err != nil {
			return commit.Commit{}, err
		}
	}
	for !c.IsSubmitted {
		c, err = a.ag.GetLatest(c.ParentL, a.wl)
		if err != nil {
			return commit.Commit{}, err
		}
	}

	// Now that c is public, keep going up to find the top-most submitted
	hasSubmittedChild := true
	for hasSubmittedChild {
		hasSubmittedChild = false
		for _, childId := range c.Children {
			var child commit.Commit
			child, err = a.ag.GetLatest(childId, a.wl)
			if err != nil {
				return commit.Commit{}, err
			}
			if child.IsSubmitted {
				hasSubmittedChild = true
				c = child
				break
			}
		}
	}
	return c, nil
}

// Gets the n-th recursive child (i.e child of child of child ...)
func (a *app) getNthChildCommit(n int) (commit.Commit, error) {
	current := a.s.Current
	var c commit.Commit
	var err error
	for i := 0; i < n; i++ {
		c, err = a.getChildCommitOf(current)
		if err != nil {
			return commit.Commit{}, err
		}
		current = c
	}
	return c, nil
}
func (a *app) getChildCommitOf(c commit.Commit) (commit.Commit, error) {
	var selectedChild commit.Commit
	if len(c.Children) == 0 {
		return commit.Commit{}, fmt.Errorf(childNotFound)
	}
	for i, childId := range c.Children {
		child, err := a.ag.GetVersion(
			childId,
			c.ChildrenVersions[i], a.wl)
		if err != nil {
			return commit.Commit{}, err
		}
		// A submited child always takes utmost precedence and we stop looking
		// at the other ones
		if child.IsSubmitted {
			selectedChild = child
			break
		}
		// Target starts out as the first child of the current commit (just to
		// avoid ending up without a target)
		if i == 0 {
			selectedChild = child
			continue
		}
		// The target will be the last added child that is visible
		childIsVisible := child.Status == commit.StatusLatest && !child.IsHidden
		if childIsVisible {
			selectedChild = child
		}
	}
	return selectedChild, nil
}
