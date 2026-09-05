package webdb

import (
	"context"
	"fmt"
)

// Adds a job to the track queue. Does nothing if the job is already queued.
func (db webDb) InsertTrackQueueJob(writeCtx context.Context, jobId string,
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
