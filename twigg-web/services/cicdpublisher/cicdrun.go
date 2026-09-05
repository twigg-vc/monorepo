package cicdpublisher

import (
	"context"
	"errors"
	"fmt"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/services/twiggtoken"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
	"monorepo/twigg/tree"
	"path"
	"slices"
	"time"
)

type publisher struct {
	tm             MaxAllowedTimeoutGetter
	jobs           JobsStorage
	serverProvider RepositoryServerProvider
	pr             Parser
	tc             TrackClient
	flags          FlagsProvider
	signer         twiggtoken.TokenSigner
}

func (s publisher) PutAutoCiCdRun(repoId uint64, commitId, commitVersion uint64,
	runNumber int64, trigger runnerlib.JobTrigger, w context.Context) error {
	if trigger == runnerlib.OnManual {
		return errors.New("PutAutoCiCdRun called for manual trigger")
	}
	// Check if we need to do an idempotent skip
	published, err := s.jobs.CiCdRunWasPublished(w, repoId, commitId, commitVersion, runNumber)
	if err != nil {
		return err
	}
	if published {
		return nil // Idempotent skip
	}

	// Read a "bunch of stuff"
	repoOwnerId, err := s.serverProvider.GetRepoOwnerId(w, repoId)
	if err != nil {
		return err
	}
	srv, err := s.serverProvider.GetServerByRepoId(w, repoId)
	if err != nil {
		return err
	}
	serverR := s.serverProvider.GetServerRead(w)
	c, err := srv.GetVersion(commitId, commitVersion, serverR)
	if err != nil {
		return err
	}
	p, err := srv.GetVersion(c.ParentL, c.ParentV, serverR)
	if err != nil {
		return err
	}
	maxTimeout, err := s.tm.GetMaxAllowedTimeout(repoOwnerId, repoId, w)
	if err != nil {
		return err
	}

	// Call the helpers for CI and CD with the "bunch of stuff" we just read
	err = s.postCiJobs(maxTimeout, trigger, repoOwnerId,
		repoId, runNumber, c, p, serverR, srv, w)
	if err != nil {
		return err
	}
	const isCreatedByUser = false // PutCiCd runs automatically (not by a user)
	const isCreatedByUserId = 0
	err = s.postCdJobs(maxTimeout, trigger, repoOwnerId,
		repoId, runNumber, c, p, serverR, srv, isCreatedByUser, isCreatedByUserId, w)
	if err != nil {
		return err
	}

	// Mark this CI/CD run as published so that we perform and idempotent skip
	// if this method is called again (see the first lines)
	return s.jobs.SetCiCdToPublished(w, repoId, commitId, commitVersion, runNumber)
}

