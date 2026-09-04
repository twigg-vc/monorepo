package client

import (
	"errors"
	"monorepo/twigg/commit"
)

func (a tw) Restore(newCommit *commit.Commit, oldCommit commit.Commit, l Write) (commit.Commit, error) {
	if newCommit.Status == commit.StatusObsolete {
		panic("tried to restore an obsolete commit")
	}
	if oldCommit.Status == commit.StatusLatest {
		panic("tried to restore to a non obsoelte commit")
	}
	if newCommit.IsSubmitted {
		return commit.Commit{},
			errors.New("can't restore old version of submitted commit")
	}
	// Get the parent
	p, err := a.GetVersion(oldCommit.ParentL, oldCommit.ParentV, l)
	if err != nil {
		return commit.Commit{}, err
	}
	// Create the restored
	restored := commit.NewRestoreCommit( /*isOnServer=*/ false,
		newCommit, oldCommit, &p)
	// Save the parent as it'll now have a new child
	err = l.SetCommit(a.QuotaOwner, a.RepoId, p)
	if err != nil {
		return commit.Commit{}, err
	}
	// Rebase the children of the old one into the restored.
	err = a.RebaseChildren(oldCommit, &restored, l)
	if err != nil {
		return commit.Commit{}, err
	}
	// Save the newCommit as it has been modified
	err = l.SetCommit(a.QuotaOwner, a.RepoId, *newCommit)
	if err != nil {
		return commit.Commit{}, err
	}
	// Save the restored now as it'll have the children
	err = l.SetCommit(a.QuotaOwner, a.RepoId, restored)
	if err != nil {
		return commit.Commit{}, err
	}
	return restored, nil
}