package repo

import (
	"context"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/secrets"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
)

type Service interface {
	GetById(rl context.Context, repoId uint64) (repo.Repo, error)
	GetByOwnerIdAndRepoName(rl context.Context, ownerId int64, displayName string) (r repo.Repo, isNotFoundErr bool, err error)
	GetAllByOwnerId(rl context.Context, ownerId int64) (it iterator.I[repo.Repo], err error)
	NonArchivedRepoCountIsGreaterThan(rl context.Context, ownerId int64, n int) (bool, error)

	CreateNew(wl context.Context, ownerId int64, displayName string, description string) (r repo.Repo, isAlreadyExistsErr bool, err error)

	GetRepoTopCommit(rl context.Context, repoId uint64) (commit.Commit, error)
	GetRepoPendingCommits(rl context.Context, repoId uint64, ascendingOrder bool) (iterator.I[commit.Commit], error)
	GetRepoPendingCommitsAfter(rl context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error)
	GetRepoCommit(rl context.Context, repoId uint64, n commit.LocalId) (commit.Commit, error)
	GetRepoCommitVersion(rl context.Context, repoId uint64, n commit.LocalId, v uint64) (commit.Commit, error)
	GetRepoFile(rl context.Context, repoId uint64, a commit.Commit, filename string, w io.Writer) error

	SetPublic(wl context.Context, ownerId int64, displayName string) error
	// Makes the repo readable only by users with permission.
	SetPrivate(wl context.Context, ownerId int64, displayName string) error

	SetGitMirrorEnabled(wl context.Context, ownerId int64, displayName string, enabled bool) error
	SetGitMirrorUrl(wl context.Context, repoId uint64, ownerId int64, displayName, url string) error
	GetGitMirrorUrl(rl context.Context, repoId uint64) (url string, isNotFoundErr bool, err error)

	ArchiveRepo(w context.Context, ownerId int64, repoId uint64) error
	GetArchivedRepoIds(rl context.Context, ownerId int64) (repoIds iterator.I[uint64], err error)

	GetServer(rl context.Context, ownerId int64, displayName string) (srv server.Server, isNotFoundErr bool, err error)
	GetServerByRepoId(rl context.Context, repoId uint64) (server.Server, error)
	GetServerRead(rl context.Context) server.Read
	GetServerWrite(wl context.Context) server.Write
}

type Db interface {
	CreateRepo(wl context.Context, ownerId int64, displayName, description string) (repoId uint64, err error)
	GetRepoById(rl context.Context, repoId uint64) (repo.Repo, error)
	GetRepoByOwnerIdAndName(rl context.Context, ownerId int64, displayName string) (r repo.Repo, isNotFoundErr bool, err error)
	GetReposByOwnerId(rl context.Context, ownerId int64) (iterator.I[repo.Repo], error)
	ArchiveRepo(wl context.Context, ownerId int64, repoId uint64) error
	GetArchivedRepoIds(rl context.Context, ownerId int64) (iterator.I[uint64], error)
	SetRepoPublic(wl context.Context, ownerId int64, displayName string) error
	SetRepoPrivate(wl context.Context, ownerId int64, displayName string) error
	SetRepoGitMirrorEnabled(wl context.Context, ownerId int64, displayName string, enabled bool) error
	SetRepoSanitizedGitMirrorUrl(wl context.Context, ownerId int64, displayName, sanitizedUrl string) error
	GetServerRead(rl context.Context) server.Read
	GetServerWrite(wl context.Context) server.Write
}

func NewService(db Db, secretsSrv secrets.Service) (Service, error) {
	return newService(db, secretsSrv)
}

func IsValidGitMirrorUrl(url string) bool {
	return isValidGitMirrorUrl(url)
}

const GitMirrorUrlSecretName = "git-mirror-secret-ulr"

const MaxDescriptionLength = 100