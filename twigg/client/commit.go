package client

import (
	"errors"
	"fmt"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
	"strconv"
	"unicode/utf8"
)

var (
	errMsgTooShort = errors.New("title can't be empty")
)

func errMsgTooLong(msg string) error {
	return fmt.Errorf("title can't be > %v. got: %v", strconv.Itoa(MaxMsgLen), utf8.RuneCountInString(msg))
}

func (a tw) GetLatest(id commit.LocalId, lock Read) (commit.Commit, error) {
	c, isNotFoundErr, err := lock.GetLatestCommitByLocalId(a.RepoId, id)
	if isNotFoundErr {
		err = ErrCommitNotFound
	}
	return c, err
}

func (a tw) GetVersion(id commit.LocalId, v uint64, l Read) (commit.Commit, error) {
	c, isNotFoundErr, err := l.GetCommitVersionByLocalId(a.RepoId, id, v)
	if isNotFoundErr {
		err = ErrCommitNotFound
	}
	return c, err
}

func (a tw) GetLatestByServerId(sId commit.LocalId, l Read) (commit.Commit, error) {
	c, isNotFoundErr, err := l.GetLatestCommitByServerId(a.RepoId, sId)
	if isNotFoundErr {
		err = ErrCommitNotFound
	}
	return c, err
}
func (a tw) GetVersionByServerId(sId commit.LocalId, sV uint64, l Read) (commit.Commit, error) {
	c, isNotFoundErr, err := l.GetCommitVersionByServerId(a.RepoId, sId, sV)
	if isNotFoundErr {
		err = ErrCommitNotFound
	}
	return c, err
}

func (a *tw) Commit(wd tree.Root, message string, parent *commit.Commit, lock Write) (c commit.Commit, err error) {
	if message == "" {
		return c, errMsgTooShort
	}
	if utf8.RuneCountInString(message) > MaxMsgLen {
		return c, errMsgTooLong(message)
	}

	// Save the workdir to the repository
	treeV, hash, err := a.repo.Save(wd, parent.TreeVersion, lock)
	if errors.Is(err, repo.ErrNoChange) {
		return c, ErrNothingToCommit
	}
	if err != nil {
		return
	}
	if hash == parent.RootDirHash {
		panic("same hash but din't get ErrNoChange error")
	}
	diffCounts, err := tree.CountDiffs(
		a.repo.Root(treeV, lock),
		a.repo.Root(parent.TreeVersion, lock))
	if err != nil {
		return
	}
	// Create the commit instance
	c = commit.NewOriginal( /*isOnServer=*/ false,
		a.NextLocalId, treeV, hash, message, parent, diffCounts,
	)
	a.NextLocalId++
	defer func() {
		if err != nil {
			a.NextLocalId--
		}
	}()
	// Save the commit and the parent
	err = lock.SetCommit(a.QuotaOwner, a.RepoId, *parent)
	if err != nil {
		return
	}
	err = lock.SetCommit(a.QuotaOwner, a.RepoId, c)
	if err != nil {
		return
	}

	// Save the client
	err = a.save(lock)
	return
}

func (a *tw) Init(lock Write) (c commit.Commit, err error) {
	if a.NextLocalId > 0 {
		err = errors.New("already created")
		return
	}
	var hash [32]byte
	_, hash, err = a.repo.Init(lock)
	if err != nil {
		return
	}
	c = commit.NewRoot( /*isOnServer=*/ false, hash)
	if c.L != 0 {
		panic("expected root to have id=0")
	}
	a.NextLocalId++
	defer func() {
		if err != nil {
			a.NextLocalId--
		}
	}()
	err = lock.SetCommit(a.QuotaOwner, a.RepoId, c)
	if err != nil {
		return
	}
	err = a.save(lock)
	return
}
