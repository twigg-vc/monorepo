package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/webdb"
	"time"
)

type service struct {
	db webdb.WebDb
}

func newService(db webdb.WebDb) service {
	return service{db: db}
}

func (s service) CiCdRunWasPublished(tx context.Context,
	repoId uint64, commit uint64, commitV uint64, runNumber int64) (bool, error) {
	return s.db.CiCdRunExists(tx, repoId, commit, commitV, runNumber)
}

func (s service) SetCiCdToPublished(tx context.Context,
	repoId uint64, commit uint64, commitV uint64, runNumber int64) error {
	return s.db.InsertCiCdRun(tx, repoId, commit, commitV, runNumber, newNonce())
}

func newNonce() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Sprintf("rand.Read err: %s", err))
	}
	return hex.EncodeToString(b)
}

func (s service) CreateNewJob(wl context.Context,
	repoId uint64, commit uint64, commitV uint64,
	filePath string, jobName string, runNumber int64, initialStatus job.JobStatus) (job.Job, error) {
	j := job.Job{
		RepoId:        repoId,
		Commit:        commit,
		CommitVersion: commitV,
		Path:          filePath,
		Name:          jobName,
		RunNumber:     runNumber,
		Status:        initialStatus,
		CreatedTime:   time.Now().UTC().Format(time.RFC3339),
	}
	taken, err := s.db.JobExists(wl, repoId, commit, commitV, filePath, jobName, runNumber)
	if err != nil {
		return job.Job{}, err
	}
	if taken {
		return job.Job{}, errors.New("runNumber already taken")
	}
	j.InternalId, err = s.db.InsertJob(wl, j)
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to CreateJob: %s", err)
	}
	return j, nil
}

func (s service) GetJobById(rl context.Context, id string) (j job.Job, err error) {
	repoId, commit, commitV, path, name, runNumber, idIsOk := job.ParseJobId(id)
	if !idIsOk {
		err = fmt.Errorf("bad job id %s", id)
		return
	}
	j, isNotFoundErr, err := s.db.GetJob(rl, repoId, commit, commitV, path, name, runNumber)
	if isNotFoundErr {
		return j, fmt.Errorf("GetJobById: job %s not found: %w", id, err)
	}
	if err != nil {
		return j, fmt.Errorf("GetJobById: failed to scan row: %w", err)
	}
	return j, nil
}
func (s service) SetJobStatus(wl context.Context, id string, status job.JobStatus) (err error) {
	repoId, commit, commitV, path, name, runNumber, idIsOk := job.ParseJobId(id)
	if !idIsOk {
		err = fmt.Errorf("bad job id %s", id)
		return
	}
	isNotFoundErr, err := s.db.SetJobStatus(wl, repoId, commit, commitV, path, name,
		runNumber, status)
	if isNotFoundErr {
		return fmt.Errorf("SetJobStatus: job %s not found", id)
	}
	if err != nil {
		return fmt.Errorf("SetJobStatus: failed to update job status: %w", err)
	}
	return nil
}
func (s service) GetCommitJobs(
	rl context.Context,
	repoId uint64,
	commit uint64,
	afterInternalJobId int64,
) (iterator.I[job.Job], error) {
	return s.db.GetCommitJobs(rl, repoId, commit, afterInternalJobId)
}
func (s service) GetRepoJobs(
	rl context.Context,
	repoId uint64,
	afterInternalJobId int64,
) (iterator.I[job.Job], error) {
	return s.db.GetRepoJobs(rl, repoId, afterInternalJobId)
}

func (s service) CreateNewPipeline(tx context.Context,
	repoId uint64, commit uint64, commitV uint64,
	filePath string, jobName string, runNumber int64,
	stageNames []string, isCreatedByUser bool, createdByUserId int64) (job.Pipeline, error) {
	if len(stageNames) == 0 {
		return job.Pipeline{}, fmt.Errorf("cant create Pipeline with no stages")
	}
	if !isCreatedByUser {
		createdByUserId = 0
	}
	const initialStatus = job.PipelineStatusRunning
	createdTime := time.Now().UTC().Format(time.RFC3339)
	j := job.Pipeline{
		RepoId:          repoId,
		Commit:          commit,
		CommitVersion:   commitV,
		Path:            filePath,
		Name:            jobName,
		RunNumber:       runNumber,
		NumberOfStages:  int32(len(stageNames)),
		Status:          initialStatus,
		CreatedTime:     createdTime,
		IsCreatedByUser: isCreatedByUser,
		CreatedByUserId: createdByUserId,
	}
	taken, err := s.db.PipelineExists(tx, repoId, commit, commitV, filePath, jobName, runNumber)
	if err != nil {
		return job.Pipeline{}, err
	}
	if taken {
		return job.Pipeline{}, errors.New("runNumber already taken")
	}
	j.InternalId, err = s.db.InsertPipeline(tx, j)
	if err != nil {
		return job.Pipeline{}, fmt.Errorf("failed to CreateNewPipeline: %s", err)
	}
	_, err = s.PutPipelineRef(tx, j.RepoId, j.Path, j.Name)
	if err != nil {
		return job.Pipeline{}, err
	}

	// Create all the stages in waiting status
	for i := range stageNames {
		stage := job.PipelineStage{
			PipelineId:  j.Id(),
			Stage:       int32(i),
			Name:        stageNames[i],
			Status:      job.JobStatusWaiting,
			CreatedTime: createdTime,
		}
		err = s.db.InsertPipelineStage(tx, stage.PipelineId, stage.Stage, stage.Name,
			stage.CreatedTime, stage.Status)
		if err != nil {
			return job.Pipeline{}, err
		}
	}

	return j, nil
}
func (s service) GetPipelineById(rl context.Context, id string) (job.Pipeline, error) {
	p, _, err := s.getPipelineById(rl, id)
	return p, err
}
func (s service) getPipelineById(rl context.Context, id string) (p job.Pipeline, isNotFoundErr bool, err error) {
	repoId, commitId, commitVersion,
		path, name, runNumber, ok := job.ParsePipelineId(id)
	if !ok {
		return job.Pipeline{}, false, fmt.Errorf("bad PipelineId: %q", id)
	}
	return s.db.GetPipeline(rl, repoId, commitId, commitVersion, path, name, runNumber)
}
func (s service) GetPipelineStagesById(rl context.Context, id string) (iterator.I[job.PipelineStage], error) {
	return s.db.GetPipelineStages(rl, id)
}

