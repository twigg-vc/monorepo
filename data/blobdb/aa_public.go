// Package blobdb implements versioned blob storage with delta compression
// and quota tracking.
package blobdb

import (
	"context"
	"errors"
	"io"
	"monorepo/data/deltastream"
	"time"
)

// Append-only log holding the blob bytes. Writes always append at the end.
// Assumes a single writer: callers must serialize writes.
type AppendLog interface {
	// Reads at an offset
	io.ReaderAt
	// Writes at the end
	io.Writer
	// Returns the total size
	Size() (int64, error)
	// Identifies this instance
	Name() string
}

// Stores the bytes written/refused per quota owner.
// No ctx on purpose: quota tracks physical AppendLog bytes, which survive
// rollbacks, so implementations must persist immediately and never join
// the caller's transaction.
type QuotaDb interface {
	GetQuotaLeft(quotaOwner string) (int64, error)
	IncreaseQuotaLimittedBytes(quotaOwner string, n int64) error
	IncreaseSuccessfullBytes(quotaOwner string, n int64) error
}

// Stores the metadata of every blob version
type MetadataDb interface {
	GetLatestMetadata(readCtx context.Context, idPrefix string, id string) (m BlobData, isNotFoundErr bool, err error)
	GetMetadataByVersion(readCtx context.Context, idPrefix string, id string, v Version) (m BlobData, isNotFoundErr bool, err error)
	InsertMetadata(writeCtx context.Context, m BlobData) error
	SetMetadataIsLatest(writeCtx context.Context, idPrefix string, id string, v Version, isLatest bool) error
}

// MUST BE INITIALIZED WITH `New`
type BlobDb struct {
	db db
}

// enforceQuota refuses writes once the quota owner runs out of quota.
func New(log AppendLog, q QuotaDb, m MetadataDb, enforceQuota bool) BlobDb {
	return BlobDb{db: db{log: log, q: q, m: m, enforceQuota: enforceQuota}}
}

// Write a blob by its Id. The first version is 0, each write creates
// version latest+1. Can return ErrNotEnoughQuota.
func (db BlobDb) SetBlob(writeCtx context.Context,
	quotaOwner string, idPrefix, id string, wt io.WriterTo) (v Version, err error) {
	return db.db.SetBlob(writeCtx, quotaOwner, idPrefix, id, wt)
}

// Get the latest version of a blob by its id. Returns ErrNotFound if not
// found. closeReader must always be called.
func (db BlobDb) GetBlob(readCtx context.Context, idPrefix, id string) (
	m BlobData, r io.Reader, closeReader func(), err error) {
	return db.db.GetBlob(readCtx, idPrefix, id)
}

// Get a version of a blob by its id. Returns ErrNotFound if not found.
// closeReader must always be called.
func (db BlobDb) GetBlobVersion(readCtx context.Context, idPrefix, id string, v Version) (
	m BlobData, r io.Reader, closeReader func(), err error) {
	return db.db.GetBlobVersion(readCtx, idPrefix, id, v)
}

type Version = uint64

type BlobData struct {
	IdPrefix       string
	Id             string
	Version        Version
	Size           int64
	CompressedSize int64
	SavedAt        time.Time
	IsDeleted      bool
	QuotaOwner     string
	IsLatest       bool
	Datastrip      string
	Offset         int64
	// DistanceToNonDelta indicates the size of the consecutive delta chain
	DistanceToNonDelta int64
	// Encoding indicates the method of compression used to store the blob.
	// For delta compression methods:
	// if HasDeltaEncodingBase, DeltaEncodingBase indicates which version was
	// used as the delta base. Else, Version-1 is the delta base.
	Encoding             deltastream.CompressionMethod
	HasDeltaEncodingBase bool
	DeltaEncodingBase    Version
}

var (
	ErrNotFound       = errors.New("blobdb: not found")
	ErrNotEnoughQuota = errors.New("blobdb: not enough quota")
)
