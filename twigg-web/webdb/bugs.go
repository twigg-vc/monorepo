package webdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monorepo/twigg-web/bug"
	"time"
)

func scanBug(s rowScanner) (bug.Bug, error) {
	var b bug.Bug
	var createdOn, updatedOn int64
	err := s.Scan(&b.Number, &b.Title, &b.Body, &b.Status, &b.AuthorUserId,
		&b.AssigneeUserId, &createdOn, &updatedOn)
	if err != nil {
		return bug.Bug{}, err
	}
	b.CreatedOn = time.UnixMilli(createdOn).UTC()
	b.UpdatedOn = time.UnixMilli(updatedOn).UTC()
	return b, nil
}

func (db webDb) CreateBug(writeCtx context.Context, repoId uint64, authorId int64,
	title, body string) (bug.Bug, error) {
	if title == "" {
		return bug.Bug{}, fmt.Errorf("missing title")
	}
	now := time.Now().UnixMilli()
	var number uint64
	err := db.s.QueryRow(writeCtx, `
		INSERT INTO bugs (repoId, number, authorId, title, body, status,
			createdOnUnixMilli, updatedOnUnixMilli)
		SELECT ?, COALESCE(MAX(number), 0) + 1, ?, ?, ?, ?, ?, ?
		FROM bugs WHERE repoId = ?
		RETURNING number
	`, repoId, authorId, title, body, bug.Status_Open, now, now, repoId).Scan(&number)
	if err != nil {
		return bug.Bug{}, fmt.Errorf("failed inserting bug (repoId=%v): %w", repoId, err)
	}
	b, _, err := db.GetBug(writeCtx, repoId, number)
	return b, err
}

func (db webDb) GetBug(ctx context.Context, repoId uint64, number uint64) (
	b bug.Bug, isNotFoundErr bool, err error) {
	row := db.s.QueryRow(ctx, `
		SELECT number, title, body, status, authorId, assigneeUserId,
			createdOnUnixMilli, updatedOnUnixMilli
		FROM bugs
		WHERE repoId = ? AND number = ?
	`, repoId, number)
	b, err = scanBug(row)
	if errors.Is(err, sql.ErrNoRows) {
		return bug.Bug{}, true, ErrNotFound
	}
	if err != nil {
		return bug.Bug{}, false, fmt.Errorf("failed getting bug (repoId=%v number=%v): %w", repoId, number, err)
	}
	return b, false, nil
}