func (s service) SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status job.JobStatus) error {
	pipeline, isNotFoundErr, err := s.getPipelineById(tx, pipelineId)
	if isNotFoundErr {
		return fmt.Errorf("pipelineId=%s not found", pipelineId)
	}
	if err != nil {
		return err
	}
	stageIsNotFoundErr, err := s.db.SetPipelineStageStatus(tx, pipelineId, stage, status)
	if stageIsNotFoundErr {
		return fmt.Errorf("jobPipelineId=%q stage=%d not found", pipelineId, stage)
	}
	if err != nil {
		return err
	}

	return s.updatePipelineStatus(tx, pipeline)
}

func (s service) SetResumerOfPipelineStage(tx context.Context, pipelineId string, stage int32, userId int64) error {
	isNotFoundErr, err := s.db.SetPipelineStageResumer(tx, pipelineId, stage, userId)
	if isNotFoundErr {
		return fmt.Errorf("jobPipelineId=%q stage=%d not found", pipelineId, stage)
	}
	return err
}

// Reads all the stages and sets the pipeline status based on the stages.
func (s service) updatePipelineStatus(tx context.Context, pipe job.Pipeline) error {
	var currentStage job.PipelineStage
	stages, err := s.GetPipelineStagesById(tx, pipe.Id())
	if err != nil {
		return err
	}
	hasStage := false
	for stages.Next() {
		hasStage = true
		currentStage, err = stages.Get()
		if err != nil {
			return err
		}
		if currentStage.Status != job.JobStatusSuccess {
			break
		}
	}
	err = stages.Err()
	if err != nil {
		return err
	}
	if !hasStage {
		return nil
	}

	var pipelineStatus job.PipelineStatus
	switch currentStage.Status {
	case job.JobStatusWaitingManualStart:
		pipelineStatus = job.PipelineStatusWaitingManualStart
	case job.JobStatusWaiting:
		pipelineStatus = job.PipelineStatusRunning
	case job.JobStatusQueued:
		pipelineStatus = job.PipelineStatusRunning
	case job.JobStatusPosted:
		pipelineStatus = job.PipelineStatusRunning
	case job.JobStatusRunning:
		pipelineStatus = job.PipelineStatusRunning
	case job.JobStatusSuccess:
		pipelineStatus = job.PipelineStatusSuccess
	case job.JobStatusFail:
		pipelineStatus = job.PipelineStatusFail
	case job.JobStatusTimeout:
		pipelineStatus = job.PipelineStatusFail
	case job.JobStatusCanceled:
		pipelineStatus = job.PipelineStatusCancel
	case job.JobStatusTooManyJobs:
		pipelineStatus = job.PipelineStatusFail
	case job.JobStatusBadFileFormat:
		pipelineStatus = job.PipelineStatusFail
	case job.JobStatusBadFileSize:
		pipelineStatus = job.PipelineStatusFail
	case job.JobStatusExceedsPlanLimits:
		pipelineStatus = job.PipelineStatusFail
	default:
		panic(fmt.Sprintf("unexpected status: %s", currentStage.Status))
	}

	// Update the table if needed, else just return
	if pipelineStatus == pipe.Status {
		return nil
	}
	return s.db.SetPipelineStatus(tx, pipe.RepoId, pipe.Commit, pipe.CommitVersion,
		pipe.Path, pipe.Name, pipe.RunNumber, pipelineStatus)
}

func (s service) PutPipelineRef(tx context.Context,
	repoId uint64, filePath string, jobName string) (job.PipelineRef, error) {
	err := s.db.PutPipelineRef(tx, repoId, filePath, jobName)
	if err != nil {
		return job.PipelineRef{}, fmt.Errorf("failed to PutPipelineRef : %s", err)
	}
	return job.PipelineRef{
		RepoId: repoId,
		Path:   filePath,
		Name:   jobName,
	}, nil
}

func (s service) ArchivePipelineRefIfExists(tx context.Context,
	repoId uint64, filePath string, jobName string) error {
	return s.db.ArchivePipelineRef(tx, repoId, filePath, jobName)
}

func (s service) GetRepoPipelineRefs(tx context.Context,
	repoId uint64, afterPath string, afterJobName string) (iterator.I[job.PipelineRef], error) {
	return s.db.GetRepoPipelineRefs(tx, repoId, afterPath, afterJobName)
}

func (s service) GetRepoPipelinesByRef(tx context.Context,
	repoId uint64, filePath string, jobName string, afterInternalJobId int64) (iterator.I[job.Pipeline], error) {
	return s.db.GetRepoPipelinesByRef(tx, repoId, filePath, jobName, afterInternalJobId)
}
