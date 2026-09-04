package db

import (
	"context"
	"database/sql"
	"errors"
	"monorepo/twigg-track/trackclient"
	"testing"
	"time"
)

// MUST BE INITIALIZED WITH NewSqlite
type Sqlite struct {
	s *sqlite
}

// Creates a new Sqlite instance. Use InMemoryPathToDir to run an in-memory db.
func NewSqlite(pathToDir string) (Sqlite, error) {
	return newSqlite(pathToDir)
}

// Creates a new Sqlite instance using an sql.Db instance.
// The instance must be using SQLite drivers.
func NewSqliteFromSql(db *sql.DB) (Sqlite, error) {
	return newSqliteFromSql(db)
}

// Closes the underlying database.
func (db Sqlite) Close() {
	db.s.close()
}

// Does the necessary initial setup. Must be called once before using the db.
func (db Sqlite) Init() error {
	return db.s.init()
}

// Create a context to execute methods transactionally.
// closeTx must be called when done.
func (db Sqlite) BeginRead() (ctx context.Context, closeTx func(), err error) {
	return db.s.beginReadWithCtx(context.Background())
}

// Create a context to execute methods transactionally.
// closeTx must be called when done.
func (db Sqlite) BeginReadWithCtx(baseCtx context.Context) (ctx context.Context, closeTx func(), err error) {
	return db.s.beginReadWithCtx(baseCtx)
}

// Create a context to execute methods transactionally.
// commitTx must be called to commit all changes.
// closeTx must be called when done.
func (db Sqlite) BeginWrite() (ctx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.s.beginWriteWithCtx(context.Background())
}

// Create a context to execute methods transactionally.
// commitTx must be called to commit all changes.
// closeTx must be called when done.
func (db Sqlite) BeginWriteWithCtx(baseCtx context.Context) (ctx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.s.beginWriteWithCtx(baseCtx)
}

const InMemoryPathToDir = ":memory:"

const MaxPayloadLen = 512 * 1024 // 512KB

// Creates a job with "queued" status
func (db Sqlite) Create(tx context.Context, id string, payload []byte, skipWebhooks bool) (trackclient.TrackJob, error) {
	return db.s.Create(tx, id, payload, skipWebhooks)
}
func (db Sqlite) GetMaxPayloadLen() int {
	return db.s.maxPayloadLen
}
func (db Sqlite) SetMaxPayloadLen(n int) {
	db.s.maxPayloadLen = n
}

// Mark the job as done
func (db Sqlite) SetStatus(tx context.Context, id string, st trackclient.TrackJobStatus, executionTimeMillis int64) error {
	return db.s.SetStatus(tx, id, st, executionTimeMillis)
}
func (db Sqlite) Exists(tx context.Context, id string) (bool, error) {
	_, isNotFoundErr, err := db.s.Get(tx, id)
	if isNotFoundErr {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
func (db Sqlite) Get(tx context.Context, id string) (j trackclient.TrackJob, isNotFoundErr bool, err error) {
	return db.s.Get(tx, id)
}
func (db Sqlite) RequestBestEffortCancelation(tx context.Context, id string) error {
	return db.s.RequestBestEffortCancelation(tx, id)
}
func (db Sqlite) BestEffortCancelationWasRequested(tx context.Context, id string) (bool, error) {
	return db.s.BestEffortCancelationWasRequested(tx, id)
}
func (db Sqlite) CleanupBestEffortCancelation(tx context.Context, ttl time.Duration, t *testing.T) error {
	return db.s.cleanupBestEffortCancelation(tx, ttl)
}

var (
	ErrNotFound      = errors.New("not found")
	ErrPayloadTooBig = errors.New("payload is too big")
)