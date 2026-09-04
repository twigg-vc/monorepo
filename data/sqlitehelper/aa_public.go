package sqlitehelper

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"testing"
	"time"
)

// MUST BE INITIALIZED WITH NewSqliteHelper.
// SqliteHelper is a simple wrapper around a sqlite database with methods in
// convenient signatures. It ensures multi-reader/single-writer with mutexes,
// and supports .sql migrations to be executed.
type SqliteHelper struct {
	s *sqlite
}

// Creates a new Sqlite instance.
// Use InMemoryPathToDir to run an in-memory db - dbFileName is ignored in that case.
func NewSqliteHelper(pathToDir, dbFileName string) (SqliteHelper, error) {
	return newSqlite(pathToDir, dbFileName)
}

// Closes the underlying database.
func (db SqliteHelper) Close() {
	db.s.close()
}

// Runs the db migrations. All the "migrations/*.sql" files from migrationsFs
// will be run.
func (db SqliteHelper) Init(migrationsFs fs.FS) error {
	return db.s.init(migrationsFs)
}

// Create a context to execute methods transactionally.
// closeTx must be called when done.
func (db SqliteHelper) BeginRead() (ctx context.Context, closeTx func(), err error) {
	return db.s.beginReadWithCtx(context.Background())
}

// Create a context to execute methods transactionally.
// closeTx must be called when done.
func (db SqliteHelper) BeginReadWithCtx(baseCtx context.Context) (ctx context.Context, closeTx func(), err error) {
	return db.s.beginReadWithCtx(baseCtx)
}

// Create a context to execute methods transactionally.
// commitTx must be called to commit all changes.
// closeTx must be called when done.
// Returns ErrBeginWriteTimeout if the write transaction in progress doesn't
// finish in time.
func (db SqliteHelper) BeginWrite() (ctx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.s.beginWriteWithCtx(context.Background())
}

// Shortens how long BeginWrite waits for the write transaction in progress.
// Only used for testing.
func (db SqliteHelper) SetMaxWaitForWriteTx(d time.Duration, t *testing.T) {
	db.s.maxWaitForBeginWriteTx = d
}

// Create a context to execute methods transactionally.
// commitTx must be called to commit all changes.
// closeTx must be called when done.
// Returns ErrBeginWriteTimeout if the write transaction in progress doesn't
// finish in time.
func (db SqliteHelper) BeginWriteWithCtx(baseCtx context.Context) (ctx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.s.beginWriteWithCtx(baseCtx)
}

// Runs the query in the underlying db.
// Panics if the ctx was not created with BeginRead/BeginWrite
func (db SqliteHelper) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.s.query(ctx, query, args...)
}

// Runs the query in the underlying db.
// Panics if the ctx was not created with BeginRead/BeginWrite
func (db SqliteHelper) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.s.queryRow(ctx, query, args...)
}

// Runs the query in the underlying db.
// Panics if the ctx was not created with BeginWrite
func (db SqliteHelper) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.s.exec(ctx, query, args...)
}

// Reports whether the write transaction bound to ctx should be committed:
// at least one Exec call succeeded, none failed, and PreventCommit was
// never called. Panics if the ctx was not created with BeginWrite.
func (db SqliteHelper) ShouldCommit(ctx context.Context) bool {
	return db.s.shouldCommit(ctx)
}

// Marks the write transaction bound to ctx as one that must not be
// committed, regardless of what else happens on it. Panics if the ctx was
// not created with BeginWrite.
func (db SqliteHelper) PreventCommit(ctx context.Context) {
	db.s.preventCommit(ctx)
}

const InMemoryPathToDir = ":memory:"

var (
	ErrTamperedMigration = errors.New("migration file was modified")
	ErrBeginWriteTimeout = errors.New("timed out waiting to begin write transaction")
)
