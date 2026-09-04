package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"monorepo/twigg-track/trackclient"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // register the driver
)

type sqlite struct {
	db            *sql.DB
	mu            *sync.RWMutex
	maxPayloadLen int
}

func newSqlite(pathToDir string) (Sqlite, error) {
	if pathToDir == InMemoryPathToDir {
		return newInMemorySqlite()
	}
	if !filepath.IsAbs(pathToDir) {
		currentDir, err := os.Getwd()
		if err != nil {
			return Sqlite{}, err
		}
		pathToDir = filepath.Join(currentDir, pathToDir)
	}
	const dbFileName = "track.db"
	err := os.MkdirAll(pathToDir, os.ModePerm)
	if err != nil {
		return Sqlite{}, fmt.Errorf("failed to mkdir %s: %s", pathToDir, err)
	}
	absPathToDbFile := filepath.Join(pathToDir, dbFileName)
	db, err := sql.Open(
		"sqlite",
		fmt.Sprintf("file:%s?_pragma=journal_mode=WAL&_pragma=synchronous=FULL", absPathToDbFile))
	if err != nil {
		return Sqlite{}, fmt.Errorf("failed to open db at %s: %s", absPathToDbFile, err)
	}
	return newSqliteFromSql(db)
}

func newSqliteFromSql(db *sql.DB) (Sqlite, error) {
	return Sqlite{&sqlite{
		db: db, mu: &sync.RWMutex{},
		maxPayloadLen: MaxPayloadLen}}, nil
}

func newInMemorySqlite() (Sqlite, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return Sqlite{}, err
	}
	db.SetMaxOpenConns(1)
	return newSqliteFromSql(db)
}

func (s *sqlite) close() {
	if s.db == nil {
		return
	}
	_ = s.db.Close()
}

