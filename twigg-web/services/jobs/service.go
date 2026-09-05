package jobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
	var dummyVar int64
	err := s.db.Bind(wl).QueryRow(
		`SELECT 1 FROM jobs3 WHERE
		repoId = ? AND commitId = ? AND commitVersion = ? AND
		path = ? AND name = ? AND runNumber = ?`,
		repoId, commit, commitV, filePath, jobName, runNumber).Scan(&dummyVar)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return job.Job{}, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return job.Job{}, errors.New("runNumber already taken")
	}
	err = s.db.Bind(wl).QueryRow(`
		INSERT INTO jobs3 (
			repoId,
			commitId,
			commitVersion,
			path,
			name,
			runNumber,
			status,
			createdTime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING internalJobId;
	`,
		j.RepoId,
		j.Commit,
		j.CommitVersion,
		j.Path,
		j.Name,
		j.RunNumber,
		j.Status,
		j.CreatedTime,
	).Scan(&j.InternalId)
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to CreateJob: %s", err)
	}
	return j, nil
}

func (s service) GetJobById(rl context.Context, id string) (j job.Job, err error) {
	var idIsOk bool
	j.RepoId, j.Commit, j.CommitVersion,
		j.Path, j.Name, j.RunNumber, idIsOk = job.ParseJobId(id)
	if !idIsOk {
		err = fmt.Errorf("bad job id %s", id)
		return
	}
	err = s.db.Bind(rl).QueryRow(`
		SELECT
			internalJobId,
			status,
			createdTime
		FROM jobs3
		WHERE repoId = ? AND commitId = ? AND commitVersion = ? AND
			path = ? AND name = ? AND runNumber = ?;
	`, j.RepoId,
		j.Commit,
		j.CommitVersion,
		j.Path,
		j.Name,
		j.RunNumber).Scan(
		&j.InternalId,
		&j.Status,
		&j.CreatedTime,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return j, fmt.Errorf("GetJobById: job %s not found: %w", id, err)
		}
		return j, fmt.Errorf("GetJobById: failed to scan row: %w", err)
	}
	return j, nil
}
func (s service) SetJobStatus(wl context.Context, id string, status job.JobStatus) (err error) {
	RepoId, Commit, CommitVersion,
		Path, Name, RunNumber, idIsOk := job.ParseJobId(id)
	if !idIsOk {
		err = fmt.Errorf("bad job id %s", id)
		return
	}
	res, err := s.db.Bind(wl).Exec(`
		UPDATE jobs3
		SET status = ?
		WHERE repoId = ? AND commitId = ? AND commitVersion = ? AND
			path = ? AND name = ? AND runNumber = ?;
	`, status, RepoId, Commit, CommitVersion, Path, Name, RunNumber)
	if err != nil {
		return fmt.Errorf("SetJobStatus: failed to update job status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("SetJobStatus: failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("SetJobStatus: job %s not found", id)
	}

	return nil
}
func (s service) GetCommitJobs(
	rl context.Context,
	repoId uint64,
	commit uint64,
	afterInternalJobId int64,
) (iterator.I[job.Job], error) {
	tx := s.db.Bind(rl)
	if afterInternalJobId == 0 {
		afterInternalJobId = math.MaxInt64
	}
	rows, err := tx.Query(`
		SELECT
			internalJobId,
			repoId,
			commitId,
			commitVersion,
			path,
			name,
			runNumber,
			status,
			createdTime
		FROM jobs3
		WHERE repoId = ?
		  AND commitId = ?
		  AND internalJobId < ?
		ORDER BY internalJobId DESC;
	`, repoId, commit, afterInternalJobId)
	if err != nil {
		return nil, fmt.Errorf("GetCommitJobs: query failed: %w", err)
	}
	return commitJobs{rows}, nil
}

type commitJobs struct {
	rows *sql.Rows
}

func (it commitJobs) Get() (job.Job, error) {
	var j job.Job
	var commitInt, commitVersion int64
	if err := it.rows.Scan(
		&j.InternalId,
		&j.RepoId,
		&commitInt,
		&commitVersion,
		&j.Path,
		&j.Name,
		&j.RunNumber,
		&j.Status,
		&j.CreatedTime,
	); err != nil {
		return job.Job{}, fmt.Errorf("commitJobs.Get: failed to scan job: %w", err)
	}
	j.Commit = uint64(commitInt)
	j.CommitVersion = uint64(commitVersion)
	return j, nil
}

func (it commitJobs) Next() bool {
	return it.rows.Next()
}

func (it commitJobs) Err() error {
	return it.rows.Err()
}
func (s service) GetRepoJobs(
	rl context.Context,
	repoId uint64,
	afterInternalJobId int64,
) (iterator.I[job.Job], error) {
	if afterInternalJobId == 0 {
		afterInternalJobId = math.MaxInt64
	}
	rows, err := s.db.Bind(rl).Query(`
		SELECT
			internalJobId,
			repoId,
			commitId,
			commitVersion,
			path,
			name,
			runNumber,
			status,
			createdTime
		FROM jobs3
		WHERE repoId = ? AND internalJobId < ?
		ORDER BY internalJobId DESC;
	`, repoId, afterInternalJobId)
	if err != nil {
		return nil, fmt.Errorf("GetRepoJobs: query failed: %w", err)
	}
	return commitJobs{rows}, nil
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
	var dummyVar int64
	err := s.db.Bind(tx).QueryRow(
		`SELECT 1 FROM jobPipelines WHERE
		repoId = ? AND commitId = ? AND commitVersion = ? AND
		path = ? AND name = ? AND runNumber = ?`,
		repoId, commit, commitV, filePath, jobName, runNumber).Scan(&dummyVar)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return job.Pipeline{}, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return job.Pipeline{}, errors.New("runNumber already taken")
	}
	err = s.db.Bind(tx).QueryRow(`
		INSERT INTO jobPipelines (
			repoId,
			commitId,
			commitVersion,
			path,
			name,
			runNumber,
			numberOfStages,
			status,
			createdTime,
			isCreatedByUser,
			createdByUserId
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING internalJobPipelineId;
	`,
		j.RepoId,
		j.Commit,
		j.CommitVersion,
		j.Path,
		j.Name,
		j.RunNumber,
		j.NumberOfStages,
		j.Status,
		j.CreatedTime,
		j.IsCreatedByUser,
		j.CreatedByUserId,
	).Scan(&j.InternalId)
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
		_, err = s.db.Bind(tx).Exec(`
		INSERT INTO jobPipelineStages (
			jobPipelineId,
			stage,
			name,
			createdTime,
			status
		) VALUES (?, ?, ?, ?, ?)
	`,
			stage.PipelineId,
			stage.Stage,
			stage.Name,
			stage.CreatedTime,
			stage.Status,
		)
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
	err = s.db.Bind(rl).QueryRow(`
		SELECT
			internalJobPipelineId,
			numberOfStages,
			status,
			createdTime,
			isCreatedByUser,
			createdByUserId
		FROM jobPipelines
		WHERE repoId=? AND commitId=? AND commitVersion=?
		AND path=? AND name=? AND runNumber=?
	`,
		repoId,
		commitId,
		commitVersion,
		path,
		name,
		runNumber,
	).Scan(
		&p.InternalId,
		&p.NumberOfStages,
		&p.Status,
		&p.CreatedTime,
		&p.IsCreatedByUser,
		&p.CreatedByUserId,
	)
	if err != nil {
		p = job.Pipeline{}
		isNotFoundErr = errors.Is(err, sql.ErrNoRows)
		return
	}
	p.RepoId = repoId
	p.Commit = commitId
	p.CommitVersion = commitVersion
	p.Path = path
	p.Name = name
	p.RunNumber = runNumber
	return
}
func (s service) GetPipelineStagesById(rl context.Context, id string) (iterator.I[job.PipelineStage], error) {
	rows, err := s.db.Bind(rl).Query(`
		SELECT
			jobPipelineId,
			stage,
			name,
			createdTime,
			status,
			isResumedByUser,
			resumedByUserId
		FROM jobPipelineStages
		WHERE jobPipelineId = ?
		ORDER BY stage
	`, id)
	if err != nil {
		return nil, err
	}
	return pipelineStages{rows}, nil
}

