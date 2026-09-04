package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/services/secrets"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
	"net/url"
	"strconv"
	"strings"
)

func quotaOwner(userId int64) string {
	return strconv.FormatInt(userId, 10)
}

type service struct {
	db         Db
	secretsSrv secrets.Service
}

func newService(db Db, secretsSrv secrets.Service) (Service, error) {
	return service{db, secretsSrv}, nil
}

func (s service) GetById(rl context.Context, repoId uint64) (repo.Repo, error) {
	return s.db.GetRepoById(rl, repoId)
}

func (s service) ArchiveRepo(w context.Context, ownerId int64, repoId uint64) error {
	return s.db.ArchiveRepo(w, ownerId, repoId)
}
func (s service) SetPublic(wl context.Context,
	ownerId int64, displayName string) error {
	return s.db.SetRepoPublic(wl, ownerId, displayName)
}
func (s service) SetPrivate(wl context.Context,
	ownerId int64, displayName string) error {
	return s.db.SetRepoPrivate(wl, ownerId, displayName)
}
func (s service) SetGitMirrorEnabled(wl context.Context,
	ownerId int64, repoName string, enabled bool) error {
	return s.db.SetRepoGitMirrorEnabled(wl, ownerId, repoName, enabled)
}
func (s service) SetGitMirrorUrl(wl context.Context, repoId uint64, ownerId int64, displayName, url string) error {
	if url == "" {
		return errors.New("git mirror url cant be empty")
	}

	_, err := s.secretsSrv.SetRepoIdSecret(wl, repoId, GitMirrorUrlSecretName, url)
	if err != nil {
		return fmt.Errorf("failed to set git mirror url secret. err=%s", err)
	}

	sanitizedGitMirrorUrl, valid := sanitizeGitMirrorUrl(url)
	if !valid {
		return errors.New("invalid git mirror url")
	}

	return s.db.SetRepoSanitizedGitMirrorUrl(wl, ownerId, displayName, sanitizedGitMirrorUrl)
}
func (s service) GetGitMirrorUrl(rl context.Context, repoId uint64) (url string, isNotFoundErr bool, err error) {
	return s.secretsSrv.GetRepoIdSecret(rl, repoId, GitMirrorUrlSecretName)
}

func (s service) GetByOwnerIdAndRepoName(rl context.Context,
	ownerId int64, repoDisplayName string) (r repo.Repo, isNotFoundErr bool, err error) {
	return s.db.GetRepoByOwnerIdAndName(rl, ownerId, repoDisplayName)
}
func (s service) GetAllByOwnerId(rl context.Context, ownerId int64) (it iterator.I[repo.Repo], err error) {
	return s.db.GetReposByOwnerId(rl, ownerId)
}

func (s service) CreateNew(wl context.Context, ownerId int64, displayName string,
	description string) (r repo.Repo, isAlreadyExistsErr bool, err error) {

	const repoDisplayNameMaxLength = 100
	if len(displayName) > repoDisplayNameMaxLength {
		err = fmt.Errorf("can not create repo with name > %d",
			repoDisplayNameMaxLength)
		return
	}
	if len(description) > MaxDescriptionLength {
		err = fmt.Errorf("can not create repo with description > %d",
			MaxDescriptionLength)
		return
	}

	_, isNotFoundErr, err := s.GetByOwnerIdAndRepoName(wl, ownerId, displayName)
	if err != nil && !isNotFoundErr {
		return
	}
	if !isNotFoundErr {
		err = fmt.Errorf("repo ownerId: %d displayName: %s already exists",
			ownerId, displayName)
		isAlreadyExistsErr = true
		return
	}
	repoId, err := s.db.CreateRepo(wl, ownerId, displayName, description)
	if err != nil {
		return
	}

	// Set server
	srv, err := server.NewServer(quotaOwner(ownerId),
		repoId, s.db.GetServerRead(wl))
	if err != nil {
		err = fmt.Errorf("failed to get repo server: %s", err)
		return
	}
	if srv.WasInit() {
		panic(fmt.Errorf("server for repo %s already initialized", displayName))
	}
	err = srv.Init(s.GetServerWrite(wl))
	if err != nil {
		err = fmt.Errorf("failed to init repo server: %s", err)
		return
	}
	const defaultGitMirrorEnabled = false
	const newSanitizedGitMirrorUrl = ""
	const defaultIsPublic = false
	r = repo.NewRepo(repoId, ownerId, displayName, description,
		defaultGitMirrorEnabled, newSanitizedGitMirrorUrl, defaultIsPublic)
	return
}
func (s service) GetServer(rl context.Context, ownerId int64, repoDisplayName string) (srv server.Server, isNotFoundErr bool, err error) {
	repo, isNotFoundErr, err := s.GetByOwnerIdAndRepoName(rl, ownerId, repoDisplayName)
	if err != nil {
		return
	}
	srv, err = server.NewServer(
		quotaOwner(ownerId), repo.Id, s.db.GetServerRead(rl))
	return
}
func (s service) GetServerByRepoId(rl context.Context, repoId uint64) (server.Server, error) {
	repo, err := s.GetById(rl, repoId)
	if err != nil {
		return nil, err
	}
	return server.NewServer(
		quotaOwner(repo.OwnerId), repo.Id, s.db.GetServerRead(rl))
}
func (s service) GetServerRead(rl context.Context) server.Read {
	return s.db.GetServerRead(rl)
}
func (s service) GetServerWrite(wl context.Context) server.Write {
	return s.db.GetServerWrite(wl)
}

