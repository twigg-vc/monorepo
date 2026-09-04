package client

import (
	"errors"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
	"unicode/utf8"
)

func (a *tw) Amend(c *commit.Commit, rebaseChildren bool, wd tree.Root, message string, lock Write) (newC commit.Commit, err error) {
	if c.IsSubmitted {
		err = errors.New("cant ammend submitted commits")
		return
	}
	if c.Status == commit.StatusObsolete {
		err = errors.New("can't amend obsolete commit")
		return
	}
	if utf8.RuneCountInString(message) > MaxMsgLen {
		err = errMsgTooLong(message)
		return
	}
	// Get the parent
	p, err := a.GetVersion(c.ParentL, c.ParentV, lock)
	if err != nil {
		return
	}
	// Save the new tree
	newTreeV, hash, err := a.repo.Save(wd, c.ParentTreeVersion, lock)
	if err != nil && !errors.Is(err, repo.ErrNoChange) {
		return
	}
	if hash == c.RootDirHash && c.Message == message {
		err = ErrNothingToCommit
		return
	}
	latestC, err := a.GetLatest(c.L, lock)
	if err != nil {
		return
	}
	if latestC.Version != c.Version {
		panic("amend called with out-of-date commit")
	}
	diffCounts, err := tree.CountDiffs(
		a.repo.Root(newTreeV, lock),
		a.repo.Root(p.TreeVersion, lock))
	if err != nil {
		return
	}
	// Create the new commit
	newC = commit.NewAmend(
		newTreeV, hash, message, c, &p /*isOnServer=*/, false, nil, diffCounts)
	// Save all the commits. The old and parent are modified as well
	err = lock.SetCommit(a.QuotaOwner, a.RepoId, *c)
	if err != nil {
		return
	}
	err = lock.SetCommit(a.QuotaOwner, a.RepoId, p)
	if err != nil {
		return
	}
	// Rebase the children if needed
	if rebaseChildren {
		err = a.RebaseChildren(*c, &newC, lock)
		if err != nil {
			return
		}
	}
	// Now save the new commit (it will be modified if rebaseChildren)
	err = lock.SetCommit(a.QuotaOwner, a.RepoId, newC)
	if err != nil {
		return
	}

	return
}
