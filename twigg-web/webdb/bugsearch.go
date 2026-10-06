package webdb

import (
	"context"
	"fmt"
	"math"
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
	c, err := decodeCursor[bugsPageCursor](cursor)
	if err != nil {
		return nil, "", err
	}
	// database/sql rejects uint64 values above MaxInt64.
	before := uint64(math.MaxInt64)
	if c.BeforeNumber != 0 {
		before = c.BeforeNumber
	}

	var q strings.Builder
	q.WriteString(`
		SELECT b.number, b.title, '' AS body, b.status, b.authorId, b.assigneeUserId,
			COALESCE((SELECT count FROM bug_event_counts c WHERE c.bugId = b.bugId AND c.kind = ?), 0),
			b.createdOnUnixMilli, b.updatedOnUnixMilli
		FROM bugs b
		WHERE b.repoId = ? AND b.number < ?`)
	args := []any{bug.EventKind_Comment, f.RepoId, before}

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
	q.WriteString(` ORDER BY b.number DESC LIMIT ?`)
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
		nextCursor = encodeCursor(bugsPageCursor{BeforeNumber: bugs[limit-1].Number})
	}
	return bugs, nextCursor, nil
}
