package server

import (
	"errors"
	"fmt"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"monorepo/twigg/tree"
)

type srv struct {
	Top_        commit.Commit
	QuotaOwner  string
	RepoId      uint64
	NextLocalId uint64

	r repo.Repo
}

func newSrv(quotaOwner string, id uint64, l Read) (*srv, error) {
	s := &srv{
		QuotaOwner: quotaOwner,
		RepoId:     id,
		r:          repo.New(quotaOwner, id),
	}
	var err error
	var isNotFoundErr bool
	var topLocalId uint64
	topLocalId, isNotFoundErr, err = l.GetRepoTopCommit(id)
	if isNotFoundErr {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	s.Top_, _, err = l.GetLatestCommitByLocalId(id, topLocalId)
	if err != nil {
		return nil, err
	}
	s.NextLocalId, _, err = l.GetRepoNextLocalId(id)
	if err != nil {
		return nil, err
	}
	return s, err
}

func (s srv) WasInit() bool {
	return s.NextLocalId > 0
}

// Doesn't commit lock
func (s *srv) Init(l Write) error {
	if s.WasInit() {
		return fmt.Errorf("server for %d already initialized", s.RepoId)
	}
	_, hash, err := s.r.Init(l)
	if err != nil {
		return err
	}
	rootCommit := commit.NewRoot( /*isOnServer=*/ true, hash)
	if !rootCommit.IsSubmitted {
		panic("expect root commits to start out as submitted")
	}
	if rootCommit.L != 0 {
		panic("expected root local id to be 0")
	}
	err = l.SetCommit(s.QuotaOwner, s.RepoId, rootCommit)
	if err != nil {
		return err
	}

	s.Top_ = rootCommit
	s.NextLocalId = 1
	err = s.save(l)
	if err != nil {
		s.NextLocalId = 0
	}
	return err
}

// Max distance the next server id can be moved forward in one call
const maxNextServerIdDelta = 10_000

// Doesn't commit lock
func (s *srv) SetNextServerId(id uint64, l Write) error {
	if !s.WasInit() {
		return errors.New("not initialized")
	}
	if id <= s.NextLocalId {
		return fmt.Errorf("id (%d) must be greater than the next id (%d)",
			id, s.NextLocalId)
	}
	if id-s.NextLocalId > maxNextServerIdDelta {
		return fmt.Errorf("id (%d) can be at most %d ahead of the next id (%d)",
			id, maxNextServerIdDelta, s.NextLocalId)
	}
	s.NextLocalId = id
	return s.save(l)
}

func (s *srv) save(l Write) error {
	err := l.SetRepoTopCommit(s.RepoId, s.Top_.L)
	if err != nil {
		return err
	}
	return l.SetRepoNextLocalId(s.RepoId, s.NextLocalId)
}

func (s srv) GetLatest(n uint64, l Read) (c commit.Commit, err error) {
	if !s.WasInit() {
		return c, errors.New("not initialized")
	}
	c, isNotFoundErr, err := l.GetLatestCommitByLocalId(s.RepoId, n)
	if isNotFoundErr {
		err = ErrNotFound
	}
	return
}
func (s srv) GetVersion(n uint64, v uint64, l Read) (c commit.Commit, err error) {
	if !s.WasInit() {
		return c, errors.New("not initialized")
	}
	c, isNotFoundErr, err := l.GetCommitVersionByLocalId(s.RepoId, n, v)
	if isNotFoundErr {
		err = ErrNotFound
	}
	return
}

func (s srv) Diff(A, B commit.Commit, l Read) (tree.ParallelIterator, error) {
	if !s.WasInit() {
		return nil, errors.New("not initialized")
	}
	if A.L == B.L {
		return tree.Walk4(
			s.r.Root(A.TreeVersion, l), s.r.Root(A.ParentTreeVersion, l),
			s.r.Root(B.TreeVersion, l), s.r.Root(B.ParentTreeVersion, l))
	}
	return tree.Walk2(s.r.Root(A.TreeVersion, l), s.r.Root(B.TreeVersion, l))
}
func (s srv) SearchFileInChangedDirs(A, B commit.Commit, l Read, filename string) (repo.FileInChangedDirsIter, error) {
	if !s.WasInit() {
		return repo.FileInChangedDirsIter{}, errors.New("not initialized")
	}
	return s.r.SearchFileInChangedDirs(A.TreeVersion, B.TreeVersion, l, filename)
}
func (s srv) WriteDiff(A, B commit.Commit, filename string, w io.Writer, l Read) error {
	if !s.WasInit() {
		return errors.New("not initialized")
	}
	diffBytes, err := tree.GetPathUnifiedDiff(filename,
		s.r.Root(A.TreeVersion, l), s.r.Root(B.TreeVersion, l))
	if err != nil {
		return err
	}
	_, err = w.Write(diffBytes)
	return err
}

func (s srv) GetTree(a commit.Commit, path string, l Read) (tree.Tree, error) {
	if !s.WasInit() {
		return nil, errors.New("not initialized")
	}
	tr, err := s.r.Root(a.TreeVersion, l).Tree(path)
	if err != nil {
		return nil, err
	}
	return tr, nil
}
func (s srv) WriteFile(a commit.Commit, filename string, w io.Writer, l Read) error {
	tr, err := s.GetTree(a, filename, l)
	if err != nil {
		return err
	}
	if tr.Data().IsDir {
		return fmt.Errorf("%s is a directory", filename)
	}
	wt, err := tr.GetFile()
	if err != nil {
		return err
	}
	_, err = wt.WriteTo(w)
	return err
}

func (s srv) GetCommitWorkdirSize(c commit.Commit, l Read) (int64, error) {
	if !s.WasInit() {
		return 0, errors.New("not initialized")
	}
	tr, err := s.GetTree(c, tree.RootPath, l)
	if err != nil {
		return 0, err
	}
	return tr.Data().Size, nil
}

func (s srv) Top() commit.Commit {
	if !s.WasInit() {
		panic("server was not initialized")
	}
	return s.Top_
}

func (s srv) Pending(ascendingOrder bool, l Read) (iterator.I[commit.Commit], error) {
	if !s.WasInit() {
		panic("server was not initialized")
	}
	return l.GetPendingCommits(ascendingOrder, s.RepoId)
}

// Descending order
func (s srv) PendingAfter(afterId commit.LocalId, l Read) (iterator.I[commit.Commit], error) {
	if !s.WasInit() {
		panic("server was not initialized")
	}
	return l.GetPendingCommitsAfter(s.RepoId, afterId)
}