type pipelineStages struct {
	rows *sql.Rows
}

func (it pipelineStages) Get() (job.PipelineStage, error) {
	var j job.PipelineStage
	if err := it.rows.Scan(
		&j.PipelineId,
		&j.Stage,
		&j.Name,
		&j.CreatedTime,
		&j.Status,
		&j.IsResumedByUser,
		&j.ResumedByUserId,
	); err != nil {
		return job.PipelineStage{}, fmt.Errorf("pipelineStages.Get: failed to scan job: %w", err)
	}
	return j, nil
}
func (it pipelineStages) Next() bool {
	return it.rows.Next()
}
func (it pipelineStages) Err() error {
	return it.rows.Err()
}

func (s service) SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status job.JobStatus) error {
	pipeline, isNotFoundErr, err := s.getPipelineById(tx, pipelineId)
	if isNotFoundErr {
		return fmt.Errorf("pipelineId=%s not found", pipelineId)
	}
	if err != nil {
		return err
	}
	res, err := s.db.Bind(tx).Exec(`
		UPDATE jobPipelineStages
		SET status = ?
		WHERE jobPipelineId = ? AND stage = ?
	`,
		status,
		pipelineId,
		stage,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("jobPipelineId=%q stage=%d not found", pipelineId, stage)
	}

	return s.updatePipelineStatus(tx, pipeline)
}

