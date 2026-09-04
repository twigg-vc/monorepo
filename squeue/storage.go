package squeue

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"monorepo/base/iterator"
	"monorepo/squeue/sqlock"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Stores all the messages that must be processed, as well as the dead-letter
// (messages that failed too many times)
type sqliteStorage struct {
	db sqlock.Db
	// Duration of the first retry
	baseRetryDelay time.Duration
	// Max allowed retry delay
	maxRetryDelay time.Duration
	// The n-th failure will be delayed by min(baseRetryDelay * (retryDelayMultiplier^n), maxDelay)
	retryDelayMultiplier int64
	// If retry_count >= maxNumberOfRetries. queue goes to dead_letter.
	maxNumberOfRetries int64
	// Returns the current time
	nowGetter NowGetter
	// Duration of the retry of calling the "onDeadLetter"
	onDeadLetterRetryDelay time.Duration
	// Number of times to retry the onDeadLetter
	onDeadLetterRetries int
}

func newSqliteStorage(absDirectoryPath string) (*sqliteStorage, func() error, error) {
	if !filepath.IsAbs(absDirectoryPath) {
		return nil, func() error { return nil }, fmt.Errorf(
			"absDirectoryPath=%q is not absolute path", absDirectoryPath)
	}
	const dbFileName = "queues.db"
	os.MkdirAll(absDirectoryPath, 0700)
	absPathToDbFile := filepath.Join(absDirectoryPath, dbFileName)
	sqliteDb, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?_busy_timeout=5000&_pragma=journal_mode=WAL", absPathToDbFile))
	if err != nil {
		return nil, func() error { return nil }, fmt.Errorf(
			"failed to open db at %s: %s", absPathToDbFile, err)
	}

	_, err = sqliteDb.Exec(`
		CREATE TABLE IF NOT EXISTS queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payload_type TEXT NOT NULL,
			payload BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			available_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			retry_count INTEGER DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS queue_available_idx ON queue(available_at);
		CREATE TABLE IF NOT EXISTS dead_letter (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payload_type TEXT NOT NULL,
			payload BLOB NOT NULL,
			original_created_at DATETIME NOT NULL,
			failed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			retry_count INTEGER NOT NULL
		);
    `)
	if err != nil {
		sqliteDb.Close()
		return nil, func() error { return nil }, fmt.Errorf(
			"failed to setup queue db : %s", err)
	}

	q := &sqliteStorage{
		db:                     sqlock.NewDb(sqliteDb),
		baseRetryDelay:         time.Second * 2, // MUST BE > 1s (bc its stored as timestamp)
		maxRetryDelay:          2 * time.Hour,
		retryDelayMultiplier:   2,
		maxNumberOfRetries:     20,
		onDeadLetterRetryDelay: time.Millisecond * 20,
		onDeadLetterRetries:    2,
	}
	log.Printf("[queue] %s", q.getRetrySummary())
	return q, sqliteDb.Close, nil
}

func (q sqliteStorage) getNow() time.Time {
	if q.nowGetter != nil {
		return q.nowGetter.Now()
	}
	return time.Now()
}
func formatSqlTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}
func (q sqliteStorage) getSqlFormattedNow() string {
	return formatSqlTime(q.getNow())
}

