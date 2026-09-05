package webdb

import (
	"context"
	"database/sql"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/job"
)

// Creates a pipeline ref row. Un-archives it if it already exists.
func (db webDb) PutPipelineRef(writeCtx context.Context,
	repoId uint64, path, name string) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO jobPipelineRefs (
			repoId,
			path,
			name,
			isArchived
		) VALUES (?, ?, ?, ?)
		ON CONFLICT (repoId, path, name) DO UPDATE
		SET isArchived = FALSE;
	`, repoId, path, name, false)
	if err != nil {
		return fmt.Errorf("failed to put pipeline ref: %w", err)
	}
	return nil
}

// Archives the pipeline ref row. No-op if there is none.
func (db webDb) ArchivePipelineRef(writeCtx context.Context,
	repoId uint64, path, name string) error {
	_, err := db.s.Exec(writeCtx, `
        UPDATE jobPipelineRefs
        SET isArchived = TRUE
        WHERE repoId = ? AND path = ? AND name = ?;
    `, repoId, path, name)
	if err != nil {
		return fmt.Errorf("failed to archive pipeline ref: %w", err)
	}
	return nil
}

// Returns the non-archived pipeline refs of a repo, ordered by path and name.
// Use afterPath and afterName to start after a ref.
func (db webDb) GetRepoPipelineRefs(ctx context.Context,
	repoId uint64, afterPath, afterName string) (iterator.I[job.PipelineRef], error) {
	args := []any{repoId}
	querySuffix := ""
	if afterPath != "" || afterName != "" {
		querySuffix = "AND (path, name) > (?, ?)"
		args = append(args, afterPath, afterName)
	}
	rows, err := db.s.Query(ctx, fmt.Sprintf(`
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
		return nil, fmt.Errorf("failed to query pipeline refs: %w", err)
	}
	return pipelineRefIterWrapper{repoId, rows}, nil
}

type pipelineRefIterWrapper struct {
	repoId uint64
	rows   *sql.Rows
}

func (it pipelineRefIterWrapper) Get() (job.PipelineRef, error) {
	r := job.PipelineRef{RepoId: it.repoId}
	err := it.rows.Scan(&r.Path, &r.Name)
	if err != nil {
		return job.PipelineRef{}, fmt.Errorf("failed to get pipeline ref from iter: %w", err)
	}
	return r, nil
}
func (it pipelineRefIterWrapper) Next() bool { return it.rows.Next() }
func (it pipelineRefIterWrapper) Err() error { return it.rows.Err() }