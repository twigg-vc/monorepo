package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Adds a job to the track queue. Does nothing if the job is already queued.
func (db webDb) InsertTrackQueueJobIfNotExists(writeCtx context.Context, jobId string,
	ownerId int64, payload []byte, status string, createdAtNs int64) error {
	if jobId == "" {
		return fmt.Errorf("missing jobId")
	}
	if status == "" {
		return fmt.Errorf("missing status")
	}
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO track_queue
			(job_id, owner_id, payload, status, created_at_ns)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (job_id) DO NOTHING;
	`, jobId, ownerId, payload, status, createdAtNs)
	if err != nil {
		return fmt.Errorf("failed to insert track queue job: %w", err)
	}
	return nil
}

// Starts tracking the owner's usage. Does nothing if the owner is already
// tracked, so the limits already set for it are kept.
func (db webDb) InsertZeroTrackOwnerUsageIfNotExists(writeCtx context.Context,
	ownerId int64) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO owner_usage2 (owner_id, running_jobs, running_timeout_ms)
		VALUES (?, 0, 0)
		ON CONFLICT (owner_id) DO NOTHING;
	`, ownerId)
	if err != nil {
		return fmt.Errorf("failed to insert track owner usage: %w", err)
	}
	return nil
}

// Returns the limits set for the owner. Returns ErrNotFound if the owner is
// not tracked.
func (db webDb) GetTrackOwnerLimits(ctx context.Context,
	ownerId int64) (maxJobs int64, maxTimeoutMs int64, isNotFoundErr bool,
	err error) {
	err = db.s.QueryRow(ctx, `
		SELECT max_running_jobs, max_running_timeout_ms
		FROM owner_usage2
		WHERE owner_id = ?;
	`, ownerId).Scan(&maxJobs, &maxTimeoutMs)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, true, ErrNotFound
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to query track owner limits: %w", err)
	}
	return maxJobs, maxTimeoutMs, false, nil
}

// Returns how many jobs are in the track queue.
func (db webDb) CountTrackQueueJobs(ctx context.Context) (int64, error) {
	var count int64
	err := db.s.QueryRow(ctx, `
		SELECT COUNT(*) FROM track_queue;
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count track queue jobs: %w", err)
	}
	return count, nil
}
