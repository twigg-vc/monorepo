package clidb

import (
	"context"
	"io"
	"monorepo/base/iterator"
	"monorepo/data/blobdb"
	"monorepo/twigg/clistate"
	"monorepo/twigg/commit"
	"monorepo/twigg/treev"
)

// Implements the database used by the Twigg CLI.
// Test clients can also use it because it provides all the methods a twigg
// client needs.
type CliDb struct {
	db *cliDb
}

// Creates a new instance that stores everything under pathToDir.
// dbFileName specifies the name of the underlying sqlite db file.
// closeDb must be called when done.
func New(pathToDir, dbFileName string) (db CliDb, closeDb func(), err error) {
	return newCliDb(pathToDir, dbFileName)
}

// Creates a new instance that stores the data in memory
func NewMem() (db CliDb, closeDb func(), err error) {
	return newMemCliDb()
}

// Returns a write context that must be used with all other methods that write to the db
func (db CliDb) BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.db.BeginWrite()
}

// Returns a read context that must be used with all other methods that read from the db
func (db CliDb) BeginRead() (readCtx context.Context, closeTx func(), err error) {
	return db.db.BeginRead()
}

// Marks the write transaction bound to writeCtx as one that must not be
// committed, regardless of what else happens on it.
func (db CliDb) PreventCommit(writeCtx context.Context) {
	db.db.PreventCommit(writeCtx)
}

// Reports whether the write transaction bound to writeCtx should be
// committed: at least one write succeeded, none failed, and PreventCommit
// was never called.
func (db CliDb) ShouldCommit(writeCtx context.Context) bool {
	return db.db.ShouldCommit(writeCtx)
}

func (db CliDb) GetRepoNextLocalId(ctx context.Context, repoId uint64) (n uint64, isNotFoundErr bool, e error) {
	return db.db.GetRepoNextLocalId(ctx, repoId)
}
func (db CliDb) SetRepoNextLocalId(ctx context.Context, repoId uint64, n uint64) error {
	return db.db.SetRepoNextLocalId(ctx, repoId, n)
}

func (db CliDb) GetRepoTopCommit(ctx context.Context, repoId uint64) (commitLocalId uint64, isNotFoundErr bool, e error) {
	return db.db.GetRepoTopCommit(ctx, repoId)
}
func (db CliDb) SetRepoTopCommit(ctx context.Context, repoId uint64, commitLocalId uint64) error {
	return db.db.SetRepoTopCommit(ctx, repoId, commitLocalId)
}

func (db CliDb) GetWorkdirCache(ctx context.Context, path string) (size int64, modTimeUnixMilli int64, hash [32]byte, isText bool, isNotFoundErr bool, e error) {
	return db.db.GetWorkdirCache(ctx, path)
}
func (db CliDb) SetWorkdirCache(ctx context.Context, path string, size int64, modTimeUnixMilli int64, hash [32]byte, isText bool) error {
	return db.db.SetWorkdirCache(ctx, path, size, modTimeUnixMilli, hash, isText)
}

func (db CliDb) GetCliState(ctx context.Context) (st clistate.State, isNotFoundErr bool, e error) {
	return db.db.GetCliState(ctx)
}
func (db CliDb) SetCliState(ctx context.Context, st clistate.State) error {
	return db.db.SetCliState(ctx, st)
}

func (db CliDb) GetTreeData(ctx context.Context, repoId uint64, treePath string, v uint64) (td treev.TreeDataV, isNotFoundErr bool, e error) {
	return db.db.GetTreeData(ctx, repoId, treePath, v)
}
func (db CliDb) GetTreeBlob(ctx context.Context, repoId uint64, treePath string, v uint64) (r io.Reader, closeR func(), isNotFoundErr bool, e error) {
	return db.db.GetTreeBlob(ctx, repoId, treePath, v)
}
func (db CliDb) GetLastVersionOfRootTree(ctx context.Context, repoId uint64) (v uint64, isNotFoundErr bool, e error) {
	return db.db.GetLastVersionOfRootTree(ctx, repoId)
}
func (db CliDb) SetTreeData(ctx context.Context, quotaOwner string, repoId uint64, treePath string, td treev.TreeDataV) (uint64, error) {
	return db.db.SetTreeData(ctx, quotaOwner, repoId, treePath, td)
}
func (db CliDb) SetTreeBlob(ctx context.Context, quotaOwner string, repoId uint64, treePath string, wt io.WriterTo) (uint64, error) {
	return db.db.SetTreeBlob(ctx, quotaOwner, repoId, treePath, wt)
}

