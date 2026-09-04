package jobs

import (
	"context"
	"io"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-track/wrappers"
	"time"
)

func AddHandlers(maxJobTimeout time.Duration, db JobsDb, runnersService RunnersService,
	jobsQueue, webhookQueue Queue, twiggWebClient TwiggWebClient, twiggServerUrl string,
	twiggServerWebhookPath string, twiggServerKey string, mux wrappers.AuthMux) {
	h := handler{
		maxJobTimeout:          maxJobTimeout,
		jobsService:            db,
		runnersService:         runnersService,
		jobsQueue:              jobsQueue,
		webhookQueue:           webhookQueue,
		twiggWebClient:         twiggWebClient,
		twiggServerUrl:         twiggServerUrl,
		twiggServerKey:         twiggServerKey,
		twiggServerWebhookPath: twiggServerWebhookPath,
	}
	jobsQueue.Register(runJobPayloadType, h.runJob, h.getRunJobDisplayString, h.onRunJobDeadLetter)
	onSendJobDoneWebhookDeadLetter := func(p []byte) error {
		return nil
	}
	webhookQueue.Register(sendJobDoneWebhook, h.sendJobDoneWebhook, h.sendJobDoneWebhookDisplayString, onSendJobDoneWebhookDeadLetter)
	webhookQueue.Register(deprecated_sendWebhookPayloadType, h.sendJobDoneWebhook, h.sendJobDoneWebhookDisplayString, onSendJobDoneWebhookDeadLetter)

	mux.HandleFunc("PUT /job/{id}", h.putJob)
	mux.HandleFunc("GET /job/{id}", h.getJob)
	mux.HandleFunc("GET /job/{id}/out", h.getJobOut)
	mux.HandleFunc("PUT /c-job/{id}", h.cancelJob)
}

type JobsDb interface {
	Get(tx context.Context, id string) (j trackclient.TrackJob, isNotFoundErr bool, err error)
	Exists(tx context.Context, id string) (bool, error)
	GetMaxPayloadLen() int
	Create(tx context.Context, id string, payload []byte, skipWebhooks bool) (trackclient.TrackJob, error)
	SetStatus(tx context.Context, id string, st trackclient.TrackJobStatus, executionTimeMillis int64) error
	RequestBestEffortCancelation(tx context.Context, id string) error
	BestEffortCancelationWasRequested(tx context.Context, id string) (bool, error)
	BeginRead() (ctx context.Context, closeTx func(), err error)
	BeginWrite() (ctx context.Context, closeTx func(), commitTx func() error, err error)
}

type Queue interface {
	Register(payloadType string,
		handler func(payload []byte) error,
		decoder func(payload []byte) string,
		onDeadLetter func(payload []byte) error)
	Enqueue(payloadType string, payload []byte) error
}

type RunnersService interface {
	Run(jobId string, j runnerlib.JobPayload) (trackclient.TrackJobStatus, error)
	Read(jobId string) (out io.ReadCloser, isNotFoundErr bool, err error)
	Cancel(jobId string)
	Close()
}

type TwiggWebClient interface {
	GetSecretValsFromTwiggWeb(requiredSecretsNames []string, twiggToken string) (requiredSecrets map[string]string, isNotFoundOrForbiddenErr bool, err error)
}

const (
	runJobPayloadType                 = "run-job"
	deprecated_sendWebhookPayloadType = "send-wh"
	sendJobDoneWebhook                = "send-job-done-wh"
)
