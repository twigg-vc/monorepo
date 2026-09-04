package cicdqueue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/services/gobencoding"
)

func (s service) EnqueueCiCdRun(repoId, commitId, commitVersion uint64,
	trigger runnerlib.JobTrigger, w context.Context) (runNumber int64, err error) {
	runNumber, nonce, err := s.prepareCiCdRun(repoId, commitId, commitVersion, trigger, w)
	if err != nil {
		return
	}
	err = s.enqueueExecutionOfStartCiCdRun(repoId, commitId, commitVersion, runNumber, nonce)
	return
}

// Saves to the db that a cicd run will be executed, and enqueue a message to the
// queue so that the CICD is actually executed later. Note that the method that actually
// puts the jobs will check if this transaction was actually commited.
func (s service) prepareCiCdRun(repoId, commitId, commitVersion uint64,
	trigger runnerlib.JobTrigger, w context.Context) (runNumber int64, nonce string, err error) {
	nonce = newNonce()
	lastRunNumber, isNotFoundErr, err := s.db.GetCiCdQueueLastRunNumber(w, repoId, commitId, commitVersion)
	if err != nil && !isNotFoundErr {
		return
	}
	if isNotFoundErr {
		err = nil
		lastRunNumber = -1
	}
	runNumber = lastRunNumber + 1
	err = s.db.InsertCiCdQueueRun(w, repoId, commitId, commitVersion, runNumber,
		string(trigger), nonce, string(CiCdStatusPrepared))
	return
}

// Enqueues an execution of the `startAutoCiCdRun`
func (s service) enqueueExecutionOfStartCiCdRun(repoId, commitId, commitVersion uint64, runNumber int64, nonce string) error {
	payload := startCiCdRunPayload{
		RepoId:        repoId,
		CommitId:      commitId,
		CommitVersion: commitVersion,
		RunNumber:     runNumber,
		Nonce:         nonce,
	}
	return s.queue.Enqueue(QueueStartAutoCiCdRunPayloadType, gobencoding.Encode(payload))
}
func (s service) GetCiCdLatestRunStatus(repoId, commitId, commitVersion uint64, r context.Context) (CiCdStatus, error) {
	st, isNotFoundErr, err := s.db.GetCiCdQueueLatestRunStatus(r, repoId, commitId, commitVersion)
	if err != nil && !isNotFoundErr {
		return "", err
	}
	if isNotFoundErr {
		return CiCdStatusNone, nil
	}
	return CiCdStatus(st), nil
}

func (s service) startCiCdRunPayloadDisplayString(b []byte) string {
	payload, err := gobencoding.Decode[startCiCdRunPayload](b)
	if err != nil {
		return "bad start cicd payload: " + string(b)
	}
	return fmt.Sprintf("start CICD: repo=%d c/%dv%d run=%d",
		payload.RepoId, payload.CommitId, payload.CommitVersion,
		payload.RunNumber)
}

func (s service) startAutoCiCdRun(b []byte) error {
	payload, err := gobencoding.Decode[startCiCdRunPayload](b)
	if err != nil {
		return errors.New("invalid payload")
	}
	w, closeW, commit, err := s.db.BeginWrite()
	defer closeW()
	if err != nil {
		return err
	}
	// Check if we can actually start the analysis
	trigger, st, isNotFoundErr, err := s.db.GetCiCdQueueRunTriggerAndStatus(w,
		payload.RepoId, payload.CommitId, payload.CommitVersion,
		payload.RunNumber, payload.Nonce)
	if err != nil && !isNotFoundErr {
		return err
	}
	if isNotFoundErr {
		return fmt.Errorf("analysis not found")
	}
	if st == string(CiCdStatusStarted) {
		// Idempotent skip.
		return nil
	}
	if st != string(CiCdStatusPrepared) {
		return fmt.Errorf("unexpected status: %s", st)
	}

	err = s.ciCdPublisher.PutAutoCiCdRun(payload.RepoId, payload.CommitId,
		payload.CommitVersion, payload.RunNumber,
		runnerlib.JobTrigger(trigger), w)
	if err != nil {
		return err
	}

	// Mark the analysis as complete and commit the tx
	err = s.db.SetCiCdQueueRunStatus(w,
		payload.RepoId, payload.CommitId, payload.CommitVersion,
		payload.RunNumber, payload.Nonce, string(CiCdStatusStarted))
	if err != nil {
		return err
	}
	err = commit()
	if err != nil {
		return err
	}
	return nil
}

type startCiCdRunPayload struct {
	RepoId        uint64
	CommitId      uint64
	CommitVersion uint64
	RunNumber     int64
	Nonce         string
}

type resumeCdPayload struct {
	PipelineId string
	Stage      int32
}

func (s service) resumeCdPayloadDisplayString(b []byte) string {
	payload, err := gobencoding.Decode[resumeCdPayload](b)
	if err != nil {
		return "bad run CD stage payload: " + string(b)
	}
	return fmt.Sprintf("run pipelineId=%q stage %d",
		payload.PipelineId, payload.Stage)
}

func (s service) ResumeCdToStage(pipelineId string, stage int32) error {
	payload := resumeCdPayload{
		PipelineId: pipelineId,
		Stage:      stage,
	}
	return s.queue.Enqueue(QueueResumeCdPayloadType, gobencoding.Encode(payload))
}

func (s service) resumeCd(b []byte) error {
	payload, err := gobencoding.Decode[resumeCdPayload](b)
	if err != nil {
		return errors.New("invalid payload")
	}
	w, closeW, commit, err := s.db.BeginWrite()
	defer closeW()
	if err != nil {
		return err
	}
	// Verify that the stage can start
	canStart, err := s.js.CanPutResumePipelineToStage(
		w, payload.PipelineId, payload.Stage)
	if err != nil {
		return err
	}
	if !canStart {
		return fmt.Errorf("stage %d not allowed to start", payload.Stage)
	}
	// Resumt at the specified stage
	err = s.ciCdPublisher.PutResumePipelineWaitingStage(payload.PipelineId, payload.Stage, w)
	if err != nil {
		return err
	}
	return commit()
}

func newNonce() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Sprintf("rand.Read err: %s", err))
	}
	return hex.EncodeToString(b)
}