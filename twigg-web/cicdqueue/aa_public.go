package cicdqueue

import (
	"context"
	"monorepo/twigg-runner/runnerlib"
)

// Service for creating/resuming CI/CD jobs.
// It's sole resposibility is orchestrating a queue and calling other services
// that actually check which files changed, build the CI/CD files and put
// them to be run.
// Must be created with `New` constructor.
type Service struct {
	s service
}

type Db interface {
	BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error)
	GetCiCdQueueLastRunNumber(rl context.Context, repoId, commitId, commitVersion uint64) (runNumber int64, isNotFoundErr bool, err error)
	InsertCiCdQueueRun(wl context.Context, repoId, commitId, commitVersion uint64, runNumber int64, trigger, nonce, status string) error
	GetCiCdQueueLatestRunStatus(rl context.Context, repoId, commitId, commitVersion uint64) (status string, isNotFoundErr bool, err error)
	GetCiCdQueueRunTriggerAndStatus(rl context.Context, repoId, commitId, commitVersion uint64, runNumber int64, nonce string) (trigger, status string, isNotFoundErr bool, err error)
	SetCiCdQueueRunStatus(wl context.Context, repoId, commitId, commitVersion uint64, runNumber int64, nonce, status string) error
}

func New(js JobsStorage, ciCdPublisher CiCdJobsPoster,
	db Db, queue Queue) (Service, error) {
	return newService(js, ciCdPublisher, db, queue)
}

// Enqueues the creation and execution of all CI/CD jobs of the specified commit.
// The jobs will only actually be put after the transaction is commited and closed.
func (s Service) EnqueueCiCdRun(repoId, commitId, commitVersion uint64,
	trigger runnerlib.JobTrigger, w context.Context) (runNumber int64, err error) {
	return s.s.EnqueueCiCdRun(repoId, commitId, commitVersion, trigger, w)
}

// Returns the status of the highest runNumber
func (s Service) GetCiCdLatestRunStatus(repoId, commitId, commitVersion uint64, r context.Context) (CiCdStatus, error) {
	return s.s.GetCiCdLatestRunStatus(repoId, commitId, commitVersion, r)
}

// Checks that the pipeline can resume to stageN and enques that stage
// for exexcution.
func (s Service) ResumeCdToStage(pipelineId string, stageN int32) error {
	return s.s.ResumeCdToStage(pipelineId, stageN)
}

type CiCdStatus string

const (
	// Null value
	CiCdStatusNone CiCdStatus = "none"
	// The queue has not yet started the jobs
	CiCdStatusPrepared CiCdStatus = "prepared"
	// The queue finished starting the jobs. They might still be running
	// or finished running.
	CiCdStatusStarted CiCdStatus = "started"
)

type JobsStorage interface {
	// Returns true if we can issue an "idempotent-resume" to the pipeline
	// so that it resumes from stage-1 TO the provided stage.
	// Note that it doesn't need to check the status of the stage itself,
	// because an "idempotent-resume" is responsible for doing that
	CanPutResumePipelineToStage(tx context.Context, pipelineId string, stage int32) (bool, error)
}

type CiCdJobsPoster interface {
	// Creates the jobs internally at the DB and puts them to a runner.
	// Must be idempotent.
	PutAutoCiCdRun(repoId, commitId, commitVersion uint64,
		runNumber int64, trigger runnerlib.JobTrigger, w context.Context) error
	// Continues the execution of a CD pipeline that is in a "waiting" stage.
	// Does nothing if the stage is != waiting.
	// Must be idempotent.
	PutResumePipelineWaitingStage(pipelineId string, atStage int32, w context.Context) error
}

type Queue interface {
	Register(payloadType string,
		handler func(payload []byte) error,
		decoder func(payload []byte) string,
		onDeadLetter func(payload []byte) error)
	Enqueue(payloadType string, payload []byte) error
}

const (
	QueueStartAutoCiCdRunPayloadType = "start-commit-ci2"
	QueueResumeCdPayloadType         = "run-cd-stage"
)
