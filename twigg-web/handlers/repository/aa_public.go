package repository

import (
	"context"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
)

func AddHandlers(
	rSrv RepoService,
	revSrv ReviewService,
	userSrv UserService,
	readMux wrappers.UserWithReadPermissionMux) {
	h := NewHandler(rSrv, revSrv, userSrv)
	readMux.HandleFuncR("GET "+routes.RepoPattern, h.handleGet)
	readMux.HandleFuncR("GET "+routes.RepoLoadMoreSubmitted,
		h.handleGetMoreSubmitted)
	readMux.HandleFuncR("GET "+routes.RepoLoadMorePending,
		h.handleGetMorePending)
	readMux.HandleFuncR("GET "+routes.RepoSearchCommitsPattern,
		h.HandleSearchCommits)
	readMux.HandleFuncR("GET "+routes.RepoTwiggDocPattern,
		h.handleGetTwiggDoc)
}

func NewHandler(rSrv RepoService,
	revSrv ReviewService,
	userSrv UserService) handler {
	return handler{
		rSrv:    rSrv,
		revSrv:  revSrv,
		userSrv: userSrv,
	}
}

type RepoService interface {
	GetRepoTopCommit(rl context.Context, repoId uint64) (commit.Commit, error)
	GetRepoPendingCommits(rl context.Context, repoId uint64, ascendingOrder bool) (iterator.I[commit.Commit], error)
	GetRepoPendingCommitsAfter(rl context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error)
	GetRepoCommit(rl context.Context, repoId uint64, n commit.LocalId) (commit.Commit, error)
	GetRepoCommitVersion(rl context.Context, repoId uint64, n commit.LocalId, v uint64) (commit.Commit, error)
	GetRepoFile(rl context.Context, repoId uint64, a commit.Commit, filename string, w io.Writer) error
}

type ReviewService interface {
	GetData(r context.Context, repoId uint64, cId commit.LocalId, checkOwners bool,
		cIdToReadOwners uint64, supremeLeaders []string) (d review.Data, isNotFoundErr bool, err error)
	ResolveSupremeLeaders(db context.Context, ownerUsr user.User) ([]string, error)
}

type UserService interface {
	Get(r context.Context, id int64) (u user.User, isNotFoundErr bool, err error)
}