func (s service) GetArchivedRepoIds(rl context.Context, ownerId int64) (iterator.I[uint64], error) {
	return s.db.GetArchivedRepoIds(rl, ownerId)
}

func (s service) NonArchivedRepoCountIsGreaterThan(rl context.Context, ownerId int64, n int) (bool, error) {
	repos, err := s.GetAllByOwnerId(rl, ownerId)
	if err != nil {
		return false, err
	}
	count := 0
	for repos.Next() {
		count += 1
		if count > n {
			break
		}
	}
	err = repos.Err()
	if err != nil {
		return false, err
	}
	return count > n, nil
}

// validates a Git remote repository URL.
// It ensures the payload uses safe web protocols and blocks hidden flag injections.
//
// Note: http carries the credential in cleartext, so prefer https.
func isValidGitMirrorUrl(rawUrl string) bool {
	if len(rawUrl) > 2048 {
		return false
	}
	// Block control characters
	if strings.ContainsAny(rawUrl, "\x00\n\r") {
		return false
	}
	// Prevent the URL from being evaluated as a Git command-line flag
	if strings.HasPrefix(rawUrl, "-") {
		return false
	}
	u, err := url.ParseRequestURI(rawUrl)
	if err != nil {
		return false
	}
	// Restricting to http/https blocks dangerous git
	// transports (ext::, file://, git://, ssh://)
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	// Non-empty host, and a host that does not start with "-" to block
	// SSH-style parameter smuggling attacks embedded inside host blocks
	// (-oProxyCommand=...)
	if u.Host == "" || strings.HasPrefix(u.Host, "-") {
		return false
	}
	// Credential (userinfo, i.e. a "<token>@" part)
	if u.User == nil || u.User.String() == "" {
		return false // a mirror needs a credential to push
	}
	return true
}

func sanitizeGitMirrorUrl(url string) (sanitized string, valid bool) {
	if !isValidGitMirrorUrl(url) {
		return "", false
	}
	prefix := ""
	if strings.HasPrefix(url, "https://") {
		prefix = "https://"
	} else if strings.HasPrefix(url, "http://") {
		prefix = "http://"
	} else {
		return "", false // invalid
	}

	rest := strings.TrimPrefix(url, prefix)

	indexOfAt := strings.Index(rest, "@")
	if indexOfAt == -1 {
		// No token
		return "", false // invalid
	}

	// Rebuild URL but mask token
	sanitized = prefix + "<token>@" + rest[indexOfAt+1:]

	return sanitized, true
}

func (s service) GetRepoTopCommit(rl context.Context, repoId uint64) (commit.Commit, error) {
	server, err := s.GetServerByRepoId(rl, repoId)
	if err != nil {
		return commit.Commit{}, err
	}
	return server.Top(), nil
}
func (s service) GetRepoPendingCommits(rl context.Context, repoId uint64, ascendingOrder bool) (iterator.I[commit.Commit], error) {
	server, err := s.GetServerByRepoId(rl, repoId)
	if err != nil {
		return nil, err
	}
	return server.Pending(ascendingOrder, s.GetServerRead(rl))
}

// Descending order
func (s service) GetRepoPendingCommitsAfter(rl context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	server, err := s.GetServerByRepoId(rl, repoId)
	if err != nil {
		return nil, err
	}
	return server.PendingAfter(afterId, s.GetServerRead(rl))
}
func (s service) GetRepoCommit(rl context.Context, repoId uint64, n commit.LocalId) (commit.Commit, error) {
	server, err := s.GetServerByRepoId(rl, repoId)
	if err != nil {
		return commit.Commit{}, err
	}
	return server.GetLatest(n, s.GetServerRead(rl))
}
func (s service) GetRepoCommitVersion(rl context.Context, repoId uint64, n commit.LocalId, v uint64) (commit.Commit, error) {
	server, err := s.GetServerByRepoId(rl, repoId)
	if err != nil {
		return commit.Commit{}, err
	}
	return server.GetVersion(n, v, s.GetServerRead(rl))
}
func (s service) GetRepoFile(rl context.Context, repoId uint64, a commit.Commit, filename string, w io.Writer) error {
	server, err := s.GetServerByRepoId(rl, repoId)
	if err != nil {
		return err
	}
	return server.WriteFile(a, filename, w, s.GetServerRead(rl))
}