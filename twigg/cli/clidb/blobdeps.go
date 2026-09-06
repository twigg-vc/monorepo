package clidb

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"math"
	"monorepo/data/blobdb"
	"monorepo/data/sqlitehelper"
)

// Implements blobdb.MetadataDb backed by the main sqlite db, so the blob
// metadata participates in the caller's transaction.
// Uses the same table (sqlarge_blobs) and columns as sqlarge/sqlarge.go.
type blobMetadataDb struct {
	s sqlitehelper.SqliteHelper
}

const blobMetadataColumns = `IdPrefix, Id, Version, IsLatest, SavedAt,
	IsDeleted, Datastrip, Offset, DistanceToNonDelta, CompressedSize,
	UncompressedSize, Encoding, HasDeltaEncodingBase, DeltaEncodingBase, QuotaOwner`

func (db blobMetadataDb) GetLatestMetadata(readCtx context.Context,
	idPrefix string, id string) (m blobdb.BlobData, isNotFoundErr bool, err error) {
	return scanBlobMetadata(db.s.QueryRow(readCtx, `
		SELECT `+blobMetadataColumns+` FROM sqlarge_blobs
		WHERE IdPrefix = ? AND Id = ? AND IsLatest = TRUE
	`, idPrefix, id))
}

func (db blobMetadataDb) GetMetadataByVersion(readCtx context.Context,
	idPrefix string, id string, v blobdb.Version) (m blobdb.BlobData, isNotFoundErr bool, err error) {
	return scanBlobMetadata(db.s.QueryRow(readCtx, `
		SELECT `+blobMetadataColumns+` FROM sqlarge_blobs
		WHERE IdPrefix = ? AND Id = ? AND Version = ?
	`, idPrefix, id, v))
}

func (db blobMetadataDb) InsertMetadata(writeCtx context.Context, m blobdb.BlobData) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO sqlarge_blobs (`+blobMetadataColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.IdPrefix, m.Id, m.Version, m.IsLatest, m.SavedAt,
		m.IsDeleted, m.Datastrip, m.Offset, m.DistanceToNonDelta, m.CompressedSize,
		m.Size, m.Encoding, m.HasDeltaEncodingBase, m.DeltaEncodingBase, m.QuotaOwner)
	return err
}

func (db blobMetadataDb) SetMetadata(writeCtx context.Context,
	idPrefix string, id string, v blobdb.Version, isLatest bool) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE sqlarge_blobs
		SET IsLatest = ?
		WHERE IdPrefix = ? AND Id = ? AND Version = ?
	`, isLatest, idPrefix, id, v)
	return err
}

func scanBlobMetadata(row *sql.Row) (m blobdb.BlobData, isNotFoundErr bool, err error) {
	err = row.Scan(&m.IdPrefix, &m.Id, &m.Version, &m.IsLatest, &m.SavedAt,
		&m.IsDeleted, &m.Datastrip, &m.Offset, &m.DistanceToNonDelta, &m.CompressedSize,
		&m.Size, &m.Encoding, &m.HasDeltaEncodingBase, &m.DeltaEncodingBase, &m.QuotaOwner)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
	}
	return
}

// The CLI doesn't enforce quotas, so blobdb gets a no-op blobdb.QuotaDb
type noopQuotaDb struct{}

func (noopQuotaDb) GetQuotaLeft(quotaOwner string) (int64, error) {
	return math.MaxInt64, nil
}
func (noopQuotaDb) IncreaseQuotaLimittedBytes(quotaOwner string, n int64) error {
	return nil
}
func (noopQuotaDb) IncreaseSuccessfullBytes(quotaOwner string, n int64) error {
	return nil
}

// In-memory blobdb.AppendLog used by NewMem
type memAppendLog struct {
	data   []byte
	synced bool
}

func (l *memAppendLog) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(l.data)) {
		return 0, io.EOF
	}
	n := copy(p, l.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
func (l *memAppendLog) Write(p []byte) (int, error) {
	l.data = append(l.data, p...)
	return len(p), nil
}
func (l *memAppendLog) Size() (int64, error) {
	return int64(len(l.data)), nil
}
func (l *memAppendLog) Name() string {
	return "mem"
}
func (l *memAppendLog) Sync() error {
	l.synced = true
	return nil
}
