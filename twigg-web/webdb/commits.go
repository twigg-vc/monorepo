package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/data/blobdb"
	"monorepo/twigg-web/services/gobencoding"
	"monorepo/twigg/commit"
)

const commitBlobIdPrefix = "twigg-internal-commit-blobs"

func commitBlobId(repoId uint64, commitId uint64, commitVersion uint64) string {
	return fmt.Sprintf("%d-%d-%d", repoId, commitId, commitVersion)
}

func (db webDb) SetCommit(ctx context.Context, quotaOwner string, repoId uint64, c commit.Commit) (err error) {
	_, err = db.setBlob(ctx, quotaOwner, commitBlobIdPrefix,
		commitBlobId(repoId, c.L, c.Version), gobencoding.StructWriterTo(c))
	if err != nil {
		return
	}
	_, err = db.s.Exec(ctx, `
		INSERT INTO twigg_commits
			(repoId, commitId, commitVersion, authorId, isSubmitted,
			hasServerCommitId, serverCommitId, hasServerCommitVersion, serverCommitVersion)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repoId, commitId, commitVersion)
		DO UPDATE SET
			authorId = EXCLUDED.authorId,
			isSubmitted = EXCLUDED.isSubmitted,
			hasServerCommitId = EXCLUDED.hasServerCommitId,
			serverCommitId = EXCLUDED.serverCommitId,
			hasServerCommitVersion = EXCLUDED.hasServerCommitVersion,
			serverCommitVersion = EXCLUDED.serverCommitVersion
	`, repoId, c.L, c.Version, c.AuthorUserId, c.IsSubmitted,
		c.HasServerL, c.ServerL, c.HasServerV, c.ServerV)
	if err != nil {
		return
	}

	// Commits are indexed by their parent so that the children of a commit
	// can be listed. This can be read from the commit struct bc the server
	// strips the children of the struct to ensure that struct remains small
	isRoot := c.L == commit.RootCommitId
	if !isRoot && !c.IsDetached {
		_, err = db.s.Exec(ctx, `
			INSERT INTO twigg_commit_children
				(repoId, commitId, commitVersion,
				childCommitId, childCommitVersion)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(repoId, commitId, commitVersion,
				childCommitId, childCommitVersion) DO NOTHING
		`, repoId, c.ParentL, c.ParentV, c.L, c.Version)
		if err != nil {
			return
		}
	}

	if c.IsSubmitted {
		_, err = db.s.Exec(ctx, `DELETE FROM twigg_pending_commits WHERE repoId = ? AND commitId = ?`, repoId, c.L)
		if err != nil {
			return
		}
		_, err = db.s.Exec(ctx, `
			INSERT INTO twigg_submitted_commits (repoId, commitId, authorId)
			VALUES (?, ?, ?) ON CONFLICT(repoId, commitId) DO NOTHING
		`, repoId, c.L, c.AuthorUserId)
		if err != nil {
			return
		}
	}

	// Only add to "pending commits" if another (future) version hasn't
	// already been submitted
	var dummyVar int64
	err = db.s.QueryRow(ctx, `
		SELECT 1 FROM twigg_submitted_commits WHERE repoId = ? AND commitId = ?
	`, repoId, c.L).Scan(&dummyVar)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}
	hasSubmittedVersion := !errors.Is(err, sql.ErrNoRows)
	if !c.IsSubmitted && !hasSubmittedVersion {
		_, err = db.s.Exec(ctx, `
			INSERT INTO twigg_pending_commits (repoId, commitId, authorId)
			VALUES (?, ?, ?) ON CONFLICT(repoId, commitId, authorId) DO NOTHING
		`, repoId, c.L, c.AuthorUserId)
	}
	return
}

func (db webDb) GetLatestCommitByLocalId(ctx context.Context, repoId uint64, L uint64) (c commit.Commit, isNotFoundErr bool, err error) {
	var latestV uint64
	err = db.s.QueryRow(ctx, `
		SELECT commitVersion FROM twigg_commits
		WHERE repoId = ? AND commitId = ? ORDER BY commitVersion DESC LIMIT 1
	`, repoId, L).Scan(&latestV)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		return
	}
	_, r, closeR, err := db.blobs.GetBlob(ctx, commitBlobIdPrefix, commitBlobId(repoId, L, latestV))
	if errors.Is(err, blobdb.ErrNotFound) {
		closeR()
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		closeR()
		return
	}
	c, err = gobencoding.ReadIntoStruct[commit.Commit](r, closeR)
	return
}