func (s service) SetResumerOfPipelineStage(tx context.Context, pipelineId string, stage int32, userId int64) error {
	res, err := s.db.Bind(tx).Exec(`
		UPDATE jobPipelineStages
		SET isResumedByUser = ?, resumedByUserId = ?
		WHERE jobPipelineId = ? AND stage = ?
	`, true, userId, pipelineId, stage)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("jobPipelineId=%q stage=%d not found", pipelineId, stage)
	}
	return nil
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
	_, err = s.db.Bind(tx).Exec(`
		UPDATE jobPipelines
		SET status=?
		WHERE repoId=? AND commitId=? AND commitVersion=?
		AND path=? AND name=? AND runNumber=?
	`,
		pipelineStatus,
		pipe.RepoId,
		pipe.Commit,
		pipe.CommitVersion,
		pipe.Path,
		pipe.Name,
		pipe.RunNumber,
	)
	return err
}

func (s service) PutPipelineRef(tx context.Context,
	repoId uint64, filePath string, jobName string) (job.PipelineRef, error) {
	_, err := s.db.Bind(tx).Exec(`
		INSERT INTO jobPipelineRefs (
			repoId,
			path,
			name,
			isArchived
		) VALUES (?, ?, ?, ?)
		ON CONFLICT (repoId, path, name) DO UPDATE
		SET isArchived = FALSE;
	`,
		repoId,
		filePath,
		jobName,
		false,
	)
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
	_, err := s.db.Bind(tx).Exec(`
        UPDATE jobPipelineRefs
        SET isArchived = TRUE
        WHERE repoId = ? AND path = ? AND name = ?;
    `,
		repoId, filePath, jobName,
	)
	if err != nil {
		return fmt.Errorf("failed to archive pipeline ref: %w", err)
	}
	return nil
}

func (s service) GetRepoPipelineRefs(tx context.Context,
	repoId uint64, afterPath string, afterJobName string) (iterator.I[job.PipelineRef], error) {
	args := []any{repoId}
	querySuffix := ""
	if afterPath != "" || afterJobName == "" {
		querySuffix = "AND (path, name) > (?, ?)"
		args = append(args, afterPath, afterJobName)
	}
	rows, err := s.db.Bind(tx).Query(fmt.Sprintf(`
		SELECT
			path,
			name
		FROM jobPipelineRefs
		WHERE
			repoId = ?
			%s
			AND NOT isArchived
		ORDER BY path, name
	`, querySuffix), args...)
	if err != nil {
		return nil, err
	}
	return pipelineNames{repoId, rows}, nil
}

type pipelineNames struct {
	repoId uint64
	rows   *sql.Rows
}

func (it pipelineNames) Get() (job.PipelineRef, error) {
	var j job.PipelineRef
	j.RepoId = it.repoId
	if err := it.rows.Scan(
		&j.Path,
		&j.Name,
	); err != nil {
		return job.PipelineRef{}, fmt.Errorf("pipelineNames.Get: failed to scan job: %w", err)
	}
	return j, nil
}
func (it pipelineNames) Next() bool {
	return it.rows.Next()
}
func (it pipelineNames) Err() error {
	return it.rows.Err()
}

func (s service) GetRepoPipelinesByRef(tx context.Context,
	repoId uint64, filePath string, jobName string, afterInternalJobId int64) (iterator.I[job.Pipeline], error) {
	if afterInternalJobId == 0 {
		afterInternalJobId = math.MaxInt64
	}
	rows, err := s.db.Bind(tx).Query(`
		SELECT
			internalJobPipelineId,
			repoId,
			commitId,
			commitVersion,
			path,
			name,
			runNumber,
			numberOfStages,
			status,
			createdTime,
			isCreatedByUser,
			createdByUserId
		FROM jobPipelines
		WHERE repoId = ? AND path = ? AND name = ? AND internalJobPipelineId < ?
		ORDER BY internalJobPipelineId DESC
	`, repoId, filePath, jobName, afterInternalJobId)
	if err != nil {
		return nil, err
	}
	return pipelinesByName{rows}, err
}

type pipelinesByName struct {
	rows *sql.Rows
}

func (it pipelinesByName) Get() (job.Pipeline, error) {
	var j job.Pipeline
	if err := it.rows.Scan(
		&j.InternalId,
		&j.RepoId,
		&j.Commit,
		&j.CommitVersion,
		&j.Path,
		&j.Name,
		&j.RunNumber,
		&j.NumberOfStages,
		&j.Status,
		&j.CreatedTime,
		&j.IsCreatedByUser,
		&j.CreatedByUserId,
	); err != nil {
		return job.Pipeline{}, fmt.Errorf("pipelinesByName.Get: failed to scan job: %w", err)
	}
	return j, nil
}
func (it pipelinesByName) Next() bool {
	return it.rows.Next()
}
func (it pipelinesByName) Err() error {
	return it.rows.Err()
}
