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
		&b.AssigneeUserId, &b.CommentCount, &createdOn, &updatedOn)
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
	err = db.addRepoBugCount(writeCtx, repoId, bug.Status_Open, 1)
	if err != nil {
		return bug.Bug{}, err
	}
	b, _, err := db.GetBug(writeCtx, repoId, number)
	return b, err
}

func (db webDb) addRepoBugCount(writeCtx context.Context, repoId uint64,
	status bug.Status, delta int64) error {
	_, err := db.s.Exec(writeCtx, `
		INSERT INTO repo_bug_counts (repoId, status, count) VALUES (?, ?, ?)
		ON CONFLICT (repoId, status) DO UPDATE SET count = count + excluded.count
	`, repoId, status, delta)
	if err != nil {
		return fmt.Errorf("failed counting %s bugs (repoId=%v): %w", status, repoId, err)
	}
	return nil
}

func (db webDb) CountRepoBugs(ctx context.Context, repoId uint64) (open, closed int64, err error) {
	err = db.s.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT count FROM repo_bug_counts WHERE repoId = ? AND status = ?), 0),
			COALESCE((SELECT count FROM repo_bug_counts WHERE repoId = ? AND status = ?), 0)
	`, repoId, bug.Status_Open, repoId, bug.Status_Closed).Scan(&open, &closed)
	if err != nil {
		return 0, 0, fmt.Errorf("failed counting bugs (repoId=%v): %w", repoId, err)
	}
	return open, closed, nil
}

func (db webDb) GetBug(ctx context.Context, repoId uint64, number uint64) (
	b bug.Bug, isNotFoundErr bool, err error) {
	row := db.s.QueryRow(ctx, `
		SELECT b.number, b.title, b.body, b.status, b.authorId, b.assigneeUserId,
			COALESCE((SELECT count FROM bug_event_counts c WHERE c.bugId = b.bugId AND c.kind = ?), 0),
			b.createdOnUnixMilli, b.updatedOnUnixMilli
		FROM bugs b
		WHERE b.repoId = ? AND b.number = ?
	`, bug.EventKind_Comment, repoId, number)
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

func (db webDb) GetBugsPage(ctx context.Context, repoId uint64, status bug.Status,
	cursor string, limit int) (bugs []bug.Bug, nextCursor string, err error) {
	if limit <= 0 {
		return nil, "", fmt.Errorf("invalid limit %d", limit)
	}
	c, err := decodeCursor[bugsPageCursor](cursor)
	if err != nil {
		return nil, "", err
	}
	// database/sql rejects uint64 values above MaxInt64.
	before := uint64(math.MaxInt64)
	if c.BeforeNumber != 0 {
		before = c.BeforeNumber
	}
	const selectBugsWithoutBody = `
		SELECT b.number, b.title, '' AS body, b.status, b.authorId, b.assigneeUserId,
			COALESCE((SELECT count FROM bug_event_counts c WHERE c.bugId = b.bugId AND c.kind = ?), 0),
			b.createdOnUnixMilli, b.updatedOnUnixMilli
		FROM bugs b
	`
	// Two static queries, so SQLite always picks an index.
	var rows *sql.Rows
	if status == "" {
		rows, err = db.s.Query(ctx, selectBugsWithoutBody+`
			WHERE b.repoId = ? AND b.number < ?
			ORDER BY b.number DESC
			LIMIT ?
		`, bug.EventKind_Comment, repoId, before, limit+1)
	} else {
		rows, err = db.s.Query(ctx, selectBugsWithoutBody+`
			WHERE b.repoId = ? AND b.status = ? AND b.number < ?
			ORDER BY b.number DESC
			LIMIT ?
		`, bug.EventKind_Comment, repoId, status, before, limit+1)
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed getting bugs page (repoId=%v): %w", repoId, err)
	}
	defer rows.Close()
	bugs = []bug.Bug{}
	for rows.Next() {
		b, err := scanBug(rows)
		if err != nil {
			return nil, "", fmt.Errorf("failed scanning bug: %w", err)
		}
		bugs = append(bugs, b)
	}
	err = rows.Err()
	if err != nil {
		return nil, "", fmt.Errorf("failed iterating bugs: %w", err)
	}
	if len(bugs) > limit {
		bugs = bugs[:limit]
		nextCursor = encodeCursor(bugsPageCursor{BeforeNumber: bugs[limit-1].Number})
	}
	return bugs, nextCursor, nil
}

func (db webDb) SetBugStatus(writeCtx context.Context, repoId uint64, number uint64,
	authorId int64, status bug.Status) (e bug.Event, isNotFoundErr bool, err error) {
	if !status.IsValid() {
		return bug.Event{}, false, fmt.Errorf("invalid status %q", status)
	}
	e, isNotFoundErr, err = db.insertBugEvent(writeCtx, repoId, number,
		bug.EventKind_StatusChange, authorId)
	if err != nil {
		return bug.Event{}, isNotFoundErr, err
	}
	_, err = db.s.Exec(writeCtx, `
		INSERT INTO bug_status_changes (eventId, newStatus) VALUES (?, ?)
	`, e.Id, status)
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed inserting bug status change: %w", err)
	}
	var oldStatus bug.Status
	err = db.s.QueryRow(writeCtx, `
		SELECT status FROM bugs WHERE repoId = ? AND number = ?
	`, repoId, number).Scan(&oldStatus)
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed getting bug status (repoId=%v number=%v): %w", repoId, number, err)
	}
	_, err = db.s.Exec(writeCtx, `
		UPDATE bugs SET status = ? WHERE repoId = ? AND number = ?
	`, status, repoId, number)
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed setting bug status (repoId=%v number=%v): %w", repoId, number, err)
	}
	if oldStatus != status {
		err = db.addRepoBugCount(writeCtx, repoId, oldStatus, -1)
		if err != nil {
			return bug.Event{}, false, err
		}
		err = db.addRepoBugCount(writeCtx, repoId, status, 1)
		if err != nil {
			return bug.Event{}, false, err
		}
	}
	e.StatusChange = bug.NewStatusChange(status)
	return e, false, nil
}

// Also bumps the bug's updatedOn and its count of the kind. The caller must
// insert the kind's details.
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
	_, err = db.s.Exec(writeCtx, `
		INSERT INTO bug_event_counts (bugId, kind, count) VALUES (?, ?, 1)
		ON CONFLICT (bugId, kind) DO UPDATE SET count = count + 1
	`, bugId, kind)
	if err != nil {
		return bug.Event{}, false, fmt.Errorf("failed counting bug event (bugId=%v): %w", bugId, err)
	}
	return bug.NewEvent(eventId, kind, authorId, time.UnixMilli(now).UTC()), false, nil
}

func (db webDb) GetBugEvents(ctx context.Context, repoId uint64, number uint64,
	cursor string, limit int) (events []bug.Event, nextCursor string, err error) {
	if limit <= 0 {
		return nil, "", fmt.Errorf("invalid limit %d", limit)
	}
	c, err := decodeCursor[bugEventsCursor](cursor)
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
			COALESCE(c.body, ''),
			COALESCE(s.newStatus, '')
		FROM bug_events e
		LEFT JOIN bug_comments c ON c.eventId = e.eventId
		LEFT JOIN bug_status_changes s ON s.eventId = e.eventId
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
		var newStatus bug.Status
		err := rows.Scan(&e.Id, &e.Kind, &e.AuthorUserId, &createdOn, &commentBody, &newStatus)
		if err != nil {
			return nil, "", fmt.Errorf("failed scanning bug event: %w", err)
		}
		e.CreatedOn = time.UnixMilli(createdOn).UTC()
		switch e.Kind {
		case bug.EventKind_Comment:
			e.Comment = bug.NewComment(commentBody)
		case bug.EventKind_StatusChange:
			e.StatusChange = bug.NewStatusChange(newStatus)
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
		nextCursor = encodeCursor(bugEventsCursor{BeforeEventId: events[limit-1].Id})
	}
	slices.Reverse(events)
	return events, nextCursor, nil
}

type bugEventsCursor struct {
	BeforeEventId uint64
}

type bugsPageCursor struct {
	BeforeNumber uint64
}

func encodeCursor[C any](c C) string {
	return base64.RawURLEncoding.EncodeToString(gobencoding.Encode(c))
}

// An empty cursor decodes to the zero C, which starts at the newest.
func decodeCursor[C any](cursor string) (C, error) {
	var zero C
	if cursor == "" {
		return zero, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return zero, fmt.Errorf("bad cursor: %w", err)
	}
	c, err := gobencoding.Decode[C](encoded)
	if err != nil {
		return zero, fmt.Errorf("bad cursor: %w", err)
	}
	return c, nil
}
