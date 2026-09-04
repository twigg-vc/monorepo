package client

import (
	"monorepo/twigg/commit"
)

func (a *tw) Hide(c *commit.Commit, lock Write) error {
	c.IsHidden = true
	return lock.SetCommit(a.QuotaOwner, a.RepoId, *c)
}
func (a *tw) Unhide(c *commit.Commit, lock Write) error {
	c.IsHidden = false
	return lock.SetCommit(a.QuotaOwner, a.RepoId, *c)
}