func (db CliDb) SetCommit(ctx context.Context, quotaOwner string, repoId uint64, c commit.Commit) error {
	return db.db.SetCommit(ctx, quotaOwner, repoId, c)
}
func (db CliDb) GetLatestCommitByLocalId(ctx context.Context, repoId uint64, L uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetLatestCommitByLocalId(ctx, repoId, L)
}
func (db CliDb) GetCommitVersionByLocalId(ctx context.Context, repoId uint64, L uint64, v uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetCommitVersionByLocalId(ctx, repoId, L, v)
}
func (db CliDb) GetLatestCommitByServerId(ctx context.Context, repoId uint64, ServerL uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetLatestCommitByServerId(ctx, repoId, ServerL)
}
func (db CliDb) GetCommitVersionByServerId(ctx context.Context, repoId uint64, ServerL uint64, ServerV uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetCommitVersionByServerId(ctx, repoId, ServerL, ServerV)
}
func (db CliDb) GetCommitChildren(ctx context.Context, repoId uint64, commitL uint64, commitV uint64, maxRowsToRead int) (children []commit.LocalId, childrenVersions []uint64, e error) {
	return db.db.GetCommitChildren(ctx, repoId, commitL, commitV, maxRowsToRead)
}
func (db CliDb) GetPendingCommits(ctx context.Context, ascendingOrder bool, repoId uint64) (iterator.I[commit.Commit], error) {
	return db.db.GetPendingCommits(ctx, ascendingOrder, repoId)
}
func (db CliDb) GetPendingCommitsAfter(ctx context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	return db.db.GetPendingCommitsAfter(ctx, repoId, afterId)
}

// Write a blob by its Id. The first version is 0, each write creates
// version latest+1.
func (db CliDb) SetBlob(writeCtx context.Context, quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, e error) {
	return db.db.SetBlob(writeCtx, quotaOwner, idPrefix, id, wt)
}

// Get the latest version of a blob by its id. Returns ErrNotFound if not
// found. closeReader must always be called.
func (db CliDb) GetBlob(readCtx context.Context, idPrefix, id string) (
	m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return db.db.GetBlob(readCtx, idPrefix, id)
}

// Get a version of a blob by its id. Returns ErrNotFound if not found.
// closeReader must always be called.
func (db CliDb) GetBlobVersion(readCtx context.Context, idPrefix, id string, v blobdb.Version) (
	m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return db.db.GetBlobVersion(readCtx, idPrefix, id, v)
}

// Returns a new context that contains a read/write context and the db itself.
// This allows for methods to be executed directly on it; without haivng to pass
// the db around. It's needed for the twigg methods.
func (db CliDb) Bind(ctx context.Context) Ctx {
	return Ctx{db: db, ctx: ctx}
}

// Binds a db instance and a read/write context to expose methods that don't
// require a context
type Ctx struct {
	db  CliDb
	ctx context.Context
}

func (c Ctx) GetRepoNextLocalId(repoId uint64) (n uint64, isNotFoundErr bool, e error) {
	return c.db.GetRepoNextLocalId(c.ctx, repoId)
}
func (c Ctx) SetRepoNextLocalId(repoId uint64, n uint64) error {
	return c.db.SetRepoNextLocalId(c.ctx, repoId, n)
}

func (c Ctx) GetRepoTopCommit(repoId uint64) (commitLocalId uint64, isNotFoundErr bool, e error) {
	return c.db.GetRepoTopCommit(c.ctx, repoId)
}
func (c Ctx) SetRepoTopCommit(repoId uint64, commitLocalId uint64) error {
	return c.db.SetRepoTopCommit(c.ctx, repoId, commitLocalId)
}

