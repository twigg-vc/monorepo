package jobs

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
)

type Service interface {
	// Returns true if SetCiCdToPublished was ever called with the provided params
	CiCdRunWasPublished(tx context.Context,
		repoId uint64, commit uint64, commitV uint64, runNumber int64) (bool, error)
	// Mark a cicd run to having been published
	SetCiCdToPublished(tx context.Context,
		repoId uint64, commit uint64, commitV uint64, runNumber int64) error

	// Creates a new job with the provided id.
	CreateNewJob(wl context.Context,
		repoId uint64, commit uint64, commitV uint64,
		filePath string, jobName string, runNumber int64,
		status job.JobStatus) (job.Job, error)

	// GetJobById get a job by its ID from the jobs table.
	GetJobById(rl context.Context, id string) (job.Job, error)

	// SetJobStatus updates the status field of the job with the given ID.
	SetJobStatus(wl context.Context, id string, status job.JobStatus) error

	// GetCommitJobs returns an iterator over all jobs for a specific repo +
	// commit + commitVersion. Results are ordered by createdTime.
	// Use `afterInternalJobId` to read the jobs AFTER a previously read one
	// (use zero to read the latest)
	GetCommitJobs(rl context.Context,
		repoId uint64, commit uint64, afterInternalJobId int64) (iterator.I[job.Job], error)

	// GetRepoJobs returns an iterator over all jobs that belong to the given repo.
	// ordered by createdTime.
	// Use `afterInternalJobId` to read the jobs AFTER a previously read one
	// (use zero to read the latest)
	GetRepoJobs(rl context.Context, repoId uint64, afterInternalJobId int64) (iterator.I[job.Job], error)

	// Puts a pipeline ref so that GetRepoPipelineRefs returns it even
	// if no actual job pipeline was ever executed for it
	PutPipelineRef(tx context.Context,
		repoId uint64, filePath string, jobName string) (job.PipelineRef, error)
	// Marks a job.PipelineRef as archived so that GetRepoPipelineRefs no longer
	// returns it. Does nothing if the pipeline is already archived or doesnt exist.
	ArchivePipelineRefIfExists(tx context.Context,
		repoId uint64, filePath string, jobName string) error
	// Returns all pipeline refs of a repo, ordered by name.
	// Use `afterPath` and `afterJobName` to start after a ref.
	GetRepoPipelineRefs(tx context.Context,
		repoId uint64, afterPath string, afterJobName string) (iterator.I[job.PipelineRef], error)
	// Returns all pipelines by a ref, order by date of creation (newest first)
	// Use `afterInternalJobId` to read the jobs AFTER a previously read one
	// (use zero to read the latest)
	GetRepoPipelinesByRef(tx context.Context,
		repoId uint64, filePath string, jobName string, afterInternalJobId int64) (iterator.I[job.Pipeline], error)
	// Returns the next runNumber of the ref. Returns 0 if no pipeline exists yet.
	GetRepoPipelineRefNextAvailableRunNumber(tx context.Context,
		repoId uint64, filePath string, jobName string) (int64, error)

	// Creates a new instance of a job pipeline. The stages are created
	// with status "Waiting"; at least one stage is required
	CreateNewPipeline(tx context.Context,
		repoId uint64, commit uint64, commitV uint64,
		filePath string, jobName string, runNumber int64,
		stageNames []string, isCreatedByUser bool, createdByUserId int64) (job.Pipeline, error)
	// GetPipelineById gets a job.Pipeline by its Id.
	GetPipelineById(tx context.Context, id string) (job.Pipeline, error)
	// GetPipelineStagesById gets the stages of a job.Pipeline by its Id.
	GetPipelineStagesById(tx context.Context, id string) (iterator.I[job.PipelineStage], error)
	// SetStatusOfPipelineStage updates the status of a stage of a job.Pipeline.
	// Auto-updates the status of the pipeline itself when applicable
	SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status job.JobStatus) error
	// Sets IsResumedByUser=true and ResumedByUserId=userId of a pipeline stage
	SetResumerOfPipelineStage(tx context.Context, pipelineId string, stage int32, userId int64) error

	// Adaptor to set a job or the stage of a pipeline to posted
	SetToPosted(wl context.Context, jobIdOrPipelineStageId string) error
	// Adaptor to get a specific stage. Preffer to use the one that returns iterator.I[job.PipelineStage]
	// if you need more than one stage; as this is a simple wrapper around it
	GetPipelineStage(tx context.Context, pipelineId string, stage int32) (job.PipelineStage, error)
	// Returns true if the previous stage succeeded
	CanPutResumePipelineToStage(tx context.Context, pipelineId string, stage int32) (bool, error)
}

// Does the necessary setup and returns service instance
func NewService(db webdb.WebDb, wl context.Context) (Service, error) {
	return newService(db, wl)
}