func (s publisher) postCiJobs(maxTimeout time.Duration, trigger runnerlib.JobTrigger,
	repoOwnerId int64, repoId uint64, runNumber int64, c, p commit.Commit, serverR server.Read, srv server.Server, w context.Context) error {
	// Before posting anything, ensure the number of jobs and the total
	// timeout is ok. If not, create a job with a bad status.
	nJobsIsBad, timeoutIsBad, err := s.verifyTotalCiJobCountAndTimeout(maxTimeout,
		trigger, c, p, serverR, srv)
	if err != nil {
		return err
	}
	const jobFileNameForErrors = "CREATE_CI_JOBS"
	if nJobsIsBad {
		_, err = s.jobs.CreateNewJob(
			w, repoId, c.L, c.Version, jobFileNameForErrors, "", runNumber,
			job.JobStatusTooManyJobs)
		return err
	}
	if timeoutIsBad {
		_, err = s.jobs.CreateNewJob(
			w, repoId, c.L, c.Version, jobFileNameForErrors, "", runNumber,
			job.JobStatusExceedsPlanLimits)
		return err
	}

	// Once the number of jobs and total timeout was verified, create the
	// jobs at the db and put them to run on the track
	ciFiles, err := srv.SearchFileInChangedDirs(c, p, serverR, CiFilename)
	if err != nil {
		return err
	}
	for ciFiles.CanGet() {
		isCreated, isModified, isDeleted, path, _, file, _, _ := ciFiles.GetFile()
		if isCreated || isModified {
			err = s.createAndPutCiJobsOfFile(repoOwnerId, repoId, c, path, file,
				runNumber, trigger, w)
			if err != nil {
				return err
			}
		}
		if isDeleted {
			// Do nothing
		}
		err = ciFiles.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s publisher) verifyTotalCiJobCountAndTimeout(maxTimeout time.Duration, trigger runnerlib.JobTrigger, c, p commit.Commit,
	serverR server.Read, srv server.Server) (jobCountIsBad bool, timeoutIsBad bool, err error) {
	maxTimeoutMs := maxTimeout.Milliseconds()
	ciFiles, err := srv.SearchFileInChangedDirs(c, p, serverR, CiFilename)
	if err != nil {
		return
	}
	nJobs := 0
	timeoutMs := int64(0)
	for ciFiles.CanGet() {
		_, _, isDeleted, _, _, file, _, _ := ciFiles.GetFile()
		if isDeleted {
			err = ciFiles.Next()
			if err != nil {
				return
			}
			continue
		}

		ciJobs, sizeIsOk, fileIsOk, parseErr := s.parseCiFile(file)
		if parseErr != nil {
			err = parseErr
			return
		}
		// Only count what will actually run
		if sizeIsOk && fileIsOk {
			for _, ciJob := range ciJobs {
				if len(ciJob.On) > 0 && !slices.Contains(ciJob.On, trigger) {
					continue
				}
				nJobs += 1
				timeoutMs += ciJob.Job.TimeoutMilliSeconds
			}
		}
		if nJobs > MaxJobsPerCommit {
			jobCountIsBad = true
			return
		}
		if timeoutMs > maxTimeoutMs {
			timeoutIsBad = true
			return
		}
		err = ciFiles.Next()
		if err != nil {
			return
		}
	}
	return
}

// Reads a CI file and:
// 1 - Creates the jobs at the db
// 2 - Puts the jobs to the TrackClient
func (s publisher) createAndPutCiJobsOfFile(repoOwnerId int64, repoId uint64, c commit.Commit,
	jobFilePath string,
	jobFile tree.Tree,
	runNumber int64,
	trigger runnerlib.JobTrigger,
	w context.Context) (err error) {
	ciJobs, jobFileSizeIsOk, jobFileIsOk, err := s.parseCiFile(jobFile)
	if err != nil {
		return
	}
	if !jobFileSizeIsOk {
		_, err = s.jobs.CreateNewJob(w, repoId,
			c.L, c.Version, jobFilePath,
			/*jobName*/ "", runNumber, job.JobStatusBadFileSize)
		return
	}
	if !jobFileIsOk {
		_, err = s.jobs.CreateNewJob(
			w, repoId, c.L, c.Version, jobFilePath,
			/*jobName*/ "", runNumber, job.JobStatusBadFileFormat)
		return
	}

	const ciTokenDuration = 2 * time.Hour
	for i := range ciJobs {
		if len(ciJobs[i].On) > 0 &&
			!slices.Contains(ciJobs[i].On, trigger) {
			continue
		}
		err = injectEnvVarsAndDir(jobFilePath, repoId, ciTokenDuration, c, &ciJobs[i].Job, s.signer)
		if err != nil {
			return err
		}
		var jb job.Job
		jb, err = s.jobs.CreateNewJob(w, repoId,
			c.L, c.Version,
			jobFilePath, ciJobs[i].Job.Name, runNumber, job.JobStatusQueued)
		if err != nil {
			return
		}
		err = s.tc.Put(repoOwnerId, jb.Id(), ciJobs[i].Job, w)
		if err != nil {
			return
		}
	}
	return nil
}

func (s publisher) postCdJobs(maxTimeout time.Duration, trigger runnerlib.JobTrigger,
	repoOwnerId int64, repoId uint64, runNumber int64, c, p commit.Commit,
	serverR server.Read, srv server.Server, isCreatedByUser bool, isCreatedByUserId int64, w context.Context) error {
	f, err := s.flags.GetFlagsByRepoOwnerUserId(repoOwnerId, w)
	if err != nil {
		return err
	}
	if !f.CreateCdJobs {
		return nil
	}
	// Only do anything related to creating/deleting CD jobs on submit
	if trigger != runnerlib.OnSumit {
		return nil
	}

	// Before posting anything, ensure the number of jobs and the total
	// timeout is ok. If not, create a CI job with a bad status. this way,
	// the users will be able to see error messages that indicate
	// the cd jobs could not be created
	nJobsIsBad, timeoutIsBad, err := s.verifyTotalCdJobCountAndTimeout(maxTimeout,
		trigger, c, p, serverR, srv)
	if err != nil {
		return err
	}
	const jobFileNameForErrors = "CREATE_CD_JOBS"
	if nJobsIsBad {
		_, err = s.jobs.CreateNewJob(
			w, repoId, c.L, c.Version, jobFileNameForErrors, "", runNumber,
			job.JobStatusTooManyJobs)
		return err
	}
	if timeoutIsBad {
		_, err = s.jobs.CreateNewJob(
			w, repoId, c.L, c.Version, jobFileNameForErrors, "", runNumber,
			job.JobStatusExceedsPlanLimits)
		return err
	}

	// Once the number of jobs and total timeout was verified, create the
	// jobs at the db and put them to run on the track
	cdFiles, err := srv.SearchFileInChangedDirs(c, p, serverR, CdFilename)
	if err != nil {
		return err
	}
	for cdFiles.CanGet() {
		isCreated, isModified, isDeleted, path, _, file, cFile, pFile := cdFiles.GetFile()
		if isDeleted {
			err = s.archiveAllRefsOfCdFile(repoId, path, file, w)
			if err != nil {
				return err
			}
		}
		if isModified {
			err = s.archiveDeletedRefs(repoId, path, cFile, pFile, w)
			if err != nil {
				return err
			}
		}
		if isCreated || isModified {
			err = s.createAndPutCdJobsOfFile(repoOwnerId, repoId, c, path, file,
				runNumber, trigger, isCreatedByUser, isCreatedByUserId, w)
			if err != nil {
				return err
			}
		}
		err = cdFiles.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s publisher) verifyTotalCdJobCountAndTimeout(maxTimeout time.Duration, trigger runnerlib.JobTrigger, c, p commit.Commit,
	serverR server.Read, srv server.Server) (jobCountIsBad bool, timeoutIsBad bool, err error) {
	maxTimeoutMs := maxTimeout.Milliseconds()

	nJobs := 0
	timeoutMs := int64(0)
	cdFiles, err := srv.SearchFileInChangedDirs(c, p, serverR, CdFilename)
	if err != nil {
		return
	}
	for cdFiles.CanGet() {
		_, _, isDeleted, _, _, cdFile, _, _ := cdFiles.GetFile()
		if isDeleted {
			err = cdFiles.Next()
			if err != nil {
				return
			}
			continue
		}

		cdJobs, sizeIsOk, fileIsOk, parseErr := s.parseCdFile(cdFile)
		if parseErr != nil {
			err = parseErr
			return
		}
		// Only count what will actually run
		if sizeIsOk && fileIsOk {
			for i := range cdJobs {
				cdJob := &cdJobs[i]
				if len(cdJob.On) > 0 && !slices.Contains(cdJob.On, trigger) {
					continue
				}
				for _, stage := range cdJob.Stages {
					nJobs += 1
					timeoutMs += stage.TimeoutMilliSeconds
				}
			}
		}
		if nJobs > MaxJobsPerCommit {
			jobCountIsBad = true
			return
		}
		if timeoutMs > maxTimeoutMs {
			timeoutIsBad = true
			return
		}
		err = cdFiles.Next()
		if err != nil {
			return
		}
	}
	return
}

// Reads a CD file and:
// 1 - Creates the pipeline-ref (i.e. a pipeline "name") at the db if it doenst exist
// 2 - Create a pipeline (i.e. a running instance of a pipeline-ref)
// 3 - If allowed to auto-start: put the first stage to run on track. Else, set it to waiting manual start
func (s publisher) createAndPutCdJobsOfFile(repoOwnerId int64, repoId uint64, c commit.Commit,
	jobFilePath string,
	jobFile tree.Tree,
	runNumber int64,
	trigger runnerlib.JobTrigger,
	isCreatedByUser bool,
	isCreatedByUserId int64,
	w context.Context) (err error) {
	cdJobs, jobFileSizeIsOk, jobFileIsOk, err := s.parseCdFile(jobFile)
	if err != nil {
		return
	}
	if !jobFileSizeIsOk {
		_, err = s.jobs.CreateNewJob(w, repoId,
			c.L, c.Version, jobFilePath,
			/*jobName*/ "", runNumber, job.JobStatusBadFileSize)
		return
	}
	if !jobFileIsOk {
		_, err = s.jobs.CreateNewJob(
			w, repoId, c.L, c.Version, jobFilePath,
			/*jobName*/ "", runNumber, job.JobStatusBadFileFormat)
		return
	}
	for _, cdJob := range cdJobs {
		for i := range cdJob.Stages {
			err = injectEnvVarsAndDir(jobFilePath, repoId, cdTokenDuration, c, &cdJob.Stages[i].JobPayload, s.signer)
			if err != nil {
				return err
			}
		}
		_, err = s.jobs.PutPipelineRef(w, repoId, jobFilePath, cdJob.Name)
		if err != nil {
			return err
		}
		if len(cdJob.On) > 0 && !slices.Contains(cdJob.On, trigger) {
			continue
		}
		if len(cdJob.Stages) == 0 {
			panic("got valid cd job with zero stages")
		}

		stageNames := make([]string, len(cdJob.Stages))
		for i := range cdJob.Stages {
			stageNames[i] = cdJob.Stages[i].Name
		}
		firstStage := cdJob.Stages[0]

		// Create a running pipeline
		var jobPipeline job.Pipeline
		jobPipeline, err = s.jobs.CreateNewPipeline(w,
			repoId, c.L, c.Version,
			jobFilePath, cdJob.Name, runNumber,
			stageNames, isCreatedByUser, isCreatedByUserId)
		if err != nil {
			return err
		}
		// Put the first stage if allowed, or set its stage to waiting-manual
		if firstStage.CanAutoStart {
			err = s.jobs.SetStatusOfPipelineStage(
				w, jobPipeline.Id(), 0,
				job.JobStatusQueued)
			if err != nil {
				return err
			}
			err = s.tc.Put(repoOwnerId,
				jobPipeline.IdOfStage(0),
				firstStage.JobPayload, w)
			if err != nil {
				return err
			}
		} else {
			err = s.jobs.SetStatusOfPipelineStage(
				w, jobPipeline.Id(), 0,
				job.JobStatusWaitingManualStart)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func injectEnvVarsAndDir(jobFilePath string, repoId uint64, tokenDuration time.Duration, c commit.Commit, pl *runnerlib.JobPayload, signer twiggtoken.TokenSigner) error {

	tokenActions := []twiggtoken.TokenAction{twiggtoken.TokenActionPull}
	tokenActionsArg := []string{""}

	// Add 'get secrets action' for each unique secret
	uniqueRequiredSecretsNames := map[string]bool{}
	for i := range pl.Steps {
		for j := range pl.Steps[i].Secrets {
			uniqueRequiredSecretsNames[pl.Steps[i].Secrets[j]] = true
		}
	}
	for secretName := range uniqueRequiredSecretsNames {
		tokenActions = append(tokenActions, twiggtoken.TokenActionGetSecret)
		tokenActionsArg = append(tokenActionsArg, secretName)
	}

	token, err := twiggtoken.NewTwiggToken(
		repoId, c.ServerL, c.ServerV,
		tokenActions,
		tokenActionsArg,
		tokenDuration,
		signer,
	)
	if err != nil {
		return err
	}

	commitId := fmt.Sprintf("c%dv%d", c.ServerL, c.ServerV)
	repoIdStr := fmt.Sprintf("%d/%d", repoId, repoId)
	pl.Token = token
	for s := range pl.Steps {
		if pl.Steps[s].Env == nil {
			pl.Steps[s].Env = map[string]string{}
		}
		pl.Steps[s].Env[runnerlib.TwiggTokenEnvVarName] = token
		pl.Steps[s].Env[runnerlib.CommitIdEnvVarName] = commitId
		pl.Steps[s].Env[runnerlib.RepoIdEnvVarName] = repoIdStr
		if pl.Steps[s].Dir == "" {
			pl.Steps[s].Dir = path.Dir(jobFilePath)
		}
	}
	return nil
}

const cdTokenDuration = 6 * time.Hour

func (s publisher) archiveAllRefsOfCdFile(
	repoId uint64,
	jobFilePath string,
	jobFile tree.Tree,
	w context.Context) (err error) {
	cdJobs, jobFileSizeIsOk, jobFileIsOk, err := s.parseCdFile(jobFile)
	if err != nil {
		return
	}
	if !jobFileSizeIsOk {
		return
	}
	if !jobFileIsOk {
		return
	}
	for _, cdJob := range cdJobs {
		err = s.jobs.ArchivePipelineRefIfExists(w, repoId, jobFilePath, cdJob.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s publisher) archiveDeletedRefs(
	repoId uint64,
	jobFilePath string,
	newFile tree.Tree,
	oldFile tree.Tree,
	w context.Context) (err error) {
	// If the old file wasn't ok, its as if it didn't exist; so there's
	// nothing to archive.
	oldCdJobs, oldFileSizeIsOk, oldFileIsOk, err := s.parseCdFile(oldFile)
	if err != nil {
		return
	}
	if !oldFileSizeIsOk || !oldFileIsOk {
		return
	}
	// If the new file isn't ok, it's as if it were deleted. Archive all the old ones.
	newCdJobs, newFileSizeIsOk, newFileIsOk, err := s.parseCdFile(newFile)
	if err != nil {
		return
	}
	if !newFileSizeIsOk || !newFileIsOk {
		err = s.archiveAllRefsOfCdFile(repoId, jobFilePath, oldFile, w)
		return
	}

	// Get jobs that existed in the old file but are missing from the new file
	jobNamesToArchive := []string{}
	newJobNames := make(map[string]bool)
	for _, job := range newCdJobs {
		newJobNames[job.Name] = true
	}
	for _, job := range oldCdJobs {
		if _, exists := newJobNames[job.Name]; !exists {
			jobNamesToArchive = append(jobNamesToArchive, job.Name)
		}
	}
	// Archive them
	for _, name := range jobNamesToArchive {
		err = s.jobs.ArchivePipelineRefIfExists(w, repoId, jobFilePath, name)
		if err != nil {
			return err
		}
	}
	return
}
