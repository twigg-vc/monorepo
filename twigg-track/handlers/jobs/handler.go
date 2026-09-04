package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-track/wrappers"
	"net/http"
	"slices"
	"time"
)

type handler struct {
	maxJobTimeout          time.Duration
	jobsService            JobsDb
	runnersService         RunnersService
	jobsQueue              Queue
	webhookQueue           Queue
	twiggWebClient         TwiggWebClient
	twiggServerUrl         string
	twiggServerKey         string
	twiggServerWebhookPath string
}

const (
	idPathParamName = "id"
)

func (h handler) cancelJob(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	jobId := r.PathValue(idPathParamName)
	if jobId == "" {
		http.Error(w, "no jobId", http.StatusBadRequest)
		return
	}
	// Cancel if running
	h.runnersService.Cancel(jobId)

	// Request a best effort cancel
	tx, unlockTx, commitTx, err := h.jobsService.BeginWrite()
	if err != nil {
		log.Printf("failed to get tx to request cancel: %s", err)
		http.Error(w, "failed to get tx", http.StatusInternalServerError)
		return
	}
	defer unlockTx()
	err = h.jobsService.RequestBestEffortCancelation(tx, jobId)
	if err != nil {
		log.Printf("failed to request cancel: %s", err)
		http.Error(w, "failed to request cancel", http.StatusInternalServerError)
		return
	}
	err = commitTx()
	if err != nil {
		log.Printf("failed to commit request cancel: %s", err)
		http.Error(w, "failed to commit cancel", http.StatusInternalServerError)
		return
	}
}

func (h handler) putJob(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	jobId := r.PathValue(idPathParamName)
	if jobId == "" {
		http.Error(w, "no jobId", http.StatusBadRequest)
		return
	}
	tx, unlockTx, commitTx, err := h.jobsService.BeginWrite()
	if err != nil {
		log.Printf("failed to begin tx: %s", err)
		http.Error(w, "failed to begin tx", http.StatusInternalServerError)
		return
	}
	defer unlockTx()

	exists, err := h.jobsService.Exists(tx, jobId)
	if err != nil {
		log.Printf("failed to check if job exists: %s", err)
		http.Error(w, "failed to check existence", http.StatusInternalServerError)
		return
	}
	if exists {
		// Idempotent skip
		return
	}
	skipWebhooks := false
	if r.Header.Get(trackclient.SkipWehbooksHeaderName) != "" {
		skipWebhooks = true
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(h.jobsService.GetMaxPayloadLen()))
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "too large payload", http.StatusRequestEntityTooLarge)
		return
	}

	var jobPayload runnerlib.JobPayload
	err = json.Unmarshal(bodyBytes, &jobPayload)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if jobPayload.TimeoutMilliSeconds > h.maxJobTimeout.Milliseconds() {
		http.Error(w, "timeout too big", http.StatusBadRequest)
		return
	}
	_, err = h.jobsService.Create(tx, jobId, bodyBytes, skipWebhooks)
	if err != nil {
		log.Printf("failed to create job: %s", err)
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}
	err = h.jobsQueue.Enqueue(runJobPayloadType, []byte(jobId))
	if err != nil {
		log.Printf("failed to enqueue job: %s", err)
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}
	err = commitTx()
	if err != nil {
		log.Printf("failed to commit tx: %s", err)
		http.Error(w, "failed to commit tx", http.StatusInternalServerError)
		return
	}
}