func (q *sqliteStorage) Enqueue(payloadType string, payload []byte) error {
	tx, unlock, err := q.db.Write()
	if err != nil {
		return err
	}
	defer unlock()
	timeNow := q.getSqlFormattedNow()
	_, err = tx.Exec(`INSERT INTO queue (payload_type, payload, created_at, available_at) VALUES (?, ?, ?, ?)`, payloadType, payload, timeNow, timeNow)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (q *sqliteStorage) Next(afterId int64, n int) ([]Entry, error) {
	if n <= 0 {
		panic("called Next with n<=0")
	}
	tx, unlock, err := q.db.Read()
	if err != nil {
		return nil, err
	}
	defer unlock()
	timeNow := q.getSqlFormattedNow()
	rows, err := tx.Query(`
		SELECT id, payload_type, payload
		FROM queue
		WHERE available_at <= ? AND id > ?
		ORDER BY id ASC;
	`, timeNow, afterId)
	if err != nil {
		return nil, err
	}
	return iterator.GetFirstN(n, entryIter{rows})
}

type entryIter struct {
	rows *sql.Rows
}

func (it entryIter) Get() (Entry, error) {
	var e Entry
	err := it.rows.Scan(&e.Id, &e.PayloadType, &e.Payload)
	return e, err
}
func (it entryIter) Next() bool {
	return it.rows.Next()
}
func (it entryIter) Err() error {
	return it.rows.Err()
}

func (q *sqliteStorage) Fail(entryId int64, onDeadLetter func(payload []byte) error) error {
	tx, unlock, err := q.db.Write()
	if err != nil {
		return err
	}
	defer unlock()

	var retryCount int64
	var createdAt string
	var payloadType string
	var payload []byte
	err = tx.QueryRow(`
        SELECT retry_count, created_at, payload_type, payload 
        FROM queue 
        WHERE id = ?`, entryId,
	).Scan(&retryCount, &createdAt, &payloadType, &payload)
	if err != nil {
		return err
	}
	// If exceeded retry limit, move to dead letter
	if retryCount+1 >= q.maxNumberOfRetries {
		if err := q.moveToDeadLetter(tx, entryId, createdAt,
			payloadType, payload, retryCount+1, onDeadLetter); err != nil {
			return err
		}
		err = tx.Commit()
		if err != nil {
			return err
		}
		log.Printf("[queue] moved payloadType=%s entryId=%v to the dead-letter",
			payloadType, entryId)
		return nil
	}

	exponent := math.Pow(float64(q.retryDelayMultiplier), float64(retryCount))
	delayDuration := time.Duration(exponent) * q.baseRetryDelay
	if delayDuration > q.maxRetryDelay {
		delayDuration = q.maxRetryDelay
	}
	newAvailableAt := formatSqlTime(q.getNow().Add(delayDuration))

	_, err = tx.Exec(`
        UPDATE queue
        SET 
            retry_count = retry_count + 1,
            available_at = ?
        WHERE id = ?`,
		newAvailableAt,
		entryId,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
func (q *sqliteStorage) Success(entryId int64) error {
	tx, unlock, err := q.db.Write()
	if err != nil {
		return err
	}
	defer unlock()
	_, err = tx.Exec(`DELETE FROM queue WHERE id = ?`, entryId)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (q *sqliteStorage) GetAllQueued() (it iterator.I[QueueItem], unlock func(), err error) {
	tx, unlock, err := q.db.Read()
	if err != nil {
		return
	}
	rows, err := tx.Query(`
		SELECT
			id,
			payload_type,
			payload,
			created_at,
			available_at,
			retry_count
		FROM
			queue
		ORDER BY 
			created_at ASC
	`)
	if err != nil {
		unlock()
		return nil, func() {}, fmt.Errorf("GetCommitJobs: query failed: %w", err)
	}
	return queuedRowsIter{rows}, unlock, nil
}

func (q *sqliteStorage) GetAllDeadLetter() (it iterator.I[DeadLetterItem], unlock func(), err error) {
	tx, unlock, err := q.db.Read()
	if err != nil {
		return
	}
	rows, err := tx.Query(`
		SELECT
			id,
			payload_type,
			payload,
			original_created_at,
			failed_at,
			retry_count
		FROM
			dead_letter
		ORDER BY
			failed_at DESC, id DESC 
	`)
	if err != nil {
		unlock()
		return nil, func() {}, fmt.Errorf("GetAllDeadLetter: query failed: %w", err)
	}
	return deadLetterRowsIter{rows: rows}, unlock, nil
}

func (q *sqliteStorage) moveToDeadLetter(tx sqlock.WriteTx,
	entryId int64,
	createdAt string,
	payloadType string,
	payload []byte,
	finalRetryCount int64,
	onDeadLetter func(payload []byte) error) error {

	_, err := tx.Exec(`
        INSERT INTO dead_letter (
            payload_type,
            payload,
            original_created_at,
			failed_at,
            retry_count
        )
        VALUES (?, ?, ?, ?, ?)`,
		payloadType,
		payload,
		createdAt,
		q.getSqlFormattedNow(),
		finalRetryCount,
	)
	if err != nil {
		return err
	}

	// Remove from main queue
	_, err = tx.Exec(`DELETE FROM queue WHERE id = ?`, entryId)
	if err != nil {
		return err
	}

	// Try running the onDeadLetter.
	// This is a "best-effort" call; we try a couple times only and ignore
	// errors if it doesn't suceed.
	if onDeadLetter == nil {
		onDeadLetter = func(payload []byte) error { return nil }
	}
	for i := 0; i < q.onDeadLetterRetries; i++ {
		onDeadLetterErr := onDeadLetter(payload)
		if onDeadLetterErr != nil {
			time.Sleep(q.onDeadLetterRetryDelay)
			continue
		}
	}
	return nil
}

func (q *sqliteStorage) RequeueDeadLetter(id int64) error {
	tx, unlock, err := q.db.Write()
	if err != nil {
		return err
	}
	defer unlock()

	var payloadType string
	var payload []byte

	err = tx.QueryRow(`
		SELECT payload_type, payload
		FROM dead_letter
		WHERE id = ?`,
		id,
	).Scan(&payloadType, &payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("dead-letter entry %d not found", id)
		}
		return err
	}

	// Enqueue back into queue
	_, err = tx.Exec(
		`INSERT INTO queue (payload_type, payload) VALUES (?, ?)`,
		payloadType, payload,
	)
	if err != nil {
		return err
	}

	// Delete from dead letter
	_, err = tx.Exec(`DELETE FROM dead_letter WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

type queuedRowsIter struct {
	rows *sql.Rows
}

func (it queuedRowsIter) Get() (QueueItem, error) {
	var q QueueItem
	if err := it.rows.Scan(
		&q.Id,
		&q.PayloadType,
		&q.Payload,
		&q.CreatedAt,
		&q.AvailableAt,
		&q.RetryCount,
	); err != nil {
		return QueueItem{}, fmt.Errorf("queuedRowsIter.Get: failed to scan QueueItem: %w", err)
	}
	return q, nil
}

func (it queuedRowsIter) Next() bool {
	return it.rows.Next()
}

func (it queuedRowsIter) Err() error {
	return it.rows.Err()
}

type deadLetterRowsIter struct {
	rows *sql.Rows
}

func (it deadLetterRowsIter) Next() bool {
	return it.rows.Next()
}

func (it deadLetterRowsIter) Get() (DeadLetterItem, error) {
	var d DeadLetterItem
	if err := it.rows.Scan(
		&d.Id,
		&d.PayloadType,
		&d.Payload,
		&d.OriginalCreatedAt,
		&d.FailedAt,
		&d.RetryCount,
	); err != nil {
		return DeadLetterItem{}, fmt.Errorf(
			"deadLetterRowsIter.Get: failed to scan DeadLetterItem: %w",
			err,
		)
	}
	return d, nil
}

func (it deadLetterRowsIter) Err() error {
	return it.rows.Err()
}

type testStorage struct {
	sqliteStorage
	t              *testing.T
	failCounter    int
	successCounter int
	n              NowGetter
}

func (q *testStorage) Fail(entryId int64, onDeadLetter func([]byte) error) error {
	q.failCounter += 1
	return q.sqliteStorage.Fail(entryId, onDeadLetter)
}
func (q *testStorage) Success(entryId int64) error {
	err := q.sqliteStorage.Success(entryId)
	if err != nil {
		return err
	}
	q.successCounter += 1
	return nil
}
func (q *testStorage) SetDelayMultiplier(delayMultiplier int64) {
	if delayMultiplier < 0 {
		panic("invalid delayMultiplier")
	}
	q.retryDelayMultiplier = delayMultiplier
}
func (q *testStorage) SetMaxNumberOfRetries(maxNumberOfRetries int64) {
	if maxNumberOfRetries <= 0 {
		panic("invalid maxNumberOfRetries")
	}
	q.maxNumberOfRetries = maxNumberOfRetries
}
func (q *testStorage) FailCount() int {
	return q.failCounter
}
func (q *testStorage) SuccessCount() int {
	return q.successCounter
}
func (q *testStorage) SetOnDeadLetterRetryDelay(d time.Duration) {
	q.onDeadLetterRetryDelay = d
}
func (q *testStorage) SetOnDeadLetterRetries(n int) {
	q.onDeadLetterRetries = n
}

func newtTestStorage(n NowGetter, t *testing.T) TestStorage {
	t.Helper()
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	pathToDir := filepath.Join(currentDir, "test-queue-dir")
	os.RemoveAll(pathToDir)

	tService, cleanup, err := newSqliteStorage(pathToDir)
	if err != nil {
		t.Fatalf("failed to create queue service: %v", err)
	}
	tService.nowGetter = n
	t.Cleanup(func() {
		cleanup()
		os.RemoveAll(pathToDir)
	})
	return TestStorage{&testStorage{
		sqliteStorage: *tService,
		t:             t,
	}}
}

func (q *sqliteStorage) getRetrySummary() string {
	var delays []string
	var totalDuration time.Duration
	// We'll track the first 3 delays specifically for the summary
	for i := int64(0); i < q.maxNumberOfRetries; i++ {
		// Exponential calculation: base * (multiplier ^ retryCount)
		exponent := math.Pow(float64(q.retryDelayMultiplier), float64(i))
		delay := time.Duration(exponent) * q.baseRetryDelay
		if delay > q.maxRetryDelay {
			delay = q.maxRetryDelay
		}
		totalDuration += delay
		// Grab the first 3 for the "30s, 1m, 2m..." part of the string
		if i < 3 {
			delays = append(delays, delay.String())
		}
	}
	return fmt.Sprintf("Messages will retry after %s, ..., %s for a total window of %s (%d retries)",
		strings.Join(delays, ", "),
		q.maxRetryDelay.String(),
		totalDuration.Round(time.Minute).String(), // Rounded for readability
		q.maxNumberOfRetries,
	)
}
