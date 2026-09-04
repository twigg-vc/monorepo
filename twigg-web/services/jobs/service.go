package jobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"monorepo/base/iterator"
	"monorepo/twigg-web/webdb"
	"strconv"
	"strings"
	"time"
)

type service struct {
	db webdb.WebDb
}

func newService(db webdb.WebDb, wl context.Context) (service, error) {
	tx := db.Bind(wl)
	_, err := tx.Exec(`
	CREATE TABLE IF NOT EXISTS cicdruns (
		repoId        INTEGER  NOT NULL,
		commitId      INTEGER  NOT NULL,
		commitVersion INTEGER  NOT NULL,
		runNumber     INTEGER  NOT NULL,
		nonce         TEXT     NOT NULL,
		PRIMARY KEY   (repoId, commitId, commitVersion, runNumber)
	);

	CREATE TABLE IF NOT EXISTS jobs3 (
		internalJobId INTEGER PRIMARY KEY AUTOINCREMENT,
		repoId        INTEGER  NOT NULL,
		commitId      INTEGER  NOT NULL,
		commitVersion INTEGER  NOT NULL,
		path          TEXT     NOT NULL,
		name          TEXT     NOT NULL,
		runNumber     INTEGER  NOT NULL,
		status        TEXT     NOT NULL,
		createdTime   TEXT     NOT NULL,
		UNIQUE (repoId, commitId, commitVersion, path, name, runNumber)
	);
	CREATE INDEX IF NOT EXISTS jobs3_by_id
	ON jobs3 (repoId, commitId, commitVersion, path, name, runNumber);

	CREATE TABLE IF NOT EXISTS jobPipelines (
		internalJobPipelineId  INTEGER PRIMARY KEY AUTOINCREMENT,
		repoId                 INTEGER  NOT NULL,
		commitId               INTEGER  NOT NULL,
		commitVersion          INTEGER  NOT NULL,
		path                   TEXT     NOT NULL,
		name                   TEXT     NOT NULL,
		runNumber              INTEGER  NOT NULL,
		numberOfStages         INTEGER  NOT NULL,
		status                 TEXT     NOT NULL,
		createdTime            TEXT     NOT NULL,
		isCreatedByUser        BOOLEAN  NOT NULL,
		createdByUserId        INTEGER  NOT NULL,
		UNIQUE (repoId, commitId, commitVersion, path, name, runNumber)
	);
	CREATE INDEX IF NOT EXISTS jobPipelines_by_id
	ON jobPipelines (repoId, commitId, commitVersion, path, name, runNumber);
	CREATE INDEX IF NOT EXISTS jobPipelines_by_path_name_internalId
	ON jobPipelines (repoId, path, name, internalJobPipelineId);

	CREATE TABLE IF NOT EXISTS jobPipelineStages (
		jobPipelineId   TEXT  NOT NULL,
		stage           INTEGER  NOT NULL,
		name            TEXT     NOT NULL,
		createdTime     TEXT     NOT NULL,
		status          TEXT     NOT NULL,
		isResumedByUser BOOLEAN NOT NULL DEFAULT FALSE,
		resumedByUserId INTEGER  NOT NULL DEFAULT 0,
		PRIMARY KEY (jobPipelineId, stage)
	);

	CREATE TABLE IF NOT EXISTS jobPipelineRefs (
		repoId      INTEGER  NOT NULL,
		path        TEXT     NOT NULL,
		name        TEXT     NOT NULL,
		isArchived  BOOLEAN NOT NULL DEFAULT FALSE,
		PRIMARY KEY (repoId, path, name)
	);
	CREATE INDEX IF NOT EXISTS jobPipelineRefs_by_repoId
	ON jobPipelineRefs (repoId, isArchived);
	`)
	if err != nil {
		return service{}, fmt.Errorf(
			"failed to setup job service table: %s", err)
	}
	return service{db: db}, nil
}

