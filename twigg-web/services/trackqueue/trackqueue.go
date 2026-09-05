package trackqueue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/webdb"
	"time"
)

type trackQueue struct {
	db webdb.WebDb
	js JobsStorage
	tc TrackClient

	pollInterval        time.Duration
	janitorPollInterval time.Duration

	loopWakeupCh chan bool
	loopStopCh   chan bool
	loopDoneCh   chan bool
	obs          []Observer

	janitorStopCh chan bool
	janitorDoneCh chan bool
}

const (
	statusQueued = "queued"

	defaultMaxRunningJobs      = 1
	defaultMaxRunningTimeoutMs = 60_000
)

func newTrackQueue(js JobsStorage, tc TrackClient, setupTx context.Context, db webdb.WebDb, options ...Option) (*trackQueue, error) {
	// Table/index creation
	_, err := db.Bind(setupTx).Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS track_queue (
			job_id        TEXT PRIMARY KEY,
			owner_id      BIGINT NOT NULL,
			payload       BLOB NOT NULL,
			status        TEXT NOT NULL,
			created_at_ns BIGINT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS track_queue_pick
		ON track_queue (status, created_at_ns);

		CREATE TABLE IF NOT EXISTS owner_usage2 (
			owner_id               BIGINT PRIMARY KEY,
			running_jobs           INTEGER NOT NULL,
			running_timeout_ms     BIGINT NOT NULL,
			max_running_jobs       INTEGER NOT NULL DEFAULT %d,
			max_running_timeout_ms INTEGER NOT NULL DEFAULT %d
		);
	`, defaultMaxRunningJobs, defaultMaxRunningTimeoutMs))
	if err != nil {
		return nil, err
	}

	q := &trackQueue{
		db: db,
		js: js,
		tc: tc,

		loopStopCh:   make(chan bool, 1),
		loopDoneCh:   make(chan bool, 1),
		loopWakeupCh: make(chan bool, 1),

		janitorStopCh: make(chan bool, 1),
		janitorDoneCh: make(chan bool, 1),

		pollInterval:        3 * time.Second,
		janitorPollInterval: 15 * time.Minute,
	}
	for _, op := range options {
		op(q)
	}
	return q, nil
}

func (q *trackQueue) Start() {
	go q.loop()
	go q.janitorLoop()
}

func (q *trackQueue) Stop() {
	close(q.loopStopCh)
	<-q.loopDoneCh
	close(q.janitorStopCh)
	<-q.janitorDoneCh
}

func (q *trackQueue) loop() {
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()
	defer close(q.loopDoneCh)
	for {
		select {
		case <-q.loopStopCh:
			return
		case <-ticker.C:
			q.tryPublishOne()
		case <-q.loopWakeupCh:
			q.tryPublishOne()
		}
		for _, obs := range q.obs {
			obs.OnLoop()
		}
	}
}
func (q *trackQueue) janitorLoop() {
	ticker := time.NewTicker(q.janitorPollInterval)
	defer ticker.Stop()
	defer close(q.janitorDoneCh)
	for {
		select {
		case <-q.janitorStopCh:
			return
		case <-ticker.C:
			q.tryCleanup()
		}
		for _, obs := range q.obs {
			obs.OnJanitorLoop()
		}
	}
}

func (q *trackQueue) Put(ownerId int64, jobId string, pl runnerlib.JobPayload, tx context.Context) error {
	payloadBytes, err := json.Marshal(pl)
	if err != nil {
		return err
	}
	now := time.Now().UnixNano()
	err = q.db.InsertTrackQueueJobIfNotExists(tx, jobId, ownerId, payloadBytes,
		statusQueued, now)
	if err != nil {
		return err
	}
	err = q.db.InsertZeroTrackOwnerUsageIfNotExists(tx, ownerId)
	if err != nil {
		return err
	}
	// Send wakeup if channel is not full
	select {
	case q.loopWakeupCh <- true:
		// Signal sent, loop will wake up.
	default:
		// Drop wakeup because channel is full.
	}
	return nil
}

func (q *trackQueue) PutJobFinished(jobId string, tx context.Context) error {
	var ownerId int64
	var payloadBytes []byte
	err := q.db.Bind(tx).QueryRow(`
		SELECT owner_id, payload
		FROM track_queue
		WHERE job_id = ?
	`, jobId).Scan(&ownerId, &payloadBytes)
	if errors.Is(err, sql.ErrNoRows) {
		// No work to do
		return nil
	}
	if err != nil {
		return err
	}
	var pl runnerlib.JobPayload
	if err := json.Unmarshal(payloadBytes, &pl); err != nil {
		return err
	}
	timeoutMs := pl.TimeoutMilliSeconds
	if _, err := q.db.Bind(tx).Exec(`
		UPDATE owner_usage2
		SET
			running_jobs = running_jobs - 1,
			running_timeout_ms = running_timeout_ms - ?
		WHERE owner_id = ?
	`, timeoutMs, ownerId); err != nil {
		return err
	}
	_, err = q.db.Bind(tx).Exec(`
		DELETE FROM track_queue
		WHERE job_id = ?
	`, jobId)
	return err
}

func (q *trackQueue) PutLimits(ownerId int64, maxJobs int,
	maxTimeout time.Duration, tx context.Context) error {
	err := q.db.SetTrackOwnerLimits(tx, ownerId, int64(maxJobs),
		maxTimeout.Milliseconds())
	if err != nil {
		return err
	}
	// Send wakeup if channel is not full
	select {
	case q.loopWakeupCh <- true:
		// Signal sent, loop will wake up.
	default:
		// Drop wakeup because channel is full.
	}
	return nil
}

func (q *trackQueue) GetLimits(ownerId int64, tx context.Context) (maxJobs int,
	maxTimeout time.Duration, err error) {
	maxJobsInt64, maxTimeoutMs, isNotFoundErr, err := q.db.GetTrackOwnerLimits(tx, ownerId)
	if isNotFoundErr {
		err = nil
		maxJobsInt64 = defaultMaxRunningJobs
		maxTimeoutMs = defaultMaxRunningTimeoutMs
	}
	if err != nil {
		return
	}
	maxJobs = int(maxJobsInt64)
	maxTimeout = time.Duration(maxTimeoutMs) * time.Millisecond
	return
}

func (q *trackQueue) tryPublishOne() {
	tx, close, commit, err := q.db.BeginWrite()
	if err != nil {
		log.Printf("%sfailed to get tx: %s", logPrefix, err)
		return
	}
	defer close()

	// Pick the oldest job which has an elligible owner
	var jobId string
	var ownerId int64
	var payloadBytes []byte
	err = q.db.Bind(tx).QueryRow(`
		SELECT q.job_id, q.owner_id, q.payload
		FROM track_queue q
		JOIN owner_usage2 u ON u.owner_id = q.owner_id
		WHERE q.status = 'queued'
		  AND u.running_jobs < u.max_running_jobs
		  AND u.running_timeout_ms < u.max_running_timeout_ms
		ORDER BY q.created_at_ns
		LIMIT 1
	`,
	).Scan(&jobId, &ownerId, &payloadBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("%sfailed to get next job: %s", logPrefix, err)
		return
	}
	var pl runnerlib.JobPayload
	err = json.Unmarshal(payloadBytes, &pl)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal payload: %s", err))
	}
	// Reserve the usage
	_, err = q.db.Bind(tx).Exec(`
		UPDATE owner_usage2
		SET
			running_jobs = running_jobs + 1,
			running_timeout_ms = running_timeout_ms + ?
		WHERE owner_id = ?
	`,
		pl.TimeoutMilliSeconds,
		ownerId,
	)
	if err != nil {
		log.Printf("%sfailed to reserve usage: %s", logPrefix, err)
		return
	}
	// Mark the job as published
	_, err = q.db.Bind(tx).Exec(`
		UPDATE track_queue
		SET status = 'published'
		WHERE job_id = ?
	`, jobId)
	if err != nil {
		log.Printf("%sfailed to mark publish: %s", logPrefix, err)
		return
	}
	err = q.js.SetToPosted(tx, jobId)
	if err != nil {
		log.Printf("%sfailed set job to posted: %s", logPrefix, err)
		return
	}
	// Put the job to the runner BEFORE commiting.
	// This will be retried if commiting the tx fails
	err = q.tc.Put(jobId, pl)
	if err != nil {
		return
	}
	_ = commit()
}

func (q *trackQueue) tryCleanup() {
	readTx, closeReadTx, err := q.db.BeginRead()
	if err != nil {
		log.Printf("%sfailed to get read tx: %s", logPrefix, err)
		return
	}
	defer closeReadTx()

	// Pick some published jobs and check track server if they already finished
	const cleanupBatchSize = 50
	cleanupJobIdCandidates := make([]string, 0, cleanupBatchSize)
	rows, err := q.db.Bind(readTx).Query(`
		SELECT job_id
		FROM track_queue
		WHERE status = 'published'
		ORDER BY created_at_ns ASC
	`)
	if err != nil {
		log.Printf("%sfailed to get jobs to cleanup: %s", logPrefix, err)
		return
	}
	for rows.Next() {
		var jobId string
		err = rows.Scan(&jobId)
		if err != nil {
			log.Printf("%sfailed to scan cleanup jobs: %s", logPrefix, err)
			return
		}
		cleanupJobIdCandidates = append(cleanupJobIdCandidates, jobId)
	}
	err = rows.Err()
	if err != nil {
		log.Printf("%sfailed to iterate on cleanup jobs: %s", logPrefix, err)
		return
	}
	if len(cleanupJobIdCandidates) == 0 {
		return
	}
	closeReadTx()

	// Check with the track server which jobs finished running
	jobIdsToCleanup := []string{}
	for _, jobId := range cleanupJobIdCandidates {
		j, _, isNotFoundErr, err := q.tc.Get(jobId)
		if err != nil && !isNotFoundErr {
			continue
		}
		if isNotFoundErr || j.Status != trackclient.TrackJobStatusQueued {
			jobIdsToCleanup = append(jobIdsToCleanup, jobId)
		}
	}
	if len(jobIdsToCleanup) == 0 {
		return
	}

	// Mark them as finished
	writeTx, closeWriteTx, commit, err := q.db.BeginWrite()
	if err != nil {
		log.Printf("%sfailed to get write tx: %s", logPrefix, err)
		return
	}
	defer closeWriteTx()
	for _, jobId := range jobIdsToCleanup {
		err = q.PutJobFinished(jobId, writeTx)
		if err != nil {
			log.Printf("%sfailed to get cleanup job: %s", logPrefix, err)
			return
		}
	}
	err = commit()
	if err != nil {
		log.Printf("%sfailed to commit cleanup: %s", logPrefix, err)
		return
	}
}

const logPrefix = "[trackqueue] "
