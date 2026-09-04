package clidb

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"monorepo/data/appendlog"
	"monorepo/data/appendlog/tiered"
	"monorepo/data/blobdb"
	"monorepo/data/sqlitehelper"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

var (
	ErrNotFound = errors.New("not found")
)

const (
	appendlogName      = "ds0"
	appendlogBlockSize = 4 * 1024 * 1024 * 1024 // 4GB
)

// The blob bytes storage.
type blobAppendLog interface {
	blobdb.AppendLog
	Sync() error
}

type cliDb struct {
	s     sqlitehelper.SqliteHelper
	log   blobAppendLog
	blobs blobdb.BlobDb
}

func newCliDb(pathToDir, dbFileName string) (CliDb, func(), error) {
	noClose := func() {}
	s, err := sqlitehelper.NewSqliteHelper(pathToDir, dbFileName)
	if err != nil {
		return CliDb{}, noClose, err
	}
	err = s.Init(embeddedMigrations)
	if err != nil {
		s.Close()
		return CliDb{}, noClose, err
	}
	provider, closeProvider, err := tiered.NewProvider(
		pathToDir, appendlogName, appendlogBlockSize,
		/*BlobStorage=*/ nil,
		/*blobStorageCacheCapacity=*/ 0)
	if err != nil {
		if closeProvider != nil {
			_ = closeProvider()
		}
		s.Close()
		return CliDb{}, noClose, err
	}
	log := appendlog.New(provider)
	blobs := blobdb.New(log, noopQuotaDb{}, blobMetadataDb{s} /*enforceQuota=*/, false)
	closeDb := func() {
		_ = closeProvider()
		s.Close()
	}
	return CliDb{&cliDb{s: s, log: log, blobs: blobs}}, closeDb, nil
}

func newMemCliDb() (CliDb, func(), error) {
	s, err := sqlitehelper.NewSqliteHelper(sqlitehelper.InMemoryPathToDir, "")
	if err != nil {
		return CliDb{}, func() {}, err
	}
	err = s.Init(embeddedMigrations)
	if err != nil {
		s.Close()
		return CliDb{}, func() {}, err
	}
	log := &memAppendLog{}
	blobs := blobdb.New(log, noopQuotaDb{}, blobMetadataDb{s} /*enforceQuota=*/, false)
	return CliDb{&cliDb{s: s, log: log, blobs: blobs}}, s.Close, nil
}

func (db cliDb) BeginWrite() (ctx context.Context, closeTx func(), commitTx func() error, err error) {
	ctx, closeTx, commitTx, err = db.s.BeginWrite()
	if err != nil {
		return
	}
	innerCommitTx := commitTx
	commitTx = func() error {
		err := db.log.Sync()
		if err != nil {
			return err
		}
		return innerCommitTx()
	}
	return
}

func (db cliDb) BeginRead() (ctx context.Context, closeTx func(), err error) {
	return db.s.BeginRead()
}

func (db cliDb) GetRepoNextLocalId(ctx context.Context, repoId uint64) (n uint64, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT nextCommitLocalId FROM twigg_repo_next
		WHERE repoId = ?
	`, repoId).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
	}
	return
}
func (db cliDb) SetRepoNextLocalId(ctx context.Context, repoId uint64, n uint64) error {
	_, err := db.s.Exec(ctx, `
		INSERT INTO twigg_repo_next (repoId, nextCommitLocalId)
		VALUES (?, ?)
		ON CONFLICT (repoId) DO UPDATE SET
			nextCommitLocalId = EXCLUDED.nextCommitLocalId;
	`, repoId, n)
	return err
}

func (db cliDb) GetRepoTopCommit(ctx context.Context, repoId uint64) (localId uint64, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT topCommitLocalId FROM twigg_repo_top
		WHERE repoId = ?
	`, repoId).Scan(&localId)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
	}
	return
}
func (db cliDb) SetRepoTopCommit(ctx context.Context, repoId uint64, localId uint64) error {
	_, err := db.s.Exec(ctx, `
		INSERT INTO twigg_repo_top (repoId, topCommitLocalId)
		VALUES (?, ?)
		ON CONFLICT (repoId) DO UPDATE SET
			topCommitLocalId = EXCLUDED.topCommitLocalId;
	`, repoId, localId)
	return err
}

func (db cliDb) GetWorkdirCache(ctx context.Context, path string) (size int64, modTimeUnixMilli int64, hash [32]byte, isText bool, isNotFoundErr bool, err error) {
	var hashBlob []byte
	err = db.s.QueryRow(ctx, `
		SELECT size, modTimeUnixMilli, hash, isText
		FROM twigg_workdir_tree_cache2
		WHERE path = ?
	`, path).Scan(&size, &modTimeUnixMilli, &hashBlob, &isText)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		return
	}
	if len(hashBlob) != len(hash) {
		err = fmt.Errorf("invalid hash length: got %d, want %d", len(hashBlob), len(hash))
		return
	}
	copy(hash[:], hashBlob)
	return
}
func (db cliDb) SetWorkdirCache(ctx context.Context, path string, size int64, modTimeUnixMilli int64, hash [32]byte, isText bool) error {
	_, err := db.s.Exec(ctx, `
		INSERT INTO twigg_workdir_tree_cache2 (path, size, modTimeUnixMilli, hash, isText)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (path) DO UPDATE SET
			size = EXCLUDED.size,
			modTimeUnixMilli = EXCLUDED.modTimeUnixMilli,
			hash = EXCLUDED.hash,
			isText = EXCLUDED.isText;
	`, path, size, modTimeUnixMilli, hash[:], isText)
	return err
}

func (db cliDb) PreventCommit(ctx context.Context) {
	db.s.PreventCommit(ctx)
}
func (db cliDb) ShouldCommit(ctx context.Context) bool {
	return db.s.ShouldCommit(ctx)
}

// setBlob wraps db.blobs.SetBlob so a failed blob write also prevents the
// enclosing write transaction from being committed. This is needed because
// blobdb.SetBlob can fail before ever touching SQL (e.g. while writing the
// blob bytes themselves), a path db.s.Exec's own tracking can't see.
func (db cliDb) setBlob(ctx context.Context, quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, err error) {
	v, err = db.blobs.SetBlob(ctx, quotaOwner, idPrefix, id, wt)
	if err != nil {
		db.s.PreventCommit(ctx)
	}
	return
}

func (db cliDb) SetBlob(writeCtx context.Context, quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, err error) {
	return db.setBlob(writeCtx, quotaOwner, idPrefix, id, wt)
}
func (db cliDb) GetBlob(readCtx context.Context, idPrefix, id string) (
	m blobdb.BlobData, r io.Reader, closeReader func(), err error) {
	return db.blobs.GetBlob(readCtx, idPrefix, id)
}
func (db cliDb) GetBlobVersion(readCtx context.Context, idPrefix, id string, v blobdb.Version) (
	m blobdb.BlobData, r io.Reader, closeReader func(), err error) {
	return db.blobs.GetBlobVersion(readCtx, idPrefix, id, v)
}
