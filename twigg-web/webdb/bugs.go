package webdb

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/services/gobencoding"
	"slices"
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
	now := db.getNow().UnixMilli()
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

func (db webDb) AddBugComment(writeCtx context.Context, repoId uint64, number uint64,
	authorId int64, body string) (e bug.Event, isNotFoundErr bool, err error) {
	if body == "" {
		return bug.Event{}, false, fmt.Errorf("missing body")
	}
	e, isNotFoundErr, err = db.insertBugEvent(writeCtx, repoId, number,
		bug.EventKind_Comment, authorId)
	if err != nil {
		return bug.Event{}, isNotFoundErr, err
	}
	_, err = db.s.Exec(writeCtx, `
		INSERT INTO bug_comments (eventId, body) VALUES (?, ?)
	`, e.Id, body)
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed inserting bug comment: %w", err)
	}
	e.Comment = bug.NewComment(body)
	return e, false, nil
}

// Also bumps the bug's updatedOn. The caller must insert the kind's details.
func (db webDb) insertBugEvent(writeCtx context.Context, repoId uint64, number uint64,
	kind bug.EventKind, authorId int64) (e bug.Event, isNotFoundErr bool, err error) {
	now := db.getNow().UnixMilli()
	var bugId uint64
	err = db.s.QueryRow(writeCtx, `
		UPDATE bugs SET updatedOnUnixMilli = ?
		WHERE repoId = ? AND number = ?
		RETURNING bugId
	`, now, repoId, number).Scan(&bugId)
	if errors.Is(err, sql.ErrNoRows) {
		return bug.Event{}, true, ErrNotFound
	}
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed touching bug (repoId=%v number=%v): %w", repoId, number, err)
	}
	var eventId uint64
	err = db.s.QueryRow(writeCtx, `
		INSERT INTO bug_events (bugId, kind, authorId, createdOnUnixMilli)
		VALUES (?, ?, ?, ?)
		RETURNING eventId
	`, bugId, kind, authorId, now).Scan(&eventId)
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed inserting bug event (bugId=%v): %w", bugId, err)
	}
	return bug.NewEvent(eventId, kind, authorId, time.UnixMilli(now).UTC()), false, nil
}

func (db webDb) GetBugEvents(ctx context.Context, repoId uint64, number uint64,
	cursor string, limit int) (events []bug.Event, nextCursor string, err error) {
	if limit <= 0 {
		return nil, "", fmt.Errorf("invalid limit %d", limit)
	}
	c, err := decodeBugEventsCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	// database/sql rejects uint64 values above MaxInt64.
	before := uint64(math.MaxInt64)
	if c.BeforeEventId != 0 {
		before = c.BeforeEventId
	}
	rows, err := db.s.Query(ctx, `
		SELECT e.eventId, e.kind, e.authorId, e.createdOnUnixMilli,
			COALESCE(c.body, '')
		FROM bug_events e
		LEFT JOIN bug_comments c ON c.eventId = e.eventId
		WHERE e.bugId = (SELECT bugId FROM bugs WHERE repoId = ? AND number = ?)
			AND e.eventId < ?
		ORDER BY e.eventId DESC
		LIMIT ?
	`, repoId, number, before, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("failed getting bug events (repoId=%v number=%v): %w", repoId, number, err)
	}
	defer rows.Close()
	events = []bug.Event{}
	for rows.Next() {
		var e bug.Event
		var createdOn int64
		var commentBody string
		err := rows.Scan(&e.Id, &e.Kind, &e.AuthorUserId, &createdOn, &commentBody)
		if err != nil {
			return nil, "", fmt.Errorf("failed scanning bug event: %w", err)
		}
		e.CreatedOn = time.UnixMilli(createdOn).UTC()
		switch e.Kind {
		case bug.EventKind_Comment:
			e.Comment = bug.NewComment(commentBody)
		default:
			return nil, "", fmt.Errorf("unknown bug event kind %d (eventId=%d)", e.Kind, e.Id)
		}
		events = append(events, e)
	}
	err = rows.Err()
	if err != nil {
		return nil, "", fmt.Errorf("failed iterating bug events: %w", err)
	}
	if len(events) > limit {
		events = events[:limit]
		nextCursor = bugEventsCursor{BeforeEventId: events[limit-1].Id}.encode()
	}
	slices.Reverse(events)
	return events, nextCursor, nil
}

type bugEventsCursor struct {
	BeforeEventId uint64
}

func (c bugEventsCursor) encode() string {
	return base64.RawURLEncoding.EncodeToString(gobencoding.Encode(c))
}

// An empty cursor starts at the newest event.
func decodeBugEventsCursor(cursor string) (bugEventsCursor, error) {
	if cursor == "" {
		return bugEventsCursor{}, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return bugEventsCursor{}, fmt.Errorf("bad cursor: %w", err)
	}
	c, err := gobencoding.Decode[bugEventsCursor](encoded)
	if err != nil {
		return bugEventsCursor{}, fmt.Errorf("bad cursor: %w", err)
	}
	return c, nil
}
