package cicdpublisher

import (
	"context"
	"fmt"
	"math"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/services/jobs"
	"monorepo/twigg/commit"
	"slices"
)

func (s publisher) PutResumePipelineWaitingStage(pipelineId string, atStage int32, w context.Context) error {
	// Check if the stage is in fact waiting
	stage, err := s.jobs.GetPipelineStage(w, pipelineId, atStage)
	if err != nil {
		return err
	}
	// If not, just return (idempotent skip)
	if stage.Status != jobs.JobStatusWaiting {
		return nil
	}

	return s.resumePipelineStage( /*isManualResume*/ false,
		pipelineId, atStage, w)
}

func (s publisher) ManualResumePipeline(pipelineId string, currentStage int32, userId int64, w context.Context) (isCantResumeErr bool, err error) {
	stage, err := s.jobs.GetPipelineStage(w, pipelineId, currentStage)
	if err != nil {
		return false, err
	}
	if stage.Status != jobs.JobStatusWaitingManualStart {
		return true, fmt.Errorf(
			"pipelineId=%q stage=%d stage=%q is not waiting for manual",
			pipelineId, currentStage, stage.Status)
	}
	isCantResumeErr = false
	err = s.resumePipelineStage( /*isManualResume*/ true,
		pipelineId, currentStage, w)
	if err != nil {
		return
	}
	err = s.jobs.SetResumerOfPipelineStage(w, pipelineId, currentStage, userId)
	if err != nil {
		return
	}
	return
}

func (s publisher) ManuallyLaunchCd(repoId, commitId, commitVersion uint64, jobPath, jobName string, userId int64, w context.Context) error {
	_, err := s.jobs.PutPipelineRef(w, repoId, jobPath, jobName)
	if err != nil {
		return err
	}
	runNumber, err := s.jobs.GetRepoPipelineRefNextAvailableRunNumber(w, repoId, jobPath, jobName)
	if err != nil {
		return err
	}
	_, cdJobs, jobFileSizeIsOk, jobFileIsOk, err := s.readCdFile(commitId, commitVersion, repoId, jobPath, w)
	if err != nil {
		return err
	}
	// We just need to know the name of each stage, and then create a pipeline
	// with all the required stages.
	var stageNames []string
	if !jobFileSizeIsOk || !jobFileIsOk {
		stageNames = []string{""}
	} else {
		found, cdJob := getCdJobByName(cdJobs, jobName)
		if !found {
			return fmt.Errorf("cant resume pipeliner-ref (%d-%s-%s) at commit c%dv%d: job not found",
				repoId, jobPath, jobName, commitId, commitVersion)
		}
		if len(cdJob.Stages) == 0 {
			return fmt.Errorf("cant resume pipeliner-ref (%d-%s-%s) at commit c%dv%d: job has no stages",
				repoId, jobPath, jobName, commitId, commitVersion)
		}
		if len(cdJob.Stages) >= math.MaxInt32 {
			return fmt.Errorf("cant resume pipeliner-ref (%d-%s-%s) at commit c%dv%d: too many stages",
				repoId, jobPath, jobName, commitId, commitVersion)
		}
		if !slices.Contains(cdJob.On, runnerlib.OnManual) {
			return fmt.Errorf("cant manually resume pipeliner-ref (%d-%s-%s) at commit c%dv%d: manual trigger not supported",
				repoId, jobPath, jobName, commitId, commitVersion)
		}
		stageNames = make([]string, 0, len(cdJob.Stages))
		for stage := range cdJob.Stages {
			stageNames = append(stageNames, cdJob.Stages[stage].Name)
		}
	}
	// Create the pipeline and all the stages in waiting status
	pipeline, err := s.jobs.CreateNewPipeline(w,
		repoId, commitId, commitVersion,
		jobPath, jobName, runNumber, stageNames, true, userId)
	if err != nil {
		return err
	}
	// Resume the first stage
	// Note: calling `resumePipelineStage` is definitely sub-optimal in terms
	// of performance because it'll re-read many things we already have read
	// here (such as the commit, the cdjobs and others). However, it makes
	// this code MUCH easier to follow and maintain; which I think is a good
	// tradeoff.
	return s.resumePipelineStage( /*isManualResume*/ false, pipeline.Id(), 0, w)
}

