package track

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(
	configName string, js JobsStorage, db webdb.WebDb,
	tq TrackQueue, trackObs TrackObserver, secrets Secrets,
	cdQueue CdQueue,
	serverKeyAuthTrackMux wrappers.ServerKeyAuthTrackMux,
	serverKeyAndTokenAuthTrackMux wrappers.ServerKeyAndTokenAuthTrackMux) {
	h := handler{
		js:       js,
		db:       db,
		tq:       tq,
		trackObs: trackObs,
		secrets:  secrets,
		cdQueue:  cdQueue,
	}
	serverKeyAndTokenAuthTrackMux.HandleFunc("GET "+routes.TrackWebhooksSecrets, h.handleGetSecrets)
	serverKeyAuthTrackMux.HandleFunc("PUT "+routes.TrackWebhooksPath, h.handleTrackWebhook)
}

type JobsStorage interface {
	GetJobById(rl context.Context, id string) (job.Job, error)
	SetJobStatus(wl context.Context, id string, status job.JobStatus) error
	GetPipelineStagesById(tx context.Context, id string) (iterator.I[job.PipelineStage], error)
	SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status job.JobStatus) error
}

type TrackQueue interface {
	PutJobFinished(jobId string, tx context.Context) error
}

type TrackObserver interface {
	OnTrackWebhookReceived(job trackclient.TrackJob, payload runnerlib.JobPayload)
}

type Secrets interface {
	GetRepoIdSecret(rl context.Context, repoId uint64, secretName string) (secret string, isNotFoundErr bool, err error)
}

type CdQueue interface {
	// Enqueues the execution of a stage of a CD pipeline stage.
	// Note that whoever implements this must verify that the previous stage
	// we indeed succesfully marked as completed
	ResumeCdToStage(pipelineId string, Stage int32) error
}
