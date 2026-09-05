package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/twigg-web/job"
)

// Reports whether a job row with that key exists.
func (db webDb) JobExists(ctx context.Context, repoId, commitId, commitVersion uint64,
	path, name string, runNumber int64) (bool, error) {
	var dummy int64
	err := db.s.QueryRow(ctx, `
		SELECT 1 FROM jobs3 WHERE
		repoId = ? AND commitId = ? AND commitVersion = ? AND
		path = ? AND name = ? AND runNumber = ?
	`, repoId, commitId, commitVersion, path, name, runNumber).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query job: %w", err)
	}
	return dummy == 1, nil
}

// Inserts a job row and returns the internal id assigned to it.
func (db webDb) InsertJob(writeCtx context.Context, j job.Job) (internalJobId int64, err error) {
	if j.Status == "" {
		return 0, fmt.Errorf("missing status")
	}
	if j.CreatedTime == "" {
		return 0, fmt.Errorf("missing createdTime")
	}
	err = db.s.QueryRow(writeCtx, `
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
	).Scan(&internalJobId)
	if err != nil {
		return 0, fmt.Errorf("failed to insert job: %w", err)
	}
	return internalJobId, nil
}