// If `isManualResume`, will start the stage even if it can't auto start
func (s publisher) resumePipelineStage(isManualResume bool, pipelineId string, currentStage int32, w context.Context) error {
	// Get the repoId, commit, etc to read the actual job file
	repoId, commitId, commitVersion,
		filePath, jobName, _, ok := jobs.ParsePipelineId(pipelineId)
	if !ok {
		return fmt.Errorf("bad pipelineId=%q", pipelineId)
	}
	c, cdJobs, jobFileSizeIsOk, jobFileIsOk, err := s.readCdFile(commitId, commitVersion, repoId, filePath, w)
	if err != nil {
		return err
	}
	if !jobFileSizeIsOk {
		err = s.jobs.SetStatusOfPipelineStage(
			w, pipelineId, currentStage,
			jobs.JobStatusBadFileSize)
		if err != nil {
			return err
		}
		return nil
	}
	if !jobFileIsOk {
		err = s.jobs.SetStatusOfPipelineStage(
			w, pipelineId, currentStage,
			jobs.JobStatusBadFileFormat)
		if err != nil {
			return err
		}
		return nil
	}
	// Get the specified job by the name
	found, cdJob := getCdJobByName(cdJobs, jobName)
	if !found {
		return fmt.Errorf("cant resume pipelineId=%q at stage %d: jobName=%q not found", pipelineId, currentStage, jobName)
	}
	if currentStage < 0 || int64(currentStage) >= int64(len(cdJob.Stages)) {
		return fmt.Errorf("cant resume pipelineId=%q at stage %d: job has %d stages", pipelineId, currentStage, len(cdJob.Stages))
	}
	// Finally, get the payload of the stage we want to run
	cdJobStagePayload := cdJob.Stages[currentStage]

	// Having all the required data we can now put the stage to run
	// and update its stage

	// Check if the owner's allowed timeout is ok.
	// If not, just set the stage to canceled.
	repoOwnerId, timeoutIsOk, err := s.getRepoOwnerIdAndCheckStageTimeoutIsOk(
		cdJobStagePayload.TimeoutMilliSeconds, repoId, w)
	if err != nil {
		return err
	}
	if !timeoutIsOk {
		err = s.jobs.SetStatusOfPipelineStage(w,
			pipelineId, currentStage, jobs.JobStatusExceedsPlanLimits)
		if err != nil {
			return err
		}
		return nil
	}

	// Auto-start the stage if allowed; or just put it in waiting manual start
	if cdJobStagePayload.CanAutoStart || isManualResume {
		err = s.jobs.SetStatusOfPipelineStage(w, pipelineId, currentStage,
			jobs.JobStatusQueued)
		if err != nil {
			return err
		}
		err = injectEnvVarsAndDir(filePath, repoId, cdTokenDuration, c, &cdJobStagePayload.JobPayload, s.signer)
		if err != nil {
			return err
		}
		err = s.tc.Put(repoOwnerId,
			jobs.PipelineStageId(pipelineId, currentStage),
			cdJobStagePayload.JobPayload, w)
		if err != nil {
			return err
		}
	} else {
		err = s.jobs.SetStatusOfPipelineStage(w, pipelineId, currentStage,
			jobs.JobStatusWaitingManualStart)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s publisher) readCdFile(commitId, commitVersion, repoId uint64, filePath string, w context.Context) (
	c commit.Commit, cdJobs []runnerlib.CdJob, jobFileSizeIsOk bool, jobFileIsOk bool, err error) {
	srv, err := s.serverProvider.GetServerByRepoId(w, repoId)
	if err != nil {
		return
	}
	serverR := s.serverProvider.GetServerRead(w)
	c, err = srv.GetVersion(commitId, commitVersion, serverR)
	if err != nil {
		return
	}
	jobFile, err := srv.GetTree(c, filePath, serverR)
	if err != nil {
		return
	}
	cdJobs, jobFileSizeIsOk, jobFileIsOk, err = s.parseCdFile(jobFile)
	return
}

// Simple helper to iterate on jobs and get one by name
func getCdJobByName(cdJobs []runnerlib.CdJob, name string) (found bool, cdJob runnerlib.CdJob) {
	for i := range cdJobs {
		if cdJobs[i].Name == name {
			cdJob = cdJobs[i]
			found = true
			return
		}
	}
	return
}

// Simple helper to read the repoOwnerId and check that the timeout of the stage
// is <= the max timeout allowed for them
func (s publisher) getRepoOwnerIdAndCheckStageTimeoutIsOk(
	stageTimeoutMilliSeconds int64, repoId uint64, r context.Context) (repoOwnerId int64, ok bool, err error) {
	repoOwnerId, err = s.serverProvider.GetRepoOwnerId(r, repoId)
	if err != nil {
		return
	}
	maxTimeout, err := s.tm.GetMaxAllowedTimeout(repoOwnerId, repoId, r)
	if err != nil {
		return
	}
	ok = stageTimeoutMilliSeconds <= maxTimeout.Milliseconds()
	err = nil
	return
}