func (db webDb) GetCommitVersionByLocalId(ctx context.Context, repoId uint64, L uint64, v uint64) (c commit.Commit, isNotFoundErr bool, err error) {
	_, r, closeR, err := db.blobs.GetBlob(ctx, commitBlobIdPrefix, commitBlobId(repoId, L, v))
	if errors.Is(err, blobdb.ErrNotFound) {
		closeR()
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		closeR()
		return
	}
	c, err = gobencoding.ReadIntoStruct[commit.Commit](r, closeR)
	return
}

func (db webDb) GetLatestCommitByServerId(ctx context.Context, repoId uint64, ServerL uint64) (c commit.Commit, isNotFoundErr bool, err error) {
	var commitId uint64
	err = db.s.QueryRow(ctx, `
		SELECT commitId FROM twigg_commits
		WHERE repoId = ? AND hasServerCommitId = TRUE AND serverCommitId = ?
	`, repoId, ServerL).Scan(&commitId)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		return
	}
	return db.GetLatestCommitByLocalId(ctx, repoId, commitId)
}

func (db webDb) GetCommitVersionByServerId(ctx context.Context, repoId uint64, ServerL uint64, ServerV uint64) (c commit.Commit, isNotFoundErr bool, err error) {
	var commitId, commitVersion uint64
	err = db.s.QueryRow(ctx, `
		SELECT commitId, commitVersion FROM twigg_commits
		WHERE repoId = ?
			AND hasServerCommitId = TRUE AND serverCommitId = ?
			AND hasServerCommitVersion = TRUE AND serverCommitVersion = ?
	`, repoId, ServerL, ServerV).Scan(&commitId, &commitVersion)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		isNotFoundErr = true
		return
	}
	if err != nil {
		return
	}
	return db.GetCommitVersionByLocalId(ctx, repoId, commitId, commitVersion)
}

func (db webDb) GetCommitChildren(ctx context.Context, repoId uint64,
	commitL uint64, commitV uint64, maxRowsToRead int) (
	children []commit.LocalId, childrenVersions []uint64, err error) {
	// Read an extra row to detect that there are more children than
	// the caller is willing to read
	rows, err := db.s.Query(ctx, `
		SELECT childCommitId, childCommitVersion FROM twigg_commit_children
		WHERE repoId = ? AND commitId = ? AND commitVersion = ?
		ORDER BY childCommitId ASC, childCommitVersion ASC
		LIMIT ?
	`, repoId, commitL, commitV, maxRowsToRead+1)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var childL uint64
		var childV uint64
		err = rows.Scan(&childL, &childV)
		if err != nil {
			return
		}
		if len(children) == maxRowsToRead {
			err = fmt.Errorf("commit %dv%d has more than %d children",
				commitL, commitV, maxRowsToRead)
			return
		}
		children = append(children, childL)
		childrenVersions = append(childrenVersions, childV)
	}
	err = rows.Err()
	return
}

func (db webDb) GetPendingCommits(ctx context.Context, ascendingOrder bool, repoId uint64) (iterator.I[commit.Commit], error) {
	var rows *sql.Rows
	var err error
	if ascendingOrder {
		rows, err = db.s.Query(ctx, `
			SELECT commitId FROM twigg_pending_commits
			WHERE repoId = ? GROUP BY commitId ORDER BY commitId ASC
		`, repoId)
	} else {
		rows, err = db.s.Query(ctx, `
			SELECT commitId FROM twigg_pending_commits
			WHERE repoId = ? GROUP BY commitId ORDER BY commitId DESC
		`, repoId)
	}
	if err != nil {
		return nil, err
	}
	return commitIter{db: db, ctx: ctx, repoId: repoId, commitIds: rows}, nil
}

// Descending order
func (db webDb) GetPendingCommitsAfter(ctx context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	rows, err := db.s.Query(ctx, `
		SELECT commitId FROM twigg_pending_commits
		WHERE repoId = ? AND commitId < ? GROUP BY commitId ORDER BY commitId DESC
	`, repoId, afterId)
	if err != nil {
		return nil, err
	}
	return commitIter{db: db, ctx: ctx, repoId: repoId, commitIds: rows}, nil
}

type commitIter struct {
	db        webDb
	ctx       context.Context
	repoId    uint64
	commitIds *sql.Rows
}

func (ci commitIter) Get() (commit.Commit, error) {
	var commitId uint64
	err := ci.commitIds.Scan(&commitId)
	if err != nil {
		return commit.Commit{}, err
	}
	c, _, err := ci.db.GetLatestCommitByLocalId(ci.ctx, ci.repoId, commitId)
	return c, err
}
func (ci commitIter) Next() bool { return ci.commitIds.Next() }
func (ci commitIter) Err() error { return ci.commitIds.Err() }