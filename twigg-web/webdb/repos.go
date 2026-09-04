package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/repo"
)

func (db webDb) CreateRepo(writeCtx context.Context, ownerId int64,
	displayName, description string) (repoId uint64, err error) {
	if displayName == "" {
		return 0, fmt.Errorf("missing displayName")
	}
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO repos (ownerId, displayName, description, isGitMirrorEnabled)
		VALUES (?, ?, ?, FALSE)
		RETURNING repoId;
	`, ownerId, displayName, description).Scan(&repoId)
	if err != nil {
		return 0, fmt.Errorf("failed to create repo: %w", err)
	}
	return repoId, nil
}

func (db webDb) GetRepoById(ctx context.Context, repoId uint64) (repo.Repo, error) {
	var ownerId int64
	var displayName string
	var description string
	var sanitizedGitMirrorUrl string
	var enabled bool
	var isPublic bool
	err := db.s.QueryRow(ctx, `
		SELECT
			ownerId,
			displayName,
			description,
			isGitMirrorEnabled,
			COALESCE(sanitizedGitMirrorUrl, ''),
			isPublic
		FROM repos
		WHERE
			repoId = ?;
	`, repoId).Scan(&ownerId, &displayName, &description, &enabled,
		&sanitizedGitMirrorUrl, &isPublic)
	if errors.Is(err, sql.ErrNoRows) {
		return repo.Repo{}, ErrNotFound
	}
	if err != nil {
		return repo.Repo{}, fmt.Errorf("failed to query repo: %w", err)
	}
	return repo.NewRepo(repoId, ownerId, displayName, description, enabled,
		sanitizedGitMirrorUrl, isPublic), nil
}

func (db webDb) GetRepoByOwnerIdAndName(ctx context.Context,
	ownerId int64, displayName string) (r repo.Repo, isNotFoundErr bool, err error) {
	var repoId uint64
	var description string
	var sanitizedGitMirrorUrl string
	var enabled bool
	var isPublic bool
	err = db.s.QueryRow(ctx, `
		SELECT
			repoId, description, isGitMirrorEnabled,
			COALESCE(sanitizedGitMirrorUrl, ''), isPublic
		FROM repos
		WHERE
			ownerId = ? AND displayName = ?;
	`, ownerId, displayName).Scan(&repoId, &description, &enabled,
		&sanitizedGitMirrorUrl, &isPublic)
	if errors.Is(err, sql.ErrNoRows) {
		return repo.Repo{}, true, ErrNotFound
	}
	if err != nil {
		return repo.Repo{}, false, fmt.Errorf("failed to query repo: %w", err)
	}
	return repo.NewRepo(repoId, ownerId, displayName, description, enabled,
		sanitizedGitMirrorUrl, isPublic), false, nil
}

func (db webDb) GetReposByOwnerId(ctx context.Context, ownerId int64) (iterator.I[repo.Repo], error) {
	rows, err := db.s.Query(ctx, `
		SELECT
			repoId, ownerId, displayName, description, isGitMirrorEnabled,
			COALESCE(sanitizedGitMirrorUrl, ''), isPublic
		FROM repos
		WHERE
			ownerId = ?;
	`, ownerId)
	if err != nil {
		return nil, fmt.Errorf("failed to get repos: %w", err)
	}
	return repoIterWrapper{rows}, nil
}

func (db webDb) ArchiveRepo(writeCtx context.Context, ownerId int64, repoId uint64) error {
	r, err := db.GetRepoById(writeCtx, repoId)
	if err != nil {
		return err
	}
	_, err = db.s.Exec(writeCtx, `
		DELETE FROM repos WHERE ownerId = ? AND repoId = ?
	`, ownerId, repoId)
	if err != nil {
		return fmt.Errorf(
			"failed to delete repo ownr=%d repoId=%d: %s",
			ownerId, repoId, err)
	}
	_, err = db.s.Exec(writeCtx, `
		INSERT INTO archived_repos
            (repoId, ownerId, displayName, description, archivedDate, isGitMirrorEnabled, sanitizedGitMirrorUrl, isPublic)
        VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)
	`, repoId, ownerId, r.DisplayName, r.Description, r.IsGitMirrorEnabled,
		r.SanitizedGitMirrorUrl, r.IsPublic)
	if err != nil {
		return fmt.Errorf(
			"failed to insert to archived_repos ownr=%d repoId=%d: %s",
			ownerId, repoId, err)
	}
	return nil
}

func (db webDb) GetArchivedRepoIds(ctx context.Context, ownerId int64) (iterator.I[uint64], error) {
	repoIds, err := db.s.Query(ctx, `
		SELECT
			repoId
		FROM
			archived_repos
		WHERE
			ownerId = ?
	`, ownerId)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get archived ownerId=%d: %s", ownerId, err)
	}
	return repoIdIter{repoIds: repoIds}, nil
}

func (db webDb) SetRepoPublic(writeCtx context.Context,
	ownerId int64, displayName string) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE repos
		SET isPublic = TRUE
		WHERE ownerId = ? AND displayName = ?;
	`, ownerId, displayName)
	return err
}

func (db webDb) SetRepoPrivate(writeCtx context.Context,
	ownerId int64, displayName string) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE repos
		SET isPublic = FALSE
		WHERE ownerId = ? AND displayName = ?;
	`, ownerId, displayName)
	return err
}

func (db webDb) SetRepoDescription(writeCtx context.Context,
	ownerId int64, displayName, description string) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE repos
		SET description = ?
		WHERE ownerId = ? AND displayName = ?;
	`, description, ownerId, displayName)
	return err
}

func (db webDb) SetRepoGitMirrorEnabled(writeCtx context.Context,
	ownerId int64, displayName string, enabled bool) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE repos
		SET isGitMirrorEnabled = ?
		WHERE ownerId = ? AND displayName = ?;
	`, enabled, ownerId, displayName)
	return err
}

func (db webDb) SetRepoSanitizedGitMirrorUrl(writeCtx context.Context,
	ownerId int64, displayName, sanitizedUrl string) error {
	_, err := db.s.Exec(writeCtx, `
		UPDATE repos
		SET sanitizedGitMirrorUrl = ?
		WHERE ownerId = ? AND displayName = ?;
	`, sanitizedUrl, ownerId, displayName)
	return err
}

type repoIterWrapper struct {
	rows *sql.Rows
}

func (it repoIterWrapper) Get() (repo.Repo, error) {
	var repoId uint64
	var repoDisplayName string
	var ownerId int64
	var description string
	var sanitizedGitMirrorUrl string
	var enabled bool
	var isPublic bool

	err := it.rows.Scan(
		&repoId,
		&ownerId,
		&repoDisplayName,
		&description,
		&enabled,
		&sanitizedGitMirrorUrl,
		&isPublic,
	)
	if err != nil {
		return repo.Repo{}, fmt.Errorf("failed to get repo from iter: %s", err)
	}

	return repo.NewRepo(repoId, ownerId, repoDisplayName, description, enabled,
		sanitizedGitMirrorUrl, isPublic), nil
}
func (it repoIterWrapper) Next() bool {
	return it.rows.Next()
}
func (it repoIterWrapper) Err() error {
	return it.rows.Err()
}

type repoIdIter struct {
	repoIds *sql.Rows
}

func (it repoIdIter) Get() (uint64, error) {
	var repoId uint64
	err := it.repoIds.Scan(
		&repoId,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get repoId from iter: %s", err)
	}
	return repoId, nil
}
func (it repoIdIter) Next() bool {
	return it.repoIds.Next()
}
func (it repoIdIter) Err() error {
	return it.repoIds.Err()
}