package sqlitehelper

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // register the driver
)

// defaultMaxWaitForBeginWriteTx defines how long beginWriteWithCtx waits for
// the write semaphore before giving up.
const defaultMaxWaitForBeginWriteTx = 30 * time.Second

type sqlite struct {
	db                     *sql.DB
	writeSem               chan struct{}
	maxWaitForBeginWriteTx time.Duration
}

func newSqliteStruct(db *sql.DB) *sqlite {
	return &sqlite{
		db:                     db,
		writeSem:               make(chan struct{}, 1),
		maxWaitForBeginWriteTx: defaultMaxWaitForBeginWriteTx,
	}
}

func newSqlite(pathToDir string, dbFileName string) (SqliteHelper, error) {
	if pathToDir == InMemoryPathToDir {
		return newInMemorySqlite()
	}
	if !filepath.IsAbs(pathToDir) {
		currentDir, err := os.Getwd()
		if err != nil {
			return SqliteHelper{}, err
		}
		pathToDir = filepath.Join(currentDir, pathToDir)
	}
	err := os.MkdirAll(pathToDir, os.ModePerm)
	if err != nil {
		return SqliteHelper{}, fmt.Errorf("failed to mkdir %s: %s", pathToDir, err)
	}
	absPathToDbFile := filepath.Join(pathToDir, dbFileName)
	db, err := sql.Open(
		"sqlite",
		fmt.Sprintf("file:%s?_pragma=journal_mode=WAL&_pragma=synchronous=FULL", absPathToDbFile))
	if err != nil {
		return SqliteHelper{}, fmt.Errorf("failed to open db at %s: %s", absPathToDbFile, err)
	}
	return SqliteHelper{newSqliteStruct(db)}, nil
}

func newInMemorySqlite() (SqliteHelper, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return SqliteHelper{}, err
	}
	db.SetMaxOpenConns(1)
	return SqliteHelper{newSqliteStruct(db)}, nil
}

func (s *sqlite) close() {
	if s.db == nil {
		return
	}
	_ = s.db.Close()
}

type readTxKey struct{}
type writeTxKey struct{}
type commitStateKey struct{}

// Tracks whether the write transaction bound to a ctx (as returned by
// BeginWrite) should actually be committed: at least one write must have
// succeeded, none may have failed, and PreventCommit must not have been
// called.
type commitState struct {
	hadWrite      bool
	hadError      bool
	preventCommit bool
}

func (s sqlite) beginReadWithCtx(inputCtx context.Context) (ctx context.Context, closeTx func(), err error) {
	closeTx = func() {}
	ctx = inputCtx

	_, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if ok {
		err = errors.New("context already has a read tx")
		return
	}

	tx, err := s.db.BeginTx(
		context.Background(),
		&sql.TxOptions{ReadOnly: true})
	if err != nil {
		return
	}
	ctx = context.WithValue(inputCtx, readTxKey{}, tx)
	closeTx = sync.OnceFunc(func() {
		_ = tx.Rollback()
	})
	return
}

func (s sqlite) beginWriteWithCtx(inputCtx context.Context) (ctx context.Context, closeTx func(), commit func() error, err error) {
	closeTx = func() {}
	commit = func() error { return nil }
	ctx = inputCtx

	_, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if ok {
		err = errors.New("context already has a read tx")
		return
	}
	_, ok = ctx.Value(writeTxKey{}).(*sql.Tx)
	if ok {
		err = errors.New("context already has a write tx")
		return
	}

	releaseWriteSem, err := s.acquireWriteSemaphore()
	if err != nil {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		releaseWriteSem()
		return
	}
	ctx = context.WithValue(inputCtx, readTxKey{}, tx)
	ctx = context.WithValue(ctx, writeTxKey{}, tx)
	ctx = context.WithValue(ctx, commitStateKey{}, &commitState{})
	closeTx = sync.OnceFunc(func() {
		_ = tx.Rollback()
		releaseWriteSem()
	})
	commit = tx.Commit
	return
}

// Takes the exclusive right to write, giving up with ErrBeginWriteTimeout after
// maxWaitForBeginWriteTx. release must be called exactly once when the write
// transaction is done.
func (s sqlite) acquireWriteSemaphore() (release func(), err error) {
	timer := time.NewTimer(s.maxWaitForBeginWriteTx)
	select {
	case s.writeSem <- struct{}{}:
	case <-timer.C:
		return nil, fmt.Errorf("%w: waited %s for the write transaction in "+
			"progress to finish", ErrBeginWriteTimeout, s.maxWaitForBeginWriteTx)
	}
	return func() { <-s.writeSem }, nil
}

func (s sqlite) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	tx, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if !ok {
		panic("tried to query with a context not created by BeginRead/BeginWrite")
	}
	return tx.QueryContext(ctx, query, args...)
}

func (s sqlite) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	tx, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if !ok {
		panic("tried to queryRow with a context not created by BeginRead/BeginWrite")
	}
	return tx.QueryRowContext(ctx, query, args...)
}

func (s sqlite) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	tx, ok := ctx.Value(writeTxKey{}).(*sql.Tx)
	if !ok {
		panic("tried to exec with a context not created by BeginWrite")
	}
	res, err := tx.ExecContext(ctx, query, args...)
	// beginWriteWithCtx always sets a commitState alongside the write tx.
	cs := ctx.Value(commitStateKey{}).(*commitState)
	cs.hadWrite = true
	if err != nil {
		cs.hadError = true
	}
	return res, err
}

func (s sqlite) shouldCommit(ctx context.Context) bool {
	cs, ok := ctx.Value(commitStateKey{}).(*commitState)
	if !ok {
		panic("tried to call ShouldCommit with a context not created by BeginWrite")
	}
	return cs.hadWrite && !cs.hadError && !cs.preventCommit
}

func (s sqlite) preventCommit(ctx context.Context) {
	cs, ok := ctx.Value(commitStateKey{}).(*commitState)
	if !ok {
		panic("tried to call PreventCommit with a context not created by BeginWrite")
	}
	cs.preventCommit = true
}
