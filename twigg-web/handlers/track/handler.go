package track

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"monorepo/base/iterator"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/twiggtoken"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"slices"
)

type handler struct {
	js       JobsStorage
	db       webdb.WebDb
	tq       TrackQueue
	trackObs TrackObserver
	secrets  Secrets
	cdQueue  CdQueue
	whParser webhookParser
}

func (h handler) handleTrackWebhook(w http.ResponseWriter, r wrappers.ServerKeyAuthTrackMuxRequest) {
	defer r.Body.Close()
	if h.whParser == nil {
		h.whParser = defaultWebhookParser{}
	}
	trackJob, jobPayload, err := h.whParser.ParseWebhook(r.Body)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var incomingJobStatus job.JobStatus
	switch trackJob.Status {
	case trackclient.TrackJobStatusSuccess:
		incomingJobStatus = job.JobStatusSuccess
	case trackclient.TrackJobStatusFail:
		incomingJobStatus = job.JobStatusFail
	case trackclient.TrackJobStatusTimeout:
		incomingJobStatus = job.JobStatusTimeout
	case trackclient.TrackJobStatusRunning:
		incomingJobStatus = job.JobStatusRunning
	case trackclient.TrackJobStatusCancel:
		incomingJobStatus = job.JobStatusCanceled
	default:
		log.Printf("[track webhook] got invalid job.Status=%q. job.id=%v", incomingJobStatus, trackJob.Id)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ok := false
	if job.MightBePipelineStageId(trackJob.Id) {
		ok = h.handlePipelineStageWebhook(w, incomingJobStatus, trackJob.Id)
	} else {
		ok = h.handleNonPipelineWebhook(w, incomingJobStatus, trackJob.Id)
	}
	if ok && h.trackObs != nil {
		h.trackObs.OnTrackWebhookReceived(trackJob, jobPayload)
	}
}

// Statuses that a job can be once it's done running in the track server
var finishedStatuses = []job.JobStatus{
	job.JobStatusSuccess,
	job.JobStatusFail,
	job.JobStatusTimeout,
	job.JobStatusCanceled}

// Handles a webhook regarding a "single job" (i.e. not of a pipeline)
func (h handler) handleNonPipelineWebhook(w http.ResponseWriter, incomingJobStatus job.JobStatus, jobId string) (handledOk bool) {
	wl, ul, commit, err := h.db.BeginWrite()
	defer ul()
	if err != nil {
		log.Printf("[track webhook] could not get wtx for jobId=%v", jobId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	localJob, err := h.js.GetJobById(wl, jobId)
	if err != nil {
		log.Printf("failed to get local job: %s", err)
		http.Error(w, "failed to get internal job", http.StatusBadRequest)
		return
	}
	// Ignore the webhook if it has the same status as the local job
	if localJob.Status == incomingJobStatus {
		// Idempontent skip
		handledOk = true
		return
	}

	// There are 2 kinds of expected updates:
	// from posted -> running
	// from running -> success/fail/etc (i.e. some finished state)
	isPostedToRunning := false
	isRunningToFinished := false
	switch localJob.Status {
	// Locally the job has `posted` status
	case job.JobStatusPosted:
		isPostedToRunning = incomingJobStatus == job.JobStatusRunning
	// Locally the job has `running` status
	case job.JobStatusRunning:
		isRunningToFinished = slices.Contains(finishedStatuses, incomingJobStatus)
	default:
	}
	if !isPostedToRunning && !isRunningToFinished {
		log.Printf("[track webhook] got unexpected job.Status=%q. jobId=%v", incomingJobStatus, jobId)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err = h.js.SetJobStatus(wl, jobId, incomingJobStatus)
	if err != nil {
		log.Printf("[track webhook] could not SetJobStatus(). jobId=%v, jobStatus=%q, err=%s", jobId, incomingJobStatus, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if isRunningToFinished {
		err = h.tq.PutJobFinished(jobId, wl)
		if err != nil {
			log.Printf("[track webhook] could not PutJobFinished(). jobId=%v, err=%s", jobId, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	err = commit()
	if err != nil {
		log.Printf("[track webhook] could not commit wtx. jobId=%v, jobStatus=%q", jobId, incomingJobStatus)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	handledOk = true
	return
}

// Handles a webhook regarding a stage of a pipeline
func (h handler) handlePipelineStageWebhook(w http.ResponseWriter, incomingStageStatus job.JobStatus, stageId string) (handledOk bool) {
	wl, ul, commit, err := h.db.BeginWrite()
	defer ul()
	if err != nil {
		log.Printf("[track webhook] could not get wtx for stageId=%v", stageId)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Get all the stages of the pipeline
	RepoId, Commit, CommitVersion,
		Path, Name, RunNumber, Stage, stageIdIsOk := job.ParsePipelineStageId(stageId)
	if !stageIdIsOk {
		log.Printf("bad stageId: %s", stageId)
		http.Error(w, "bad stageId", http.StatusBadRequest)
		return
	}
	incomingStage := int64(Stage)
	pipelineId := job.PipelineId(RepoId, Commit, CommitVersion, Path, Name, RunNumber)
	stagesIter, err := h.js.GetPipelineStagesById(wl, pipelineId)
	if err != nil {
		log.Printf("failed to get stages: %s", err)
		http.Error(w, "failed to get stages", http.StatusBadRequest)
		return
	}
	const maxExpectedStages = 100
	stages, err := iterator.GetFirstN(maxExpectedStages+1, stagesIter)
	if err != nil {
		log.Printf("failed to iterate on stages: %s", err)
		http.Error(w, "failed to iterate on stages", http.StatusBadRequest)
		return
	}
	if len(stages) == maxExpectedStages+1 {
		log.Printf("got too many stages: %d (expected %d)", len(stages), maxExpectedStages)
		http.Error(w, "got too many stages", http.StatusBadRequest)
		return
	}
	if incomingStage < 0 || incomingStage >= int64(len(stages)) {
		log.Printf("got bad stage: %d (pipeline has %d)", incomingStage, len(stages))
		http.Error(w, "bad Stage", http.StatusBadRequest)
		return
	}
	// Ignore the webhook if it has the same status as the local stage
	localStage := stages[incomingStage]
	if localStage.Status == incomingStageStatus {
		handledOk = true // Idempontent skip
		return
	}

	// Webhooks should only update stages "from->to":
	// posted -> running
	// running -> success/fail/timeout/cancel
	isPostedToRunning := false
	isRunningToFinished := false
	if localStage.Status == job.JobStatusPosted &&
		incomingStageStatus == job.JobStatusRunning {
		isPostedToRunning = true
	}
	if (localStage.Status == job.JobStatusRunning) &&
		slices.Contains(
			finishedStatuses, incomingStageStatus) {
		isRunningToFinished = true
	}
	if !isPostedToRunning && !isRunningToFinished {
		log.Printf("unexpected event. stage=%q is %q and got %q ",
			stageId, localStage.Status, incomingStageStatus)
		http.Error(w, "unexpected event", http.StatusBadRequest)
		return
	}
	// Get the current stage that is posted/running. We do this to ensure we
	// process the webhooks in the correct order: we expect to first
	// receive a webhook for stage0, then to stage1, etc. If they arrive
	// out of order, we return an error so that the sender can send the webhook
	// later on.
	currentRunningOrPostedStage := int64(-1)
	for i := int64(0); i < int64(len(stages)); i++ {
		if stages[i].Status == job.JobStatusRunning ||
			stages[i].Status == job.JobStatusPosted {
			currentRunningOrPostedStage = int64(i)
			break
		}
	}
	if incomingStage > currentRunningOrPostedStage {
		http.Error(w, "got stage > current stage", http.StatusTooEarly)
		return
	}
	if incomingStage < currentRunningOrPostedStage {
		http.Error(w, "got stage < current stage", http.StatusTooEarly)
		return
	}
	if incomingStage != currentRunningOrPostedStage {
		panic("I'm bad at math")
	}

	// Update the stage
	err = h.js.SetStatusOfPipelineStage(wl, pipelineId, Stage, incomingStageStatus)
	if err != nil {
		log.Printf("failed to update stage: %s", err)
		http.Error(w, "failed to update stage", http.StatusInternalServerError)
		return
	}
	// If it's just a posted->running webhook, just commit.
	// Soon there will be another webhook informing its ultimate status
	if isPostedToRunning {
		err = commit()
		if err != nil {
			log.Printf("failed commit tx: %s", err)
			http.Error(w, "failed commit tx", http.StatusInternalServerError)
		}
		handledOk = true
		return
	}
	// Mark the job as finished in the trackqueue
	err = h.tq.PutJobFinished(stageId, wl)
	if err != nil {
		log.Printf("failed to update stage: %s", err)
		http.Error(w, "failed to update stage", http.StatusInternalServerError)
		return
	}
	// Enqueue the execution of the next stage
	isLastStage := incomingStage == int64(len(stages)-1)
	isCanceled := incomingStageStatus == job.JobStatusCanceled
	if !isLastStage && !isCanceled {
		err = h.cdQueue.ResumeCdToStage(pipelineId, Stage+1)
		if err != nil {
			log.Printf("failed to enqueue next stage: %s", err)
			http.Error(w, "failed to enqueue next stage", http.StatusInternalServerError)
			return
		}
	}
	// Note that this commit might fail, so the EnqueueCdStage method must
	// check that the previous stage succeeded
	err = commit()
	if err != nil {
		log.Printf("failed commit tx: %s", err)
		http.Error(w, "failed commit tx", http.StatusInternalServerError)
	}
	handledOk = true
	return
}

func (h handler) handleGetSecrets(w http.ResponseWriter, r wrappers.ServerKeyAndTokenAuthTrackMuxRequest) {
	defer r.Body.Close()

	repoSecretsName := r.URL.Query()[routes.RepoSecretNameParamName]
	if len(repoSecretsName) == 0 {
		http.Error(w, "invalid request, no secrets name were passed", http.StatusBadRequest)
		return
	}
	if len(repoSecretsName) > 100 {
		http.Error(w, "invalid request, too many secrets name", http.StatusBadRequest)
		return
	}

	secrets := make(map[string]string, len(repoSecretsName))

	rl, ul, err := h.db.BeginRead()
	if err != nil {
		log.Printf("[track webhook] could not get rl for secrets name: %q", repoSecretsName)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer ul()

	for _, repoSecretName := range repoSecretsName {
		if r.TwiggToken.Supports(twiggtoken.TokenActionGetSecret, repoSecretName) {
			secret, isNotFoundErr, err := h.secrets.GetRepoIdSecret(rl,
				r.TwiggToken.RepoId,
				repoSecretName,
			)
			if isNotFoundErr {
				http.Error(w, fmt.Sprintf("%q not found", repoSecretName), http.StatusNotFound)
				return
			}
			if err != nil {
				log.Printf("[track webhook] could not GetRepoIdSecret for secret name: %q and repoId: %d", repoSecretName, r.TwiggToken.RepoId)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			secrets[repoSecretName] = secret
		} else {
			log.Printf("[track webhook] action TokenActionGetSecret not supported for secret name: %q", repoSecretName)
			http.Error(w, "invalid request, not supported", http.StatusForbidden)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(secrets)
	if err != nil {
		log.Printf("[track webhook] could not encode secrets: %v", err)
	}
}

type webhookParser interface {
	ParseWebhook(r io.Reader) (trackclient.TrackJob, runnerlib.JobPayload, error)
}

type defaultWebhookParser struct{}

func (d defaultWebhookParser) ParseWebhook(r io.Reader) (trackclient.TrackJob, runnerlib.JobPayload, error) {
	return trackclient.ParseWebhook(r)
}
