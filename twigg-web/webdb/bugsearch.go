package webdb

import (
	"context"
	"fmt"
	"monorepo/twigg-web/bug"
	"monorepo/twigg-web/bugsearch"
	"strings"
)

// Queries the existing bugs table directly - there is no search index, so
// this only supports the structured filters bugsearch.Filter holds.
func (db webDb) searchBugs(ctx context.Context, f bugsearch.Filter, cursor string, limit int) (
	bugs []bug.Bug, nextCursor string, err error) {
	if limit <= 0 {
		return nil, "", fmt.Errorf("invalid limit %d", limit)
	}
	c, err := decodeCursor[bugSearchCursor](cursor)
	if err != nil {
		return nil, "", err
	}

	var q strings.Builder
	q.WriteString(`
		SELECT b.number, b.title, '' AS body, b.status, b.authorId, b.assigneeUserId, b.priority,
			COALESCE((SELECT count FROM bug_event_counts c WHERE c.bugId = b.bugId AND c.kind = ?), 0),
			b.createdOnUnixMilli, b.updatedOnUnixMilli
		FROM bugs b
		WHERE b.repoId = ?`)
	args := []any{bug.EventKind_Comment, f.RepoId}
	if cursor != "" {
		q.WriteString(` AND (b.updatedOnUnixMilli, b.number) < (?, ?)`)
		args = append(args, c.BeforeUpdatedOnUnixMilli, c.BeforeNumber)
	}

	if f.Status != "" {
		q.WriteString(` AND b.status = ?`)
		args = append(args, f.Status)
	}
	if f.AssigneeUnassigned {
		q.WriteString(` AND b.assigneeUserId = 0`)
	} else if f.AssigneeUsername != "" {
		q.WriteString(` AND b.assigneeUserId = (
			SELECT u.id FROM users2 u WHERE u.username = ?)`)
		args = append(args, f.AssigneeUsername)
	}
	if f.HasPriority {
		q.WriteString(` AND b.priority = ?`)
		args = append(args, f.Priority)
	}
	q.WriteString(` ORDER BY b.updatedOnUnixMilli DESC, b.number DESC LIMIT ?`)
	args = append(args, limit+1)

	rows, err := db.s.Query(ctx, q.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("failed searching bugs (repoId=%v): %w", f.RepoId, err)
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
		last := bugs[limit-1]
		nextCursor = encodeCursor(bugSearchCursor{
			BeforeUpdatedOnUnixMilli: last.UpdatedOn.UnixMilli(),
			BeforeNumber:             last.Number,
		})
	}
	return bugs, nextCursor, nil
}

// The bug number breaks ties between bugs updated in the same millisecond.
type bugSearchCursor struct {
	BeforeUpdatedOnUnixMilli int64
	BeforeNumber             uint64
}
