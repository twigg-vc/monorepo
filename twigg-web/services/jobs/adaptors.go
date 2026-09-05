package jobs

import (
	"context"
	"fmt"
	"monorepo/twigg-web/job"
)

func (s service) SetToPosted(wl context.Context, jobIdOrPipelineStageId string) error {
	if job.MightBePipelineStageId(jobIdOrPipelineStageId) {
		repoId, commitId, commitV, path, name, runNumber, stage, ok := job.ParsePipelineStageId(jobIdOrPipelineStageId)
		if !ok {
			return fmt.Errorf("bad PipelineStageId: %s", jobIdOrPipelineStageId)
		}
		return s.SetStatusOfPipelineStage(wl,
			job.PipelineId(repoId, commitId, commitV, path, name, runNumber),
			stage, job.JobStatusPosted)
	}
	return s.SetJobStatus(wl, jobIdOrPipelineStageId, job.JobStatusPosted)
}

func (s service) GetPipelineStage(tx context.Context, pipelineId string, stageN int32) (job.PipelineStage, error) {
	stage, _, err := s.getPipelineStage(tx, pipelineId, stageN)
	return stage, err
}
func (s service) CanPutResumePipelineToStage(tx context.Context, pipelineId string, stageN int32) (bool, error) {
	if stageN <= 0 {
		return false, fmt.Errorf("invalid resume stage %d", stageN)
	}
	prevStage, _, err := s.getPipelineStage(tx, pipelineId, stageN-1)
	if err != nil {
		return false, err
	}
	return prevStage.Status == job.JobStatusSuccess, nil
}
func (s service) getPipelineStage(tx context.Context, pipelineId string, stageN int32) (stage job.PipelineStage, isNotFoundErr bool, err error) {
	stagesIter, err := s.GetPipelineStagesById(tx, pipelineId)
	if err != nil {
		return job.PipelineStage{}, false, err
	}
	found := false
	for stagesIter.Next() {
		stage, err = stagesIter.Get()
		if err != nil {
			return job.PipelineStage{}, false, err
		}
		if stage.Stage == stageN {
			found = true
			break
		}
	}
	err = stagesIter.Err()
	if err != nil {
		return job.PipelineStage{}, false, err
	}
	if !found {
		return job.PipelineStage{}, true, fmt.Errorf("stage %d not found in pipelineId=%q", stageN, pipelineId)
	}
	return stage, false, nil
}

func (s service) GetRepoPipelineRefNextAvailableRunNumber(tx context.Context,
	repoId uint64, filePath string, jobName string) (int64, error) {
	iter, err := s.GetRepoPipelinesByRef(tx, repoId, filePath, jobName, 0)
	if err != nil {
		return 0, err
	}
	// The first one is the most recent one, which should be the one with
	// the highest run number. Thus, we only iterate once.
	nextRunNumber := int64(0)
	if iter.Next() {
		p, err := iter.Get()
		if err != nil {
			return 0, err
		}
		nextRunNumber = p.RunNumber + 1
	}
	err = iter.Err()
	if err != nil {
		return 0, err
	}
	return nextRunNumber, nil
}
