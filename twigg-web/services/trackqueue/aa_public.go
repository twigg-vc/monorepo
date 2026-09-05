package trackqueue

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"time"
)

// MUST BE INITIALIZED WITH `New`
// Queue that posts jobs to be run on the track servers.
// It throttles jobs per "owner" so that the track capacity is not monopolized
// by a single runner
type TrackQueue struct {
	j *trackQueue
}

func New(js JobsStorage, tc TrackClient, db Db, options ...Option) (TrackQueue, error) {
	j, err := newTrackQueue(js, tc, db, options...)
	return TrackQueue{j}, err
}

// Starts posting the jobs
func (j TrackQueue) Start() {
	j.j.Start()
}

// Stop running the queue
func (j TrackQueue) Stop() {
	j.j.Stop()
}

// Add an observer
func (j TrackQueue) AddObserver(o Observer) {
	j.j.obs = append(j.j.obs, o)
}

// Add (idempotent) a job to a queue so that it's put to run on the track
// as soon as the owner's allocated bandwith allows
func (j TrackQueue) Put(ownerId int64, jobId string, pl runnerlib.JobPayload, tx context.Context) error {
	return j.j.Put(ownerId, jobId, pl, tx)
}

// Sets (idempotent) the max jobs and max sum of timeout a user can run in parallel
func (q TrackQueue) PutLimits(ownerId int64, maxJobs int,
	maxTimeout time.Duration, tx context.Context) error {
	return q.j.PutLimits(ownerId, maxJobs, maxTimeout, tx)
}

// Returns the limits set for the specified user.
// If PutLimits was never called, will return the default values
func (q TrackQueue) GetLimits(ownerId int64, tx context.Context) (maxJobs int,
	maxTimeout time.Duration, err error) {
	return q.j.GetLimits(ownerId, tx)
}

// Idempotently marks the job as finished.
// It's essential to call this once jobs are done running, else the owner's
// bandwidth will be blocked.
func (j TrackQueue) PutJobFinished(jobId string, tx context.Context) error {
	return j.j.PutJobFinished(jobId, tx)
}

// The storage the queue needs. The loops run outside of the request handlers,
// so they open their own transactions.
type Db interface {
	BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error)
	BeginRead() (readCtx context.Context, closeTx func(), err error)

	InsertTrackQueueJobIfNotExists(writeCtx context.Context, jobId string,
		ownerId int64, payload []byte, status string, createdAtNs int64) error
	GetTrackQueueJobOwnerAndPayload(ctx context.Context, jobId string) (
		ownerId int64, payload []byte, isNotFoundErr bool, err error)
	GetOldestTrackQueueJobWithinOwnerLimits(ctx context.Context, status string) (
		jobId string, ownerId int64, payload []byte, isNotFoundErr bool, err error)
	GetTrackQueueJobIdsByStatus(ctx context.Context, status string) (iterator.I[string], error)
	SetTrackQueueJobStatus(writeCtx context.Context, jobId string, status string) error
	DeleteTrackQueueJob(writeCtx context.Context, jobId string) error

	InsertZeroTrackOwnerUsageIfNotExists(writeCtx context.Context, ownerId int64) error
	AddTrackOwnerUsage(writeCtx context.Context, ownerId int64,
		runningJobsDelta int64, runningTimeoutMsDelta int64) error
	SetTrackOwnerLimits(writeCtx context.Context, ownerId int64,
		maxJobs int64, maxTimeoutMs int64) error
	GetTrackOwnerLimits(ctx context.Context, ownerId int64) (
		maxJobs int64, maxTimeoutMs int64, isNotFoundErr bool, err error)
}

type TrackClient interface {
	Get(jobId string) (tj trackclient.TrackJob, pl runnerlib.JobPayload, isNotFoundErr bool, err error)
	Put(jobId string, jobPayload runnerlib.JobPayload) error
}

type JobsStorage interface {
	SetToPosted(wl context.Context, jobId string) error
}

type Observer interface {
	OnLoop()        // Called when the queue runs a loop
	OnJanitorLoop() // Called when the queue runs a janitor loop
}

type Option func(*trackQueue)

func WithPoolInternal(d time.Duration) Option {
	return func(q *trackQueue) { q.pollInterval = d }
}
func WithJanitorPoolInterval(d time.Duration) Option {
	return func(q *trackQueue) { q.janitorPollInterval = d }
}
