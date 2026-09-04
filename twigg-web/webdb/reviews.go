package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/services/gobencoding"
	"monorepo/twigg/commit"
)

// The review entities (review.Data, review.Thread, review.Comment) are
// stored as gob-encoded blobs; the review tables only index them.

const reviewDataBlobIdPrefix = "review-blobs"

func reviewDataBlobId(repoId uint64, cId commit.LocalId) string {
	return fmt.Sprintf("%d-%d", repoId, cId)
}

const reviewThreadsBlobIdPrefix = "threads-blobs"

func reviewThreadBlobId(threadId int64) string {
	return fmt.Sprintf("%d", threadId)
}

const reviewCommentsBlobIdPrefix = "comments-blobs"

func reviewCommentBlobId(commentId int64) string {
	return fmt.Sprintf("%d", commentId)
}

func (db webDb) GetReviewData(ctx context.Context, repoId uint64, cId commit.LocalId) (review.Data, error) {
	_, blobRead, closeBlobRead, err := db.blobs.GetBlob(ctx,
		reviewDataBlobIdPrefix, reviewDataBlobId(repoId, cId))
	if err != nil {
		closeBlobRead()
		return review.Data{}, err
	}
	return gobencoding.ReadIntoStruct[review.Data](blobRead, closeBlobRead)
}

func (db webDb) SetReviewData(writeCtx context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, d review.Data) error {
	_, err := db.setBlob(writeCtx, quotaOwner,
		reviewDataBlobIdPrefix, reviewDataBlobId(repoId, cId), gobencoding.StructWriterTo(d))
	return err
}

func (db webDb) GetReviewThread(ctx context.Context, threadId int64) (review.Thread, error) {
	_, blobRead, closeBlobRead, err := db.blobs.GetBlob(ctx,
		reviewThreadsBlobIdPrefix, reviewThreadBlobId(threadId))
	if err != nil {
		closeBlobRead()
		return review.Thread{}, err
	}
	return gobencoding.ReadIntoStruct[review.Thread](blobRead, closeBlobRead)
}

func (db webDb) SetReviewThread(writeCtx context.Context, quotaOwner string, threadId int64, th review.Thread) error {
	_, err := db.setBlob(writeCtx, quotaOwner,
		reviewThreadsBlobIdPrefix, reviewThreadBlobId(threadId), gobencoding.StructWriterTo(th))
	return err
}

func (db webDb) GetReviewComment(ctx context.Context, commentId int64) (review.Comment, error) {
	_, blobRead, closeBlobRead, err := db.blobs.GetBlob(ctx,
		reviewCommentsBlobIdPrefix, reviewCommentBlobId(commentId))
	if err != nil {
		closeBlobRead()
		return review.Comment{}, err
	}
	return gobencoding.ReadIntoStruct[review.Comment](blobRead, closeBlobRead)
}

func (db webDb) SetReviewComment(writeCtx context.Context, quotaOwner string, commentId int64, cm review.Comment) error {
	_, err := db.setBlob(writeCtx, quotaOwner,
		reviewCommentsBlobIdPrefix, reviewCommentBlobId(commentId), gobencoding.StructWriterTo(cm))
	return err
}

func (db webDb) CreateReviewIfNotExists(writeCtx context.Context, repoId uint64, cId commit.LocalId) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO reviews (repoId, commitId)
		VALUES (?, ?)
		ON CONFLICT (repoId, commitId) DO NOTHING;
	`, repoId, cId)
	if err != nil {
		return fmt.Errorf("failed to insert into reviews: %s", err)
	}
	return nil
}

func (db webDb) HasReview(ctx context.Context, repoId uint64, cId commit.LocalId) (bool, error) {
	var dummyVar string
	err := db.s.QueryRow(ctx, `
		SELECT
			1
		FROM
			reviews
		WHERE
			repoId = ? AND commitId = ?
	`, repoId, cId).Scan(&dummyVar)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (db webDb) CreateReviewThread(writeCtx context.Context, repoId uint64, cId commit.LocalId,
	authorId int64, threadType uint32) (threadId int64, err error) {
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO threads (repoId, commitId, authorId, threadType)
		VALUES (?, ?, ?, ?)
		RETURNING id;
	`, repoId, cId, authorId, threadType).Scan(&threadId)
	return
}

func (db webDb) GetReviewThreadIds(ctx context.Context, repoId uint64, cId commit.LocalId) (iterator.I[int64], error) {
	rows, err := db.s.Query(ctx, `
		SELECT
			id
		FROM
			threads
		WHERE
			repoId = ? AND commitId = ?
	`, repoId, cId)
	if err != nil {
		return nil, err
	}
	return idIterWrapper{rows}, nil
}

func (db webDb) GetReviewUserLastLgtmThreadId(ctx context.Context, repoId uint64, cId commit.LocalId,
	userId int64, addLgtmType, removeLgtmType uint32) (threadId int64, isNotFoundErr bool, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT
			id
		FROM
			threads
		WHERE
	 		repoId = ? AND commitId = ? AND
				authorId = ? AND threadType IN (?, ?)
		ORDER BY id DESC
		LIMIT 1;
	`, repoId, cId, userId, addLgtmType, removeLgtmType).Scan(&threadId)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, true, ErrNotFound
	}
	return threadId, false, err
}

func (db webDb) GetReviewLgtmAuthorIds(ctx context.Context, repoId uint64, cId commit.LocalId,
	addLgtmType, removeLgtmType uint32) (iterator.I[int64], error) {
	// Count is odd -> last was LGTM -> current status is LGTM'd
	// Count is even -> lst was an LGTM removal -> curent status is not LGTM'd
	rows, err := db.s.Query(ctx, `
        SELECT
            authorId
        FROM
            threads
        WHERE
            repoId = ? AND
            commitId = ? AND
            threadType IN (?, ?)
        GROUP BY
            authorId
        HAVING
            (COUNT(*) % 2) = 1;
    `, repoId, cId, addLgtmType, removeLgtmType)
	if err != nil {
		return nil, fmt.Errorf("failed to get LGTM authors: %w", err)
	}
	return idIterWrapper{rows}, nil
}

func (db webDb) CreateReviewComment(writeCtx context.Context, repoId uint64, cId commit.LocalId,
	threadId int64, authorId int64) (commentId int64, err error) {
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO comments (repoId, commitId, threadId, authorId)
		VALUES (?, ?, ?, ?)
		RETURNING id;
	`, repoId, cId, threadId, authorId).Scan(&commentId)
	return
}

func (db webDb) GetReviewCommentIds(ctx context.Context, repoId uint64, cId commit.LocalId,
	threadId int64) (iterator.I[int64], error) {
	rows, err := db.s.Query(ctx, `
		SELECT
			id
		FROM
			comments
		WHERE
			repoId = ? AND commitId = ? AND threadId = ?
	`, repoId, cId, threadId)
	if err != nil {
		return nil, err
	}
	return idIterWrapper{rows}, nil
}

type idIterWrapper struct {
	rows *sql.Rows
}

func (it idIterWrapper) Get() (int64, error) {
	var id int64
	err := it.rows.Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get id from iter: %s", err)
	}
	return id, nil
}
func (it idIterWrapper) Next() bool { return it.rows.Next() }
func (it idIterWrapper) Err() error { return it.rows.Err() }
