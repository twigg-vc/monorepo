package job

import (
	"encoding/base64"
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
