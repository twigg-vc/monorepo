package webdb

import (
	"context"
	"database/sql"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
)

// Inserts a pipeline stage row. The columns that are not taken here keep their
// table default.
func (db webDb) InsertPipelineStage(writeCtx context.Context, pipelineId string,
	stage int32, name, createdTime string, status job.JobStatus) error {
	if pipelineId == "" {
		return fmt.Errorf("missing pipelineId")
	}
	if status == "" {
		return fmt.Errorf("missing status")
	}
	if createdTime == "" {
		return fmt.Errorf("missing createdTime")
	}
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO jobPipelineStages (
			jobPipelineId,
			stage,
			name,
			createdTime,
			status
		) VALUES (?, ?, ?, ?, ?)
	`, pipelineId, stage, name, createdTime, status)
	if err != nil {
		return fmt.Errorf("failed to insert pipeline stage: %w", err)
	}
	return nil
}

// Returns the stages of a pipeline, ordered by stage.
func (db webDb) GetPipelineStages(ctx context.Context,
	pipelineId string) (iterator.I[job.PipelineStage], error) {
	rows, err := db.s.Query(ctx, `
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
	`, pipelineId)
	if err != nil {
		return nil, fmt.Errorf("failed to query pipeline stages: %w", err)
	}
	return pipelineStageIterWrapper{rows}, nil
}

type pipelineStageIterWrapper struct {
	rows *sql.Rows
}

func (it pipelineStageIterWrapper) Get() (job.PipelineStage, error) {
	var s job.PipelineStage
	err := it.rows.Scan(
		&s.PipelineId,
		&s.Stage,
		&s.Name,
		&s.CreatedTime,
		&s.Status,
		&s.IsResumedByUser,
		&s.ResumedByUserId,
	)
	if err != nil {
		return job.PipelineStage{}, fmt.Errorf("failed to get pipeline stage from iter: %w", err)
	}
	return s, nil
}
func (it pipelineStageIterWrapper) Next() bool { return it.rows.Next() }
func (it pipelineStageIterWrapper) Err() error { return it.rows.Err() }
