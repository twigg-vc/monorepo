package runnerlib

import "io"

// Runs jobs on a specified workdir by calling sucessive shell commands.
// Only works on linux.
type Runner struct {
	runner
}

// Constructor to correctly instantiate a Runner
func NewRunner(absPathToWorkdir string, stdOut, stdErr io.Writer) Runner {
	return Runner{
		runner: runner{
			absPathToWorkdir: absPathToWorkdir,
			stdOut:           stdOut,
			stdErr:           stdErr,
		},
	}
}

// Runs a job, logs the results to stdout and stderr and returns a status code
func (r Runner) Run(j JobPayload) (exitCode int) {
	return r.runner.Run(j)
}

// Parses a json file that contains one `CiJobJson` or a list of `CiJobJson`
func ParseCiJobs(ciFile []byte) (ciJobs []CiJob, ok bool, notOkErrMsg string) {
	return parseValidateAndExpandCiFile(ciFile)
}

// Parses a json file that contains one `CdJobJson` or a list of `CdJobJson`
func ParseCdJobs(cdFile []byte) (ciJobs []CdJob, ok bool, notOkErrMsg string) {
	return parseValidateAndExpandCdFile(cdFile)
}

// User-input of CI jobs
type CiJobJson struct {
	Name                string
	ImageName           ImageName
	Steps               []CiJobStepJson
	On                  []JobTrigger
	TimeoutMilliSeconds int64
	TimeoutSeconds      int64
	TimeoutMinutes      int64
}

// User-input of CI job step
type CiJobStepJson struct {
	TemplateName JobStepTemplate
	Run          string
	Env          map[string]string
	Secrets      []string
	Dir          string
}

// Final typed low level representation of a CI job.
type CiJob struct {
	On  []JobTrigger
	Job JobPayload
}

// User-input of CD jobs
type CdJobJson struct {
	Name   string
	On     []JobTrigger
	Stages []CdJobStageJson
}

// User input of each stage of a CD job
type CdJobStageJson struct {
	CanAutoStart        bool
	Name                string
	ImageName           ImageName
	Steps               []CiJobStepJson
	TimeoutMilliSeconds int64
	TimeoutSeconds      int64
	TimeoutMinutes      int64
}

// Final typed low level representation of a CD job.
type CdJob struct {
	Name   string
	On     []JobTrigger
	Stages []CdJobPayload
}

// Represents each stage of a CD job.
type CdJobPayload struct {
	CanAutoStart bool
	JobPayload
}

// Describes a Job to be executed in a single machine.
// This is the actual input that is passed to runners.
type JobPayload struct {
	Name                string
	ImageName           ImageName
	Steps               []JobStep
	TimeoutMilliSeconds int64
	Token               string
}

// Intermediary action executed by a job
type JobStep struct {
	Run     string
	Env     map[string]string
	Secrets []string
	Dir     string
}

// Specifies which image (as in "docker image") the job will run
type ImageName string

const (
	BaseImageAlias ImageName = ""
	BaseImage      ImageName = "base"
	GoImage        ImageName = "go"
	Node20Image    ImageName = "node-20"
	BunImage       ImageName = "bun"
	VmImage        ImageName = "vm"
)

var (
	SupportedImages        = []ImageName{BaseImageAlias, BaseImage, GoImage, Node20Image, BunImage, VmImage}
	SupportedCiJobTriggers = []JobTrigger{"", OnPush, OnSumit}
	SupportedCdJobTriggers = []JobTrigger{"", OnSumit, OnManual}
)

// Describes when a Job should run
type JobTrigger string

const (
	OnPush   JobTrigger = "push"
	OnSumit  JobTrigger = "submit"
	OnManual JobTrigger = "manual"
)

// Used to replace a simple string with a sequence of steps
type JobStepTemplate string

const (
	GetCodeJobStepTemplate      JobStepTemplate = "get-code"
	DebugGetCodeJobStepTemplate JobStepTemplate = "debug-get-code"
)

const (
	MaxJobPayloadSteps   = 100
	MaxJobPayloadEnv     = 50
	MaxJobPayloadSecrets = 50

	MaxCiJobOn = 10

	MaxCdJobOn     = 10
	MaxCdJobStages = 100
)

const (
	TwiggTokenEnvVarName = "TWIGG_TOKEN"
	CommitIdEnvVarName   = "COMMIT_ID" // format: `c123v456`
	RepoIdEnvVarName     = "REPO_ID"   // format: `id/id`
)
