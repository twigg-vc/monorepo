package owners

import (
	"context"
	"monorepo/twigg/server"
)

// MUST BE INITIALIED WITH `New`
// Service provides methods for checking if a commit passes the Owners' approval
// requirements.
type Service struct {
	s service
}

// Returns true if no owner lgtm is required for the commit's
// modified files, or if the users who LGTM'd satisfy the owner
// requirements.
// For convenience, supremeLeaders - if non-empty - are users who
// together are always considered owners of everything regardless of
// what the OWNERS files say (like repository admins).
// I.e. this function returns "true, nil" if all users in
// supremeLeaders are present in usersWhoLgtmd.
func (s Service) OwnersLgmtIsOk(repoId uint64, commitId uint64,
	usersWhoLgtmd []string,
	commitIdToReadOwners uint64,
	supremeLeaders []string,
	r context.Context) (bool, error) {
	return s.s.OwnersLgmtIsOk(repoId, commitId, usersWhoLgtmd, commitIdToReadOwners, supremeLeaders, r)
}

func New(repo ServerProvider) Service {
	return Service{service{sp: repo}}
}

type ServerProvider interface {
	GetServerByRepoId(rl context.Context,
		repoId uint64) (server.Server, error)
	GetServerRead(rl context.Context) server.Read
}

const (
	OwnersFileName    = "OWNERS"
	MaxOwnersFileSize = 512 * 1024 // 512 KB
)
