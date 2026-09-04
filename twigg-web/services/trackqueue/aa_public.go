package trackqueue

import (
	"context"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/webdb"
	"time"
)

// MUST BE INITIALIZED WITH `New`
// Queue that posts jobs to be run on the track servers.
// It throttles jobs per "owner" so that the track capacity is not monopolized
// by a single runner
type TrackQueue struct {
	j *trackQueue
}

func New(js JobsStorage, tc TrackClient, setupTx context.Context, db webdb.WebDb, options ...Option) (TrackQueue, error) {
	j, err := newTrackQueue(js, tc, setupTx, db, options...)
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