func (s *sqlite) init() error {
	ctx, closeCtx, commitCtx, err := s.beginWriteWithCtx(context.Background())
	if err != nil {
		return err
	}
	defer closeCtx()
	_, err = s.exec(ctx, `
		CREATE TABLE IF NOT EXISTS jobs(
			id                  TEXT PRIMARY KEY,
			payload             BLOB NOT NULL,
			skipWebhooks        BOOLEAN NOT NULL DEFAULT FALSE,
			status              TEXT NOT NULL,
			createdAtMillis     INTEGER NOT NULL,
			finalDurationMillis INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS bestEffortCancels (
			id         TEXT PRIMARY KEY,
			createdAtMillis INTEGER NOT NULL
		) WITHOUT ROWID;
		CREATE INDEX IF NOT EXISTS idx_bec_created_at ON bestEffortCancels(createdAtMillis);

		CREATE TABLE IF NOT EXISTS jobs_migrations(
			name  TEXT PRIMARY KEY,
			done  BOOLEAN NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	return commitCtx()
}

type readTxKey struct{}
type writeTxKey struct{}

func (s sqlite) beginReadWithCtx(inputCtx context.Context) (ctx context.Context, closeTx func(), err error) {
	closeTx = func() {}
	ctx = inputCtx

	_, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if ok {
		err = errors.New("context already has a read tx")
		return
	}

	s.mu.RLock()
	tx, err := s.db.Begin()
	if err != nil {
		s.mu.RUnlock()
		return
	}
	ctx = context.WithValue(inputCtx, readTxKey{}, tx)
	closeTx = sync.OnceFunc(func() {
		_ = tx.Rollback()
		s.mu.RUnlock()
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

	s.mu.Lock()
	tx, err := s.db.Begin()
	if err != nil {
		s.mu.Unlock()
		return
	}
	ctx = context.WithValue(inputCtx, readTxKey{}, tx)
	ctx = context.WithValue(ctx, writeTxKey{}, tx)
	closeTx = sync.OnceFunc(func() {
		_ = tx.Rollback()
		s.mu.Unlock()
	})
	commit = tx.Commit
	return
}

func (s sqlite) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	tx, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if !ok {
		return s.db.QueryContext(ctx, query, args...)
	}
	return tx.QueryContext(ctx, query, args...)
}

func (s sqlite) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	tx, ok := ctx.Value(readTxKey{}).(*sql.Tx)
	if !ok {
		return s.db.QueryRowContext(ctx, query, args...)
	}
	return tx.QueryRowContext(ctx, query, args...)
}

func (s sqlite) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	tx, ok := ctx.Value(writeTxKey{}).(*sql.Tx)
	if !ok {
		return nil, errors.New("tried to exec outside a BeginWrite context")
	}
	return tx.ExecContext(ctx, query, args...)
}

func (s sqlite) Create(tx context.Context, id string, payload []byte, skipWebhooks bool) (trackclient.TrackJob, error) {
	if len(payload) > s.maxPayloadLen {
		return trackclient.TrackJob{}, ErrPayloadTooBig
	}
	status := trackclient.TrackJobStatusQueued
	createdAt := time.Now().UnixMilli()
	_, err := s.exec(tx, `
		INSERT INTO jobs(id, payload, skipWebhooks, status, createdAtMillis, finalDurationMillis)
		VALUES(?, ?, ?, ?, ?, ?)
	`, id, payload, skipWebhooks, status, createdAt, -1)
	if err != nil {
		return trackclient.TrackJob{}, err
	}
	return trackclient.TrackJob{
		Id:                  id,
		Payload:             payload,
		SkipWebhooks:        skipWebhooks,
		Status:              status,
		CreatedAtMillis:     createdAt,
		FinalDurationMillis: -1,
	}, nil
}

func (s sqlite) SetStatus(tx context.Context, id string, st trackclient.TrackJobStatus, executionTimeMillis int64) error {
	res, err := s.exec(tx, `
		UPDATE jobs
		SET status = ?, finalDurationMillis = ?
		WHERE id = ?
	`, st, executionTimeMillis, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s sqlite) Get(tx context.Context, id string) (job trackclient.TrackJob, isNotFoundErr bool, err error) {
	err = s.queryRow(tx, `
		SELECT id, payload, skipWebhooks, status, createdAtMillis, finalDurationMillis
		FROM jobs
		WHERE id = ?
	`, id).Scan(
		&job.Id,
		&job.Payload,
		&job.SkipWebhooks,
		&job.Status,
		&job.CreatedAtMillis,
		&job.FinalDurationMillis,
	)
	if errors.Is(err, sql.ErrNoRows) {
		isNotFoundErr = true
		err = ErrNotFound
		return
	}
	if err != nil {
		return trackclient.TrackJob{}, false, err
	}
	return job, false, nil
}

func (s sqlite) RequestBestEffortCancelation(tx context.Context, id string) error {
	now := time.Now().UnixMilli()
	_, err := s.exec(tx, `
        INSERT INTO bestEffortCancels (id, createdAtMillis)
        VALUES (?, ?)
        ON CONFLICT (id) DO NOTHING
    `, id, now)
	if err != nil {
		return err
	}
	// Add a probabilistic cleanup.
	// This prevents the table from growing forever, but doesnt require a
	// cleanup on every request
	const probabilisticCleanupPercent = 5 // Cleanup 5% of the times
	if rand.Intn(100) < probabilisticCleanupPercent {
		_ = s.cleanupBestEffortCancelation(tx, 24*time.Hour)
	}
	return nil
}
func (s sqlite) BestEffortCancelationWasRequested(tx context.Context, id string) (bool, error) {
	var dummyVar int
	err := s.queryRow(tx, `
        SELECT 1
        FROM bestEffortCancels
        WHERE id = ?
    `, id).Scan(&dummyVar)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s sqlite) cleanupBestEffortCancelation(tx context.Context, ttl time.Duration) error {
	expiryThreshold := time.Now().UnixMilli() - ttl.Milliseconds()
	_, err := s.exec(tx, `
            DELETE FROM bestEffortCancels 
            WHERE createdAtMillis < ?
        `, expiryThreshold)
	return err
}