func (h handler) getJob(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	tx, unlockTx, err := h.jobsService.BeginRead()
	if err != nil {
		log.Printf("failed to begin tx: %s", err)
		http.Error(w, "failed to start tx", http.StatusInternalServerError)
		return
	}
	defer unlockTx()

	j, isNotFoudErr, err := h.jobsService.Get(tx, r.PathValue(idPathParamName))
	if isNotFoudErr {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("failed to get job: %s", err)
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(j)
	if err != nil {
		log.Printf("failed to write job: %s", err)
		http.Error(w, "failed to write job", http.StatusInternalServerError)
		return
	}
}
func (h handler) getJobOut(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
	jobId := r.PathValue(idPathParamName)
	out, isNotFoundErr, err := h.runnersService.Read(jobId)
	if isNotFoundErr {
		w.Write([]byte("job not found/hasn't started"))
		return
	}
	if err != nil {
		log.Printf("failed to Read jobId=%s output: %s", jobId, err)
		http.Error(w, "failed Read", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(w, out)
}

func (h handler) getRunJobDisplayString(b []byte) string {
	return fmt.Sprintf("run jobId=%s", b)
}

// This handler runs again if it returns any error (as its run by the queue)
// This handler can't hold transactions for long because it can run for long
// as it blocks while running the job.
// Thus, it must get tx, use it, and unlock it.
// This is fine because it assumes that the queue only passes this
// payload to one runner at a time - else; we could get problems
// for not using transactions
func (h handler) runJob(payload []byte) error {
	jobId := string(payload)
	readTx, unlockReadTx, err := h.jobsService.BeginRead()
	if err != nil {
		log.Printf("failed to begin read tx: %s", err)
		return err
	}
	defer unlockReadTx()

	// Get the job and the payload
	job, isNotFoudErr, err := h.jobsService.Get(readTx, jobId)
	if isNotFoudErr {
		log.Printf("queue did not find job %s", err)
		return err
	}
	if job.Status != trackclient.TrackJobStatusQueued {
		panic(fmt.Sprintf("unexpected queued status %q", job.Status))
	}
	cancelRequested, err := h.jobsService.BestEffortCancelationWasRequested(readTx, jobId)
	if err != nil {
		return fmt.Errorf("failed to get cancelReq: %s", err)
	}
	// It's safe to release this transaction and pick up another one later
	// because the queue doesn't forward the same payload to more than one
	// runner; i.e. we're sure that no other routine will try to change
	// the status of this job in the db
	unlockReadTx()

	var st trackclient.TrackJobStatus
	var executionTimeMillis int64
	if cancelRequested {
		st = trackclient.TrackJobStatusCancel
		executionTimeMillis = 0
	} else {
		// Notify that the job started to run
		runStartedNotificationSent := false
		const maxRetries = 5
		jobCopy := job
		jobCopy.Status = trackclient.TrackJobStatusRunning
		for i := 0; i < maxRetries; i++ {
			err = sendWebhookIfNotSkip(jobCopy, h.twiggServerUrl, h.twiggServerWebhookPath, h.twiggServerKey)
			if err != nil {
				time.Sleep(time.Duration(i*200+1) * time.Millisecond)
			} else {
				runStartedNotificationSent = true
				break
			}
		}
		if !runStartedNotificationSent {
			return fmt.Errorf("failed to send job-started wh for jobId=%q",
				jobCopy.Id)
		}

		// Get the job payload and add the necessary secrets to it
		var jobPayload runnerlib.JobPayload
		err = json.Unmarshal(job.Payload, &jobPayload)
		if err != nil {
			return fmt.Errorf("unsuported payload for jobId %s: %s",
				jobId, job.Payload)
		}
		uniqueRequiredSecretsNames := map[string]bool{}
		for i := range jobPayload.Steps {
			for j := range jobPayload.Steps[i].Secrets {
				uniqueRequiredSecretsNames[jobPayload.Steps[i].Secrets[j]] = true
			}
		}
		if len(uniqueRequiredSecretsNames) > 0 {
			// Put the unique secrets on a list to only ask the twigg-web once
			requiredSecretsNames := make([]string, 0, len(uniqueRequiredSecretsNames))
			for secret := range uniqueRequiredSecretsNames {
				requiredSecretsNames = append(requiredSecretsNames, secret)
			}
			requiredSecrets, isNotFoundOrForbiddenErr, err := h.twiggWebClient.GetSecretValsFromTwiggWeb(
				requiredSecretsNames, jobPayload.Token,
			)
			if err != nil && !isNotFoundOrForbiddenErr {
				return fmt.Errorf("failed to get secrets for %s: %s, err=%q", jobId, err, err)
			}
			if requiredSecrets == nil { // ensure map is not nil
				requiredSecrets = map[string]string{}
			}
			// Populate the steps env with the provided secrets
			for stepI := range jobPayload.Steps {
				for _, secret := range jobPayload.Steps[stepI].Secrets {
					jobPayload.Steps[stepI].Env[secret] = requiredSecrets[secret]
				}
			}
		}
		// Run the jobPayload
		start := time.Now()
		st, err = h.runnersService.Run(jobId, jobPayload)
		if err != nil {
			return fmt.Errorf("failed to run jobId %s: %s", jobId, err)
		}
		executionTimeMillis = time.Since(start).Milliseconds()
	}

	// Update the status
	writeTx, unlockwriteTx, commitTx, err := h.jobsService.BeginWrite()
	if err != nil {
		log.Printf("failed to begin read tx: %s", err)
		return err
	}
	defer unlockwriteTx()
	err = h.jobsService.SetStatus(writeTx, jobId, st, executionTimeMillis)
	if err != nil {
		log.Printf("failed to setStatus of id %s: %s", jobId, err)
		return err
	}
	// Enqueue to nofify that the job ran ok. Note that this must be done
	// before commiting the tx, to ensure it'll be handler
	if h.twiggServerUrl != "" {
		err = h.webhookQueue.Enqueue(sendJobDoneWebhook, []byte(jobId))
		if err != nil {
			log.Printf("failed to enqueue to notify jobId %s done: %s", jobId, err)
			return err
		}
	}
	err = commitTx()
	if err != nil {
		log.Printf("failed to commit tx: %s", err)
		return err
	}
	return nil
}

func (h handler) sendJobDoneWebhookDisplayString(b []byte) string {
	return fmt.Sprintf("send 'done' webhook for jobId=%s", b)
}

// onRunJobDeadLetter is called when the queue gives up running the job
// and moves it to the deadletter. We don't expect this to happen often, but if
// it does (for example if docker is constantly blowing up or something), this
// will recover things
func (h handler) onRunJobDeadLetter(payload []byte) error {
	jobId := string(payload)
	writeTx, unlockWriteTx, commitTx, err := h.jobsService.BeginWrite()
	if err != nil {
		log.Printf("failed to begin write tx: %s", err)
		return err
	}
	defer unlockWriteTx()
	err = h.jobsService.SetStatus(writeTx, jobId, trackclient.TrackJobStatusCancel, 0)
	if err != nil {
		log.Printf("failed to set status to canceled: %s", err)
		return err
	}
	if h.twiggServerUrl != "" {
		err = h.webhookQueue.Enqueue(sendJobDoneWebhook, []byte(jobId))
		if err != nil {
			log.Printf("failed to enqueue to notify jobId %s deadletter: %s", jobId, err)
			return err
		}
	}
	err = commitTx()
	if err != nil {
		log.Printf("failed to commit tx: %s", err)
		return err
	}
	return nil
}

func (h handler) sendJobDoneWebhook(payload []byte) error {
	tx, unlockTx, err := h.jobsService.BeginRead()
	if err != nil {
		log.Printf("failed to begin tx: %s", err)
		return err
	}
	defer unlockTx()

	j, _, err := h.jobsService.Get(tx, string(payload))
	if err != nil {
		log.Printf("failed to get job of id %s: %s", string(payload), err)
		return err
	}
	if !slices.Contains([]trackclient.TrackJobStatus{
		trackclient.TrackJobStatusSuccess,
		trackclient.TrackJobStatusFail,
		trackclient.TrackJobStatusTimeout,
		trackclient.TrackJobStatusCancel,
	}, j.Status) {
		return fmt.Errorf("jobId=%q not yet done", j.Id)
	}
	return sendWebhookIfNotSkip(j, h.twiggServerUrl, h.twiggServerWebhookPath, h.twiggServerKey)
}
func sendWebhookIfNotSkip(j trackclient.TrackJob, twiggServerUrl, twiggServerWebhookPath, twiggServerKey string) error {
	if j.SkipWebhooks {
		return nil
	}
	jBytes, err := json.Marshal(j)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal trackjob: %s", err))
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s%s", twiggServerUrl,
		twiggServerWebhookPath),
		bytes.NewBuffer(jBytes))
	if err != nil {
		log.Printf("failed to create req")
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("TwiggServerKey", twiggServerKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("failed to put")
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got non-ok status code: %d", resp.StatusCode)
	}
	return nil
}
