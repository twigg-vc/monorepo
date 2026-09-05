package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (db webDb) CiCdRunExists(ctx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64) (bool, error) {
	var dummy int64
	err := db.s.QueryRow(ctx, `
		SELECT
			1
		FROM cicdruns
		WHERE
			repoId = ?
			AND commitId = ?
			AND commitVersion = ?
			AND runNumber = ?
	`, repoId, commitId, commitVersion, runNumber).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query cicd run: %w", err)
	}
	return dummy == 1, nil
}

func (db webDb) InsertCiCdRun(writeCtx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64, nonce string) error {
	if nonce == "" {
		return fmt.Errorf("missing nonce")
	}
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO cicdruns (
			repoId,
			commitId,
			commitVersion,
			runNumber,
			nonce
		) VALUES (?, ?, ?, ?, ?)
	`, repoId, commitId, commitVersion, runNumber, nonce)
	if err != nil {
		return fmt.Errorf("failed to insert cicd run: %w", err)
	}
	return nil
}
