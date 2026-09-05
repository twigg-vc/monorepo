package job

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

type Job struct {
	InternalId    int64
	RepoId        uint64
	Commit        uint64
	CommitVersion uint64
	Path          string // Path to file that defines job
	Name          string // Name of the job in the file
	RunNumber     int64  // n-th time that the job was run
	Status        JobStatus
	CreatedTime   string
}

// Returns a url-safe string that uniquely identifies a job run
func JobId(RepoId uint64, Commit uint64, CommitVersion uint64,
	Path string, Name string, RunNumber int64) string {
	parts := []string{
		strconv.FormatUint(RepoId, 10),
		strconv.FormatUint(Commit, 10),
		strconv.FormatUint(CommitVersion, 10),
		base64.RawURLEncoding.EncodeToString([]byte(Path)),
		base64.RawURLEncoding.EncodeToString([]byte(Name)),
		strconv.FormatInt(RunNumber, 10),
	}
	return strings.Join(parts, ".")
}
func (j Job) Id() string {
	return JobId(j.RepoId, j.Commit, j.CommitVersion, j.Path,
		j.Name, j.RunNumber)
}
func ParseJobId(id string) (RepoId uint64, Commit uint64, CommitVersion uint64,
	Path string, Name string, RunNumber int64, ok bool) {
	return parseJobId(id)
}

type Pipeline struct {
	InternalId      int64
	RepoId          uint64
	Commit          uint64
	CommitVersion   uint64
	Path            string // Path to file that defines job
	Name            string // Name of the job in the file
	RunNumber       int64  // n-th time that the job was run
	NumberOfStages  int32  // Number of stages in the pipeline
	Status          PipelineStatus
	CreatedTime     string
	IsCreatedByUser bool  // Indicates a user manually lanched it
	CreatedByUserId int64 // Indicates the id of the user who launched it
}

// Returns true if a string might be and id of a pipeline
func MightBePipelineStageId(id string) bool {
	return pipelineStageIdRegexp.MatchString(id)
}

func (p Pipeline) Id() string {
	return PipelineId(p.RepoId, p.Commit, p.CommitVersion, p.Path, p.Name, p.RunNumber)
}
func (p Pipeline) IdOfStage(stage int32) string {
	return PipelineStageId(p.Id(), stage)
}

func PipelineId(RepoId uint64, Commit uint64, CommitVersion uint64,
	Path string, Name string, RunNumber int64) string {
	return pipelineIdPrefix + JobId(RepoId, Commit, CommitVersion, Path, Name, RunNumber)
}
func PipelineStageId(pipelineId string, stage int32) string {
	return fmt.Sprintf("%s%s%d", pipelineId, pipelineStageIdSuffix, stage)
}
func ParsePipelineId(id string) (RepoId uint64, Commit uint64, CommitVersion uint64,
	Path string, Name string, RunNumber int64, ok bool) {
	return ParseJobId(strings.TrimPrefix(id, pipelineIdPrefix))
}
func ParsePipelineStageId(id string) (RepoId uint64, Commit uint64, CommitVersion uint64,
	Path string, Name string, RunNumber int64, Stage int32, ok bool) {
	return parsePipelineStageId(id)
}

type JobStatus string

const (
	// Job is waiting for an user input to start
	JobStatusWaitingManualStart JobStatus = "waiting-manual-start"
	// Job is waiting for a previous job to finish
	JobStatusWaiting JobStatus = "waiting"
	// Job is queued to be posted
	JobStatusQueued JobStatus = "queued"
	// Job was posted to the runner
	JobStatusPosted JobStatus = "posted"
	// Job is running
	JobStatusRunning JobStatus = "running"
	// Job finished running and succeeded
	JobStatusSuccess JobStatus = "success"
	// Job finished running and failed
	JobStatusFail JobStatus = "fail"
	// Job exectution was canceled due to timeout
	JobStatusTimeout JobStatus = "timeout"
	// Job exectution was canceled "manually"
	JobStatusCanceled JobStatus = "cancel"
	// Job exectution was canceled because the max number of jobs per commit was reached
	JobStatusTooManyJobs JobStatus = "too-many-jobs"
	// Bad job file format
	JobStatusBadFileFormat JobStatus = "bad-file-format"
	// Bad job file size
	JobStatusBadFileSize JobStatus = "bad-file-size"
	// Bad job file size
	JobStatusExceedsPlanLimits JobStatus = "exceeds-plan-limits"
)

type PipelineStatus string

const (
	PipelineStatusWaitingManualStart PipelineStatus = "waiting-manual-start"
	PipelineStatusRunning            PipelineStatus = "running"
	PipelineStatusSuccess            PipelineStatus = "success"
	PipelineStatusFail               PipelineStatus = "fail"
	PipelineStatusCancel             PipelineStatus = "cancel"
)