func (c Ctx) GetWorkdirCache(path string) (size int64, modTimeUnixMilli int64, hash [32]byte, isText bool, isNotFoundErr bool, e error) {
	return c.db.GetWorkdirCache(c.ctx, path)
}
func (c Ctx) SetWorkdirCache(path string, size int64, modTimeUnixMilli int64, hash [32]byte, isText bool) error {
	return c.db.SetWorkdirCache(c.ctx, path, size, modTimeUnixMilli, hash, isText)
}

func (c Ctx) PreventCommit() {
	c.db.PreventCommit(c.ctx)
}
func (c Ctx) ShouldCommit() bool {
	return c.db.ShouldCommit(c.ctx)
}

func (c Ctx) GetCliState() (st clistate.State, isNotFoundErr bool, e error) {
	return c.db.GetCliState(c.ctx)
}
func (c Ctx) SetCliState(st clistate.State) error {
	return c.db.SetCliState(c.ctx, st)
}

func (c Ctx) GetTreeData(repoId uint64, treePath string, v uint64) (td treev.TreeDataV, isNotFoundErr bool, e error) {
	return c.db.GetTreeData(c.ctx, repoId, treePath, v)
}
func (c Ctx) GetTreeBlob(repoId uint64, treePath string, v uint64) (r io.Reader, closeR func(), isNotFoundErr bool, e error) {
	return c.db.GetTreeBlob(c.ctx, repoId, treePath, v)
}
func (c Ctx) GetLastVersionOfRootTree(repoId uint64) (v uint64, isNotFoundErr bool, e error) {
	return c.db.GetLastVersionOfRootTree(c.ctx, repoId)
}
func (c Ctx) SetTreeData(quotaOwner string, repoId uint64, treePath string, td treev.TreeDataV) (uint64, error) {
	return c.db.SetTreeData(c.ctx, quotaOwner, repoId, treePath, td)
}
func (c Ctx) SetTreeBlob(quotaOwner string, repoId uint64, treePath string, wt io.WriterTo) (uint64, error) {
	return c.db.SetTreeBlob(c.ctx, quotaOwner, repoId, treePath, wt)
}

func (c Ctx) SetCommit(quotaOwner string, repoId uint64, cm commit.Commit) error {
	return c.db.SetCommit(c.ctx, quotaOwner, repoId, cm)
}
func (c Ctx) GetLatestCommitByLocalId(repoId uint64, L uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetLatestCommitByLocalId(c.ctx, repoId, L)
}
func (c Ctx) GetCommitVersionByLocalId(repoId uint64, L uint64, v uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetCommitVersionByLocalId(c.ctx, repoId, L, v)
}
func (c Ctx) GetLatestCommitByServerId(repoId uint64, ServerL uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetLatestCommitByServerId(c.ctx, repoId, ServerL)
}
func (c Ctx) GetCommitVersionByServerId(repoId uint64, ServerL uint64, ServerV uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetCommitVersionByServerId(c.ctx, repoId, ServerL, ServerV)
}
func (c Ctx) GetCommitChildren(repoId uint64, commitL uint64, commitV uint64, maxRowsToRead int) (children []commit.LocalId, childrenVersions []uint64, e error) {
	return c.db.GetCommitChildren(c.ctx, repoId, commitL, commitV, maxRowsToRead)
}
func (c Ctx) GetPendingCommits(ascendingOrder bool, repoId uint64) (iterator.I[commit.Commit], error) {
	return c.db.GetPendingCommits(c.ctx, ascendingOrder, repoId)
}
func (c Ctx) GetPendingCommitsAfter(repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	return c.db.GetPendingCommitsAfter(c.ctx, repoId, afterId)
}

func (c Ctx) SetBlob(quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, e error) {
	return c.db.SetBlob(c.ctx, quotaOwner, idPrefix, id, wt)
}
func (c Ctx) GetBlob(idPrefix, id string) (m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return c.db.GetBlob(c.ctx, idPrefix, id)
}
func (c Ctx) GetBlobVersion(idPrefix, id string, v blobdb.Version) (m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return c.db.GetBlobVersion(c.ctx, idPrefix, id, v)
}