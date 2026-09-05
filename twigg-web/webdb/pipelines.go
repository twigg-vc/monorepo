package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/twigg-web/job"
)

// Reports whether a pipeline row with that key exists.
func (db webDb) PipelineExists(ctx context.Context, repoId, commitId, commitVersion uint64,
	path, name string, runNumber int64) (bool, error) {
	var dummy int64
	err := db.s.QueryRow(ctx, `
		SELECT 1 FROM jobPipelines WHERE
		repoId = ? AND commitId = ? AND commitVersion = ? AND
		path = ? AND name = ? AND runNumber = ?
	`, repoId, commitId, commitVersion, path, name, runNumber).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query pipeline: %w", err)
	}
	return dummy == 1, nil
}

// Inserts a pipeline row and returns the internal id assigned to it.
func (db webDb) InsertPipeline(writeCtx context.Context, p job.Pipeline) (internalPipelineId int64, err error) {
	if p.Status == "" {
		return 0, fmt.Errorf("missing status")
	}
	if p.CreatedTime == "" {
		return 0, fmt.Errorf("missing createdTime")
	}
	err = db.s.QueryRow(writeCtx, `
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
		p.RepoId,
		p.Commit,
		p.CommitVersion,
		p.Path,
		p.Name,
		p.RunNumber,
		p.NumberOfStages,
		p.Status,
		p.CreatedTime,
		p.IsCreatedByUser,
		p.CreatedByUserId,
	).Scan(&internalPipelineId)
	if err != nil {
		return 0, fmt.Errorf("failed to insert pipeline: %w", err)
	}
	return internalPipelineId, nil
}

// Returns the pipeline with that key. If there is none, isNotFoundErr is true
// and err is ErrNotFound.
func (db webDb) GetPipeline(ctx context.Context, repoId, commitId, commitVersion uint64,
	path, name string, runNumber int64) (p job.Pipeline, isNotFoundErr bool, err error) {
	p = job.Pipeline{
		RepoId:        repoId,
		Commit:        commitId,
		CommitVersion: commitVersion,
		Path:          path,
		Name:          name,
		RunNumber:     runNumber,
	}
	err = db.s.QueryRow(ctx, `
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
	`, repoId, commitId, commitVersion, path, name, runNumber).Scan(
		&p.InternalId,
		&p.NumberOfStages,
		&p.Status,
		&p.CreatedTime,
		&p.IsCreatedByUser,
		&p.CreatedByUserId,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return job.Pipeline{}, true, ErrNotFound
	}
	if err != nil {
		return job.Pipeline{}, false, fmt.Errorf("failed to query pipeline: %w", err)
	}
	return p, false, nil
}