func (s service) CiCdRunWasPublished(tx context.Context,
	repoId uint64, commit uint64, commitV uint64, runNumber int64) (bool, error) {
	var dummy int64
	err := s.db.Bind(tx).QueryRow(`
		SELECT
			1
		FROM cicdruns
		WHERE
			repoId = ?
			AND commitId = ?
			AND commitVersion = ?
			AND runNumber = ?
	`, repoId, commit, commitV, runNumber).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return dummy == 1, nil
}

func (s service) SetCiCdToPublished(tx context.Context,
	repoId uint64, commit uint64, commitV uint64, runNumber int64) error {
	_, err := s.db.Bind(tx).Exec(`
		INSERT INTO cicdruns (
			repoId,
			commitId,
			commitVersion,
			runNumber,
			nonce
		) VALUES (?, ?, ?, ?, ?)
	`, repoId, commit, commitV, runNumber, newNonce())
	return err
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
	filePath string, jobName string, runNumber int64, initialStatus JobStatus) (Job, error) {
	j := Job{
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
		return Job{}, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Job{}, errors.New("runNumber already taken")
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
		return Job{}, fmt.Errorf("failed to CreateJob: %s", err)
	}
	return j, nil
}

func (s service) GetJobById(rl context.Context, id string) (job Job, err error) {
	var idIsOk bool
	job.RepoId, job.Commit, job.CommitVersion,
		job.Path, job.Name, job.RunNumber, idIsOk = parseJobId(id)
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
	`, job.RepoId,
		job.Commit,
		job.CommitVersion,
		job.Path,
		job.Name,
		job.RunNumber).Scan(
		&job.InternalId,
		&job.Status,
		&job.CreatedTime,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return job, fmt.Errorf("GetJobById: job %s not found: %w", id, err)
		}
		return job, fmt.Errorf("GetJobById: failed to scan row: %w", err)
	}
	return job, nil
}
func (s service) SetJobStatus(wl context.Context, id string, status JobStatus) (err error) {
	RepoId, Commit, CommitVersion,
		Path, Name, RunNumber, idIsOk := parseJobId(id)
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
) (iterator.I[Job], error) {
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

func (it commitJobs) Get() (Job, error) {
	var j Job
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
		return Job{}, fmt.Errorf("commitJobs.Get: failed to scan job: %w", err)
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
) (iterator.I[Job], error) {
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

func parseJobId(id string) (repoId uint64, commit uint64, commitVersion uint64,
	path string, name string, runNumber int64, ok bool) {
	parts := strings.Split(id, ".")
	if len(parts) != 6 {
		return
	}
	var err error
	if repoId, err = strconv.ParseUint(parts[0], 10, 64); err != nil {
		return
	}
	if commit, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
		return
	}
	if commitVersion, err = strconv.ParseUint(parts[2], 10, 64); err != nil {
		return
	}
	var b []byte
	if b, err = base64.RawURLEncoding.DecodeString(parts[3]); err != nil {
		return
	}
	path = string(b)
	if b, err = base64.RawURLEncoding.DecodeString(parts[4]); err != nil {
		return
	}
	name = string(b)
	if runNumber, err = strconv.ParseInt(parts[5], 10, 64); err != nil {
		return
	}
	ok = true
	return
}

func (s service) CreateNewPipeline(tx context.Context,
	repoId uint64, commit uint64, commitV uint64,
	filePath string, jobName string, runNumber int64,
	stageNames []string, isCreatedByUser bool, createdByUserId int64) (Pipeline, error) {
	if len(stageNames) == 0 {
		return Pipeline{}, fmt.Errorf("cant create Pipeline with no stages")
	}
	if !isCreatedByUser {
		createdByUserId = 0
	}
	const initialStatus = PipelineStatusRunning
	createdTime := time.Now().UTC().Format(time.RFC3339)
	j := Pipeline{
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
		return Pipeline{}, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Pipeline{}, errors.New("runNumber already taken")
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
		return Pipeline{}, fmt.Errorf("failed to CreateNewPipeline: %s", err)
	}
	_, err = s.PutPipelineRef(tx, j.RepoId, j.Path, j.Name)
	if err != nil {
		return Pipeline{}, err
	}

	// Create all the stages in waiting status
	for i := range stageNames {
		stage := PipelineStage{
			PipelineId:  j.Id(),
			Stage:       int32(i),
			Name:        stageNames[i],
			Status:      JobStatusWaiting,
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
			return Pipeline{}, err
		}
	}

	return j, nil
}
func (s service) GetPipelineById(rl context.Context, id string) (Pipeline, error) {
	p, _, err := s.getPipelineById(rl, id)
	return p, err
}
func (s service) getPipelineById(rl context.Context, id string) (p Pipeline, isNotFoundErr bool, err error) {
	repoId, commitId, commitVersion,
		path, name, runNumber, ok := ParsePipelineId(id)
	if !ok {
		return Pipeline{}, false, fmt.Errorf("bad PipelineId: %q", id)
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
		p = Pipeline{}
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
func (s service) GetPipelineStagesById(rl context.Context, id string) (iterator.I[PipelineStage], error) {
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

func (it pipelineStages) Get() (PipelineStage, error) {
	var j PipelineStage
	if err := it.rows.Scan(
		&j.PipelineId,
		&j.Stage,
		&j.Name,
		&j.CreatedTime,
		&j.Status,
		&j.IsResumedByUser,
		&j.ResumedByUserId,
	); err != nil {
		return PipelineStage{}, fmt.Errorf("pipelineStages.Get: failed to scan job: %w", err)
	}
	return j, nil
}
func (it pipelineStages) Next() bool {
	return it.rows.Next()
}
func (it pipelineStages) Err() error {
	return it.rows.Err()
}

func (s service) SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status JobStatus) error {
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
func (s service) updatePipelineStatus(tx context.Context, pipe Pipeline) error {
	var currentStage PipelineStage
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
		if currentStage.Status != JobStatusSuccess {
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

	var pipelineStatus PipelineStatus
	switch currentStage.Status {
	case JobStatusWaitingManualStart:
		pipelineStatus = PipelineStatusWaitingManualStart
	case JobStatusWaiting:
		pipelineStatus = PipelineStatusRunning
	case JobStatusQueued:
		pipelineStatus = PipelineStatusRunning
	case JobStatusPosted:
		pipelineStatus = PipelineStatusRunning
	case JobStatusRunning:
		pipelineStatus = PipelineStatusRunning
	case JobStatusSuccess:
		pipelineStatus = PipelineStatusSuccess
	case JobStatusFail:
		pipelineStatus = PipelineStatusFail
	case JobStatusTimeout:
		pipelineStatus = PipelineStatusFail
	case JobStatusCanceled:
		pipelineStatus = PipelineStatusCancel
	case JobStatusTooManyJobs:
		pipelineStatus = PipelineStatusFail
	case JobStatusBadFileFormat:
		pipelineStatus = PipelineStatusFail
	case JobStatusBadFileSize:
		pipelineStatus = PipelineStatusFail
	case JobStatusExceedsPlanLimits:
		pipelineStatus = PipelineStatusFail
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
	repoId uint64, filePath string, jobName string) (PipelineRef, error) {
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
		return PipelineRef{}, fmt.Errorf("failed to PutPipelineRef : %s", err)
	}
	return PipelineRef{
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
	repoId uint64, afterPath string, afterJobName string) (iterator.I[PipelineRef], error) {
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

func (it pipelineNames) Get() (PipelineRef, error) {
	var j PipelineRef
	j.RepoId = it.repoId
	if err := it.rows.Scan(
		&j.Path,
		&j.Name,
	); err != nil {
		return PipelineRef{}, fmt.Errorf("pipelineNames.Get: failed to scan job: %w", err)
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
	repoId uint64, filePath string, jobName string, afterInternalJobId int64) (iterator.I[Pipeline], error) {
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

func (it pipelinesByName) Get() (Pipeline, error) {
	var j Pipeline
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
		return Pipeline{}, fmt.Errorf("pipelinesByName.Get: failed to scan job: %w", err)
	}
	return j, nil
}
func (it pipelinesByName) Next() bool {
	return it.rows.Next()
}
func (it pipelinesByName) Err() error {
	return it.rows.Err()
}