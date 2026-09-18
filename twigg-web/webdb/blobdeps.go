package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"monorepo/data/blobdb"
	"monorepo/data/sqlitehelper"
)

// Implements blobdb.MetadataDb backed by the main sqlite db, so the blob
// metadata participates in the caller's transaction.
// Uses the same table (sqlarge_blobs) and columns as sqlarge/sqlarge.go.
type blobMetadataDb struct {
	s sqlitehelper.SqliteHelper
}

const blobMetadataColumns = `IdPrefix, Id, Version, SavedAt,
	IsDeleted, Datastrip, Offset, DistanceToNonDelta, CompressedSize,
	UncompressedSize, Encoding, HasDeltaEncodingBase, DeltaEncodingBase, QuotaOwner`

func (db blobMetadataDb) GetLatestMetadata(readCtx context.Context,
	idPrefix string, id string) (m blobdb.BlobData, isNotFoundErr bool, err error) {
	return scanBlobMetadata(db.s.QueryRow(readCtx, `
		SELECT `+blobMetadataColumns+` FROM sqlarge_blobs
		WHERE IdPrefix = ? AND Id = ?
		ORDER BY Version DESC LIMIT 1
	`, idPrefix, id))
}

func (db blobMetadataDb) GetMetadataByVersion(readCtx context.Context,
	idPrefix string, id string, v blobdb.Version) (m blobdb.BlobData, isNotFoundErr bool, err error) {
	return scanBlobMetadata(db.s.QueryRow(readCtx, `
		SELECT `+blobMetadataColumns+` FROM sqlarge_blobs
		WHERE IdPrefix = ? AND Id = ? AND Version = ?
	`, idPrefix, id, v))
}

func (db blobMetadataDb) GrabMetadataVersion(writeCtx context.Context,
	idPrefix string, id string) (v blobdb.Version, err error) {
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO sqlarge_blob_versions (IdPrefix, Id, LastGrabbedVersion)
		VALUES (?, ?, 0)
		ON CONFLICT (IdPrefix, Id) DO UPDATE
			SET LastGrabbedVersion = LastGrabbedVersion + 1
		RETURNING LastGrabbedVersion
	`, idPrefix, id).Scan(&v)
	return
}

func (db blobMetadataDb) SetMetadataGrabbedVersion(writeCtx context.Context, m blobdb.BlobData) error {
	var lastGrab blobdb.Version
	err := db.s.QueryRow(writeCtx, `
		SELECT LastGrabbedVersion FROM sqlarge_blob_versions
		WHERE IdPrefix = ? AND Id = ?
	`, m.IdPrefix, m.Id).Scan(&lastGrab)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) || lastGrab < m.Version {
		return fmt.Errorf("version %d not yet grabbed", m.Version)
	}
	// IsLatest is deprecated: nothing reads it anymore, it is only written to
	// satisfy its NOT NULL column
	_, err = db.s.Exec(writeCtx, `
		INSERT INTO sqlarge_blobs (`+blobMetadataColumns+`, IsLatest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE)
	`, m.IdPrefix, m.Id, m.Version, m.SavedAt,
		m.IsDeleted, m.Datastrip, m.Offset, m.DistanceToNonDelta, m.CompressedSize,
		m.Size, m.Encoding, m.HasDeltaEncodingBase, m.DeltaEncodingBase, m.QuotaOwner)
	return err
}

func scanBlobMetadata(row *sql.Row) (m blobdb.BlobData, isNotFoundErr bool, err error) {
	err = row.Scan(&m.IdPrefix, &m.Id, &m.Version, &m.SavedAt,
		&m.IsDeleted, &m.Datastrip, &m.Offset, &m.DistanceToNonDelta, &m.CompressedSize,
		&m.Size, &m.Encoding, &m.HasDeltaEncodingBase, &m.DeltaEncodingBase, &m.QuotaOwner)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
	}
	return
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
