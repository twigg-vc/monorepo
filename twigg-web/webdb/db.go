package webdb

import (
	"context"
	"database/sql"
	"embed"
	"errors"
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

const datastripName = "ds0"
const quotaDbFileName = "quota.db"

// The blob bytes storage.
type blobAppendLog interface {
	blobdb.AppendLog
	Sync() error
}

type webDb struct {
	s     sqlitehelper.SqliteHelper
	log   blobAppendLog
	blobs blobdb.BlobDb
	quota quotaDb
}

func newWebDb(pathToDir, dbFileName string, blockSize int64, bs BlobStorage,
	blobStorageCacheCapacity int, enforceQuota bool) (WebDb, func(), error) {
	noClose := func() {}
	s, err := sqlitehelper.NewSqliteHelper(pathToDir, dbFileName)
	if err != nil {
		return WebDb{}, noClose, err
	}
	err = s.Init(embeddedMigrations)
	if err != nil {
		s.Close()
		return WebDb{}, noClose, err
	}
	provider, closeProvider, err := tiered.NewProvider(
		pathToDir, datastripName, blockSize, bs, blobStorageCacheCapacity)
	if err != nil {
		if closeProvider != nil {
			_ = closeProvider()
		}
		s.Close()
		return WebDb{}, noClose, err
	}
	quota, err := newQuotaDb(pathToDir, quotaDbFileName)
	if err != nil {
		_ = closeProvider()
		s.Close()
		return WebDb{}, noClose, err
	}
	log := appendlog.New(provider)
	blobs := blobdb.New(log, quota, blobMetadataDb{s}, enforceQuota)
	closeDb := func() {
		_ = closeProvider()
		s.Close()
		quota.close()
	}
	return WebDb{&webDb{s: s, log: log, blobs: blobs, quota: quota}}, closeDb, nil
}

func newMemWebDb(enforceQuota bool) (WebDb, func(), error) {
	s, err := sqlitehelper.NewSqliteHelper(sqlitehelper.InMemoryPathToDir, "")
	if err != nil {
		return WebDb{}, func() {}, err
	}
	err = s.Init(embeddedMigrations)
	if err != nil {
		s.Close()
		return WebDb{}, func() {}, err
	}
	quota, err := newMemQuotaDb()
	if err != nil {
		s.Close()
		return WebDb{}, func() {}, err
	}
	log := &memAppendLog{}
	blobs := blobdb.New(log, quota, blobMetadataDb{s}, enforceQuota)
	closeDb := func() {
		s.Close()
		quota.close()
	}
	return WebDb{&webDb{s: s, log: log, blobs: blobs, quota: quota}}, closeDb, nil
}

func (db webDb) BeginWrite() (ctx context.Context, closeTx func(), commitTx func() error, err error) {
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

func (db webDb) BeginRead() (ctx context.Context, closeTx func(), err error) {
	return db.s.BeginRead()
}

func (db webDb) GetRepoNextLocalId(ctx context.Context, repoId uint64) (n uint64, isNotFoundErr bool, err error) {
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
func (db webDb) SetRepoNextLocalId(ctx context.Context, repoId uint64, n uint64) error {
	_, err := db.s.Exec(ctx, `
		INSERT INTO twigg_repo_next (repoId, nextCommitLocalId)
		VALUES (?, ?)
		ON CONFLICT (repoId) DO UPDATE SET
			nextCommitLocalId = EXCLUDED.nextCommitLocalId;
	`, repoId, n)
	return err
}

func (db webDb) GetRepoTopCommit(ctx context.Context, repoId uint64) (localId uint64, isNotFoundErr bool, err error) {
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
func (db webDb) SetRepoTopCommit(ctx context.Context, repoId uint64, localId uint64) error {
	_, err := db.s.Exec(ctx, `
		INSERT INTO twigg_repo_top (repoId, topCommitLocalId)
		VALUES (?, ?)
		ON CONFLICT (repoId) DO UPDATE SET
			topCommitLocalId = EXCLUDED.topCommitLocalId;
	`, repoId, localId)
	return err
}

func (db webDb) PreventCommit(ctx context.Context) {
	db.s.PreventCommit(ctx)
}
func (db webDb) ShouldCommit(ctx context.Context) bool {
	return db.s.ShouldCommit(ctx)
}

// setBlob wraps db.blobs.SetBlob so a failed blob write also prevents the
// enclosing write transaction from being committed. This is needed because
// blobdb.SetBlob can fail before ever touching SQL (e.g. while writing the
// blob bytes themselves), a path db.s.Exec's own tracking can't see.
func (db webDb) setBlob(ctx context.Context, quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, err error) {
	v, err = db.blobs.SetBlob(ctx, quotaOwner, idPrefix, id, wt)
	if err != nil {
		db.s.PreventCommit(ctx)
	}
	return
}

func (db webDb) SetBlob(writeCtx context.Context, quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, err error) {
	return db.setBlob(writeCtx, quotaOwner, idPrefix, id, wt)
}
func (db webDb) GetBlob(readCtx context.Context, idPrefix, id string) (
	m blobdb.BlobData, r io.Reader, closeReader func(), err error) {
	return db.blobs.GetBlob(readCtx, idPrefix, id)
}
func (db webDb) GetBlobVersion(readCtx context.Context, idPrefix, id string, v blobdb.Version) (
	m blobdb.BlobData, r io.Reader, closeReader func(), err error) {
	return db.blobs.GetBlobVersion(readCtx, idPrefix, id, v)
}
