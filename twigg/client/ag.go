package client

import (
	"monorepo/twigg/repo"
)

type tw struct {
	NextLocalId uint64
	QuotaOwner  string
	RepoId      uint64
	repo        repo.Repo
}

func newTw(repoOwner string, repoId uint64, l Read) (*tw, error) {
	a := &tw{
		NextLocalId: 0,
		RepoId:      repoId,
		QuotaOwner:  repoOwner,
		repo:        repo.New(repoOwner, repoId),
	}
	n, isNotFoundErr, err := l.GetRepoNextLocalId(repoId)
	if isNotFoundErr {
		return a, nil
	}
	a.NextLocalId = n
	if err != nil {
		return nil, err
	}
	return a, nil
}
func (a tw) IsInit() bool {
	return a.NextLocalId > 0
}

func (a tw) save(l Write) error {
	return l.SetRepoNextLocalId(a.RepoId, a.NextLocalId)
}