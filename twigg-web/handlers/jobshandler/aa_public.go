package jobshandler

import (
	"context"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(
	ug UsernameGetter,
	jobsService JobsService,
	tc TrackClient,
	pr PipelineResumer,
	userRepoMux wrappers.UserRepoMux,
	userRepoPipelineMux wrappers.UserRepoPipelineMux) {
	h := NewHandler(ug, jobsService, tc, pr)
	userRepoMux.HandleFuncR("GET "+routes.PipelineRefsPattern, h.HandleGetPipelineRefs)
	userRepoMux.HandleFuncR("GET "+routes.RefPipelinesPattern, h.HandleGetRefPipelines)
	userRepoMux.HandleFuncW("POST "+routes.ManuallyLaunchPipelinePattern, h.HandleManuallyLaunchPipeline)
	userRepoPipelineMux.HandleFuncR("GET "+routes.PipelinePattern, h.HandleGetPipeline)
	userRepoPipelineMux.HandleFuncR("GET "+routes.PipelineStagesPattern, h.HandleGetPipelineStages)
	userRepoPipelineMux.HandleFuncR("GET "+routes.PipelineStageCombinedOutPattern, h.HandleGetStageCombinedOut)
	userRepoPipelineMux.HandleFuncW("POST "+routes.ManualResumePipelineStagePattern, h.HandleManualResumeStage)
	userRepoPipelineMux.HandleFuncW("PUT "+routes.CancelPipelineStagePattern, h.HandleCancelPipelineStage)
	userRepoPipelineMux.HandleFuncR("GET "+routes.PipelineStageIsCancelledPattern, h.HandleGetPipelineStageIsCanceled)
}

func NewHandler(ug UsernameGetter, js JobsService, tc TrackClient, pr PipelineResumer) handler {
	return handler{
		usernameGetter: ug,
		jobsService:    js,
		tc:             tc,
		pr:             pr,
	}
}

type FrontendPipeline struct {
	Id string
	job.Pipeline
	CreatedByUsername string
}

type FrontendPipelineStage struct {
	PipelineId        string
	Name              string
	Stage             int32
	IsResumedByUser   bool
	ResumedByUsername string
	CreatedTime       string
	Status            job.JobStatus
}

type JobsService interface {
	GetRepoPipelineRefs(tx context.Context,
		repoId uint64, filePath string, jobName string) (iterator.I[job.PipelineRef], error)
	GetRepoPipelinesByRef(tx context.Context,
		repoId uint64, filePath string, jobName string, afterInternalJobId int64) (iterator.I[job.Pipeline], error)
	GetPipelineById(tx context.Context, id string) (job.Pipeline, error)
	GetPipelineStagesById(tx context.Context, id string) (iterator.I[job.PipelineStage], error)
	GetPipelineStage(tx context.Context, pipelineId string, stage int32) (job.PipelineStage, error)
}

type TrackClient interface {
	GetCombinedOutput(jobId string) (io.ReadCloser, error)
	Cancel(jobId string) error
}

type PipelineResumer interface {
	ManualResumePipeline(pipelineId string, currentStage int32, userId int64, w context.Context) (isCantResumeErr bool, err error)
	ManuallyLaunchCd(repoId, commitId, commitVersion uint64, jobPath, jobName string, userId int64, w context.Context) error
}

type UsernameGetter interface {
	GetUsername(userId int64, tx context.Context) (string, error)
}
