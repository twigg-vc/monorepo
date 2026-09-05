package cicdpublisher

import (
	"context"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/services/twiggtoken"
	"monorepo/twigg/server"
	"time"
)

// Saves jobs in the local db and publishes the jobs to external runners
type CiCdPublisher struct {
	s publisher
}

// Reads paths that a commit modifies with respect to its parent to find
// the jobs that must be created. Creates the jobs in the DB and puts them
// to the runner server.
// As `Put` suggests, this method is idempotent.
func (s CiCdPublisher) PutAutoCiCdRun(repoId, commitId, commitVersion uint64,
	runNumber int64, trigger runnerlib.JobTrigger, w context.Context) error {
	return s.s.PutAutoCiCdRun(repoId, commitId, commitVersion, runNumber, trigger, w)
}

// As `Put` suggests, this method is idempotent.
// It resumes a pipeline stage if it is in "waiting" stage.
// If the stage can auto-start, it Puts the payload to the trackqueue and
// sets the stage to "queued". Else, it just updates the stage status to
// "waiting manual start".
func (s CiCdPublisher) PutResumePipelineWaitingStage(pipelineId string, atStage int32, w context.Context) error {
	return s.s.PutResumePipelineWaitingStage(pipelineId, atStage, w)
}

// Resumes a pipeline that is "waiting for a manual start"
func (s CiCdPublisher) ManualResumePipeline(pipelineId string, currentStage int32, userId int64, w context.Context) (isCantResumeErr bool, err error) {
	return s.s.ManualResumePipeline(pipelineId, currentStage, userId, w)
}

// Launches a pipeline at the provided commit and commit version.
// The pipeline is created with runNumber as provided
// by GetRepoPipelineRefNextAvailableRunNumber
func (s CiCdPublisher) ManuallyLaunchCd(repoId, commitId, commitVersion uint64, jobPath, jobName string, userId int64, w context.Context) error {
	return s.s.ManuallyLaunchCd(repoId, commitId, commitVersion, jobPath, jobName, userId, w)
}

func New(serverProvider RepositoryServerProvider,
	tm MaxAllowedTimeoutGetter,
	jobs JobsStorage, pr Parser, tc TrackClient, flags FlagsProvider,
	signer twiggtoken.TokenSigner) CiCdPublisher {
	return CiCdPublisher{s: publisher{
		serverProvider: serverProvider,
		tm:             tm,
		jobs:           jobs,
		pr:             pr,
		tc:             tc,
		flags:          flags,
		signer:         signer,
	}}
}

const (
	CiFilename    = "CI.json"  // Name of the files that define a ci job
	MaxCiFileSize = 256 * 1024 // 256 kB
	CdFilename    = "CD.json"  // Name of the files that define a cd job
	MaxCdFileSize = 256 * 1024 // 256 kB
)

var (
	MaxJobsPerCommit = 100 // Hard limit for the max number of jobs that a commit can create
)

// Provides servers for each repository
type RepositoryServerProvider interface {
	GetRepoOwnerId(rl context.Context, repoId uint64) (int64, error)
	GetServerByRepoId(rl context.Context, repoId uint64) (server.Server, error)
	GetServerRead(rl context.Context) server.Read
}

// Stores jobs internally in the db
type JobsStorage interface {
	CiCdRunWasPublished(tx context.Context,
		repoId uint64, commit uint64, commitV uint64, runNumber int64) (bool, error)
	SetCiCdToPublished(tx context.Context,
		repoId uint64, commit uint64, commitV uint64, runNumber int64) error
	CreateNewJob(wl context.Context,
		repoId uint64, commit uint64, commitV uint64,
		filePath string, jobName string, runNumber int64,
		status job.JobStatus) (job.Job, error)
	PutPipelineRef(tx context.Context,
		repoId uint64, filePath string, jobName string) (job.PipelineRef, error)
	ArchivePipelineRefIfExists(tx context.Context,
		repoId uint64, filePath string, jobName string) error
	CreateNewPipeline(tx context.Context,
		repoId uint64, commit uint64, commitV uint64,
		filePath string, jobName string, runNumber int64,
		stageNames []string, isCreatedByUser bool, isCreatedByUserId int64) (job.Pipeline, error)
	SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status job.JobStatus) error
	GetPipelineStage(tx context.Context, pipelineId string, stage int32) (job.PipelineStage, error)
	GetRepoPipelineRefNextAvailableRunNumber(tx context.Context,
		repoId uint64, filePath string, jobName string) (int64, error)
	SetResumerOfPipelineStage(tx context.Context, pipelineId string, stage int32, userId int64) error
}

type MaxAllowedTimeoutGetter interface {
	// Returns the max timeout allowed for a single job
	GetMaxAllowedTimeout(repoOwnerId int64, repoId uint64, rl context.Context) (time.Duration, error)
}

// Client for posting jobs (externally) for execution
type TrackClient interface {
	// Send Job to track runner server
	Put(ownerId int64, jobId string, jobPayload runnerlib.JobPayload, tx context.Context) error
}

type Parser interface {
	ParseCiFile(ciFile []byte) (payloads []runnerlib.CiJob, ok bool, notOkMsg string)
	ParseCdFile(cdFile []byte) (payloads []runnerlib.CdJob, ok bool, notOkMsg string)
}

type FlagsProvider interface {
	GetFlagsByRepoOwnerUserId(repoOwnerUserId int64, tx context.Context) (featureflags.Flags, error)
}
