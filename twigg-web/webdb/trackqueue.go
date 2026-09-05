package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
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

// Sets the limits of the owner. Starts tracking the owner with zero usage if
// it is not tracked yet; the usage of an already tracked owner is kept.
func (db webDb) SetTrackOwnerLimits(writeCtx context.Context, ownerId int64,
	maxJobs int64, maxTimeoutMs int64) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO owner_usage2 (
			owner_id,
			running_jobs,
			running_timeout_ms,
			max_running_jobs,
			max_running_timeout_ms
		)
		VALUES (?, 0, 0, ?, ?)
		ON CONFLICT (owner_id) DO UPDATE SET
			max_running_jobs = excluded.max_running_jobs,
			max_running_timeout_ms = excluded.max_running_timeout_ms;
	`, ownerId, maxJobs, maxTimeoutMs)
	if err != nil {
		return fmt.Errorf("failed to set track owner limits: %w", err)
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

// Returns the owner and the payload of the queued job. Returns ErrNotFound if
// the job is not queued.
func (db webDb) GetTrackQueueJobOwnerAndPayload(ctx context.Context,
	jobId string) (ownerId int64, payload []byte, isNotFoundErr bool, err error) {
	if jobId == "" {
		return 0, nil, false, fmt.Errorf("missing jobId")
	}
	err = db.s.QueryRow(ctx, `
		SELECT owner_id, payload
		FROM track_queue
		WHERE job_id = ?;
	`, jobId).Scan(&ownerId, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, true, ErrNotFound
	}
	if err != nil {
		return 0, nil, false,
			fmt.Errorf("failed to query track queue job: %w", err)
	}
	return ownerId, payload, false, nil
}

// Removes the job from the track queue.
func (db webDb) DeleteTrackQueueJob(writeCtx context.Context,
	jobId string) error {
	if jobId == "" {
		return fmt.Errorf("missing jobId")
	}
	_, err := db.s.Exec(writeCtx, `
		DELETE FROM track_queue
		WHERE job_id = ?;
	`, jobId)
	if err != nil {
		return fmt.Errorf("failed to delete track queue job: %w", err)
	}
	return nil
}

// Adds the deltas to the usage the owner is currently running. The deltas are
// negative when a job stops running.
func (db webDb) AddTrackOwnerUsage(writeCtx context.Context, ownerId int64,
	runningJobsDelta int64, runningTimeoutMsDelta int64) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE owner_usage2
		SET
			running_jobs = running_jobs + ?,
			running_timeout_ms = running_timeout_ms + ?
		WHERE owner_id = ?;
	`, runningJobsDelta, runningTimeoutMsDelta, ownerId)
	if err != nil {
		return fmt.Errorf("failed to add track owner usage: %w", err)
	}
	return nil
}

// Returns the oldest job with the status whose owner is running less than the
// limits set for it. Returns ErrNotFound when no job is within the limits.
func (db webDb) GetOldestTrackQueueJobWithinOwnerLimits(ctx context.Context,
	status string) (jobId string, ownerId int64, payload []byte,
	isNotFoundErr bool, err error) {
	if status == "" {
		return "", 0, nil, false, fmt.Errorf("missing status")
	}
	err = db.s.QueryRow(ctx, `
		SELECT q.job_id, q.owner_id, q.payload
		FROM track_queue q
		JOIN owner_usage2 u ON u.owner_id = q.owner_id
		WHERE q.status = ?
		  AND u.running_jobs < u.max_running_jobs
		  AND u.running_timeout_ms < u.max_running_timeout_ms
		ORDER BY q.created_at_ns
		LIMIT 1;
	`, status).Scan(&jobId, &ownerId, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil, true, ErrNotFound
	}
	if err != nil {
		return "", 0, nil, false,
			fmt.Errorf("failed to query next track queue job: %w", err)
	}
	return jobId, ownerId, payload, false, nil
}

// Sets the status of the queued job.
func (db webDb) SetTrackQueueJobStatus(writeCtx context.Context, jobId string,
	status string) error {
	if jobId == "" {
		return fmt.Errorf("missing jobId")
	}
	if status == "" {
		return fmt.Errorf("missing status")
	}
	_, err := db.s.Exec(writeCtx, `
		UPDATE track_queue
		SET status = ?
		WHERE job_id = ?;
	`, status, jobId)
	if err != nil {
		return fmt.Errorf("failed to set track queue job status: %w", err)
	}
	return nil
}

// Returns the ids of the jobs with the status, oldest first.
func (db webDb) GetTrackQueueJobIdsByStatus(ctx context.Context,
	status string) (iterator.I[string], error) {
	if status == "" {
		return nil, fmt.Errorf("missing status")
	}
	rows, err := db.s.Query(ctx, `
		SELECT job_id
		FROM track_queue
		WHERE status = ?
		ORDER BY created_at_ns ASC;
	`, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query track queue job ids: %w", err)
	}
	return jobIdIterWrapper{rows}, nil
}

type jobIdIterWrapper struct {
	rows *sql.Rows
}

func (it jobIdIterWrapper) Get() (string, error) {
	var jobId string
	err := it.rows.Scan(&jobId)
	if err != nil {
		return "", fmt.Errorf("failed to get track queue job id from iter: %w", err)
	}
	return jobId, nil
}
func (it jobIdIterWrapper) Next() bool { return it.rows.Next() }
func (it jobIdIterWrapper) Err() error { return it.rows.Err() }

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
