package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (db webDb) GetCiCdQueueLastRunNumber(ctx context.Context,
	repoId, commitId, commitVersion uint64) (runNumber int64, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT runNumber FROM ci_queue4
		WHERE TRUE
		AND repoId = ?
		AND commitId = ?
		AND commitVersion = ?
		ORDER BY runNumber DESC
		LIMIT 1;
	`, repoId, commitId, commitVersion).Scan(&runNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, true, ErrNotFound
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to query ci queue run number: %w", err)
	}
	return runNumber, false, nil
}

func (db webDb) InsertCiCdQueueRun(writeCtx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64,
	trigger, nonce, status string) error {
	if trigger == "" {
		return fmt.Errorf("missing trigger")
	}
	if nonce == "" {
		return fmt.Errorf("missing nonce")
	}
	if status == "" {
		return fmt.Errorf("missing status")
	}
	_, err := db.s.Exec(writeCtx, `
	INSERT INTO ci_queue4 (
		repoId,
		commitId,
		commitVersion,
		runNumber,
		trigger,
		nonce,
		status
	) VALUES (?, ?, ?, ?, ?, ?, ?);
	`, repoId, commitId, commitVersion, runNumber, trigger, nonce, status)
	if err != nil {
		return fmt.Errorf("failed to insert ci queue run: %w", err)
	}
	return nil
}

func (db webDb) GetCiCdQueueLatestRunStatus(ctx context.Context,
	repoId, commitId, commitVersion uint64) (status string, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT
			status
		FROM ci_queue4
		WHERE
			TRUE
			AND repoId = ?
			AND commitId = ?
			AND commitVersion = ?
		ORDER BY runNumber DESC
		LIMIT 1;
	`, repoId, commitId, commitVersion).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", true, ErrNotFound
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to query ci queue status: %w", err)
	}
	return status, false, nil
}

func (db webDb) GetCiCdQueueRunTriggerAndStatus(ctx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64,
	nonce string) (trigger, status string, isNotFoundErr bool, err error) {
	if nonce == "" {
		return "", "", false, fmt.Errorf("missing nonce")
	}
	err = db.s.QueryRow(ctx, `
		SELECT
			trigger,
			status
		FROM ci_queue4
		WHERE
			TRUE
			AND repoId = ?
			AND commitId = ?
			AND commitVersion = ?
			AND runNumber = ?
			AND nonce = ?;
	`, repoId, commitId, commitVersion, runNumber, nonce).Scan(&trigger, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", true, ErrNotFound
	}
	if err != nil {
		return "", "", false, fmt.Errorf("failed to query ci queue run: %w", err)
	}
	return trigger, status, false, nil
}

func (db webDb) SetCiCdQueueRunStatus(writeCtx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64,
	nonce, status string) error {
	if nonce == "" {
		return fmt.Errorf("missing nonce")
	}
	if status == "" {
		return fmt.Errorf("missing status")
	}
	_, err := db.s.Exec(writeCtx, `
		UPDATE ci_queue4
		SET status = ?
		WHERE
			TRUE
			AND repoId = ?
			AND commitId = ?
			AND commitVersion = ?
			AND runNumber = ?
			AND nonce = ?;
	`, status, repoId, commitId, commitVersion, runNumber, nonce)
	if err != nil {
		return fmt.Errorf("failed to set ci queue run status: %w", err)
	}
	return nil
}
