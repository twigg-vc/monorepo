package webdb

import (
	"context"
	"encoding/base64"
	"fmt"
	"monorepo/base/iterator"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/services/gobencoding"
	"monorepo/twigg/commit"
	"strings"
)

func (db webDb) searchCommits(r context.Context, f commitsearch.Filter, cursor string, limit int) (
	[]commit.Commit, string, error) {
	hasCursor := cursor != ""
	var parsedCursor searchCommitsCursor
	if hasCursor {
		var err error
		parsedCursor, err = decodeSearchCommitsCursor(cursor)
		if err != nil {
			return nil, "", err
		}
	}

	var q strings.Builder
	q.WriteString(`
		SELECT s.commitId FROM twigg_commit_search s
		WHERE s.repoId = ?`)
	args := []any{f.RepoId}

	if f.State == commitsearch.StatePending {
		q.WriteString(` AND s.isSubmitted = 0`)
	}
	if f.State == commitsearch.StateSubmitted {
		q.WriteString(` AND s.isSubmitted = 1`)
	}
	if f.Wip != commitsearch.PresenceIgnore {
		q.WriteString(` AND s.isWip = ?`)
		args = append(args, f.Wip == commitsearch.PresenceRequire)
	}
	if f.Archived != commitsearch.PresenceIgnore {
		q.WriteString(` AND s.isArchived = ?`)
		args = append(args, f.Archived == commitsearch.PresenceRequire)
	}
	if hasCursor {
		q.WriteString(` AND s.commitId < ?`)
		args = append(args, parsedCursor.CommitId)
	}
	q.WriteString(` ORDER BY s.commitId DESC LIMIT ?`)
	args = append(args, limit)

	rows, err := db.s.Query(r, q.String(), args...)
	if err != nil {
		return nil, "", err
	}
	cIter := commitIter{db: db, ctx: r, repoId: f.RepoId, commitIds: rows}

	commits, err := iterator.GetFirstN(limit, cIter)
	if err != nil {
		return nil, "", err
	}
	if len(commits) == 0 {
		return commits, "", nil
	}
	nextCursor := searchCommitsCursor{
		CommitId: commits[len(commits)-1].L,
	}
	return commits, nextCursor.encode(), nil
}

type searchCommitsCursor struct {
	CommitId commit.LocalId
}

func (c searchCommitsCursor) encode() string {
	return base64.RawURLEncoding.EncodeToString(gobencoding.Encode(c))
}
func decodeSearchCommitsCursor(cursor string) (searchCommitsCursor, error) {
	if cursor == "" {
		panic("decodeSearchCommitsCursor called with empty cursor")
	}
	encoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return searchCommitsCursor{}, fmt.Errorf("bad cursor: %w", err)
	}
	c, err := gobencoding.Decode[searchCommitsCursor](encoded)
	if err != nil {
		return searchCommitsCursor{}, fmt.Errorf("bad cursor: %w", err)
	}
	return c, nil
}
