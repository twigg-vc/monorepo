package trackqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/base/iterator"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"time"
)

type trackQueue struct {
	db Db
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
	statusQueued    = "queued"
	statusPublished = "published"

	defaultMaxRunningJobs      = 1
	defaultMaxRunningTimeoutMs = 60_000
)

func newTrackQueue(js JobsStorage, tc TrackClient, db Db,
	options ...Option) (*trackQueue, error) {
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
	ownerId, payloadBytes, isNotFoundErr, err := q.db.GetTrackQueueJobOwnerAndPayload(tx, jobId)
	if isNotFoundErr {
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
	err = q.db.AddTrackOwnerUsage(tx, ownerId, -1, -timeoutMs)
	if err != nil {
		return err
	}
	return q.db.DeleteTrackQueueJob(tx, jobId)
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
	jobId, ownerId, payloadBytes, isNotFoundErr, err :=
		q.db.GetOldestTrackQueueJobWithinOwnerLimits(tx, statusQueued)
	if isNotFoundErr {
		return
	}
	if err != nil {
		log.Printf("%sfailed to get next job: %s", logPrefix, err)
		return
	}
	var pl runnerlib.JobPayload
	err = json.Unmarshal(payloadBytes, &pl)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal payload: %s", err))
	}
	// Reserve the usage
	err = q.db.AddTrackOwnerUsage(tx, ownerId, 1, pl.TimeoutMilliSeconds)
	if err != nil {
		log.Printf("%sfailed to reserve usage: %s", logPrefix, err)
		return
	}
	// Mark the job as published
	err = q.db.SetTrackQueueJobStatus(tx, jobId, statusPublished)
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
	jobIds, err := q.db.GetTrackQueueJobIdsByStatus(readTx, statusPublished)
	if err != nil {
		log.Printf("%sfailed to get jobs to cleanup: %s", logPrefix, err)
		return
	}
	cleanupJobIdCandidates, err := iterator.GetFirstN(cleanupBatchSize, jobIds)
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
