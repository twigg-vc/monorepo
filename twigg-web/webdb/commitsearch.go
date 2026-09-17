package webdb

import (
	"context"
	"encoding/base64"
	"fmt"
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

	// A text search is driven by the text index and ordered by its rows.
	// Since commit versions are saved incrementally, this should be equivalent
	// to ordering by commit anyway.
	// Any other search is driven by the commits themselves.
	// This is done because fts5 is efficient for text macthing, but any other
	// "order by" is O(number of matches), which would make queries like "fix"
	// be very inneficient (as basically all commits would be read).
	match := ftsMatchQuery(f.Message)
	var q strings.Builder
	var args []any
	if match != "" {
		q.WriteString(`
			SELECT t.commitId, t.rowid FROM commit_search_text t
			JOIN twigg_commit_search s
				ON s.repoId = t.repoId AND s.commitId = t.commitId
			WHERE t.repoId = ? AND commit_search_text MATCH ?`)
		args = append(args, f.RepoId, match)
		if hasCursor {
			q.WriteString(` AND t.rowid < ?`)
			args = append(args, parsedCursor.TextSearchRowId)
		}
	} else {
		q.WriteString(`
			SELECT s.commitId, 0 FROM twigg_commit_search s
			WHERE s.repoId = ?`)
		args = append(args, f.RepoId)
		if hasCursor {
			q.WriteString(` AND s.commitId < ?`)
			args = append(args, parsedCursor.CommitId)
		}
	}

	if f.AuthorId != 0 {
		q.WriteString(` AND s.authorId = ?`)
		args = append(args, f.AuthorId)
	}
	if f.ReviewerId != 0 {
		q.WriteString(` AND EXISTS (
			SELECT 1 FROM review_reviewers rr
			WHERE rr.repoId = s.repoId AND rr.commitId = s.commitId
				AND rr.userId = ?)`)
		args = append(args, f.ReviewerId)
	}
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
	if match != "" {
		q.WriteString(` ORDER BY t.rowid DESC LIMIT ?`)
	} else {
		q.WriteString(` ORDER BY s.commitId DESC LIMIT ?`)
	}
	args = append(args, limit)

	// Parse the query results
	queryResults, err := db.getSearchedCommits(r, q.String(), args)
	if err != nil {
		return nil, "", err
	}
	if len(queryResults) == 0 {
		return nil, "", nil
	}
	// Read the actual commit blobs
	commits := make([]commit.Commit, 0, len(queryResults))
	for _, s := range queryResults {
		c, _, err := db.GetLatestCommitByLocalId(r, f.RepoId, s.commitId)
		if err != nil {
			return nil, "", err
		}
		commits = append(commits, c)
	}
	// Prepare the next cursor
	last := queryResults[len(queryResults)-1]
	nextCursor := searchCommitsCursor{
		CommitId:        last.commitId,
		TextSearchRowId: last.textSearchRowId,
	}
	return commits, nextCursor.encode(), nil
}

type searchedCommit struct {
	commitId commit.LocalId
	// 0 when the search did not read the text index
	textSearchRowId int64
}

func (db webDb) getSearchedCommits(r context.Context, query string, args []any) (
	found []searchedCommit, err error) {
	rows, err := db.s.Query(r, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s searchedCommit
		err = rows.Scan(&s.commitId, &s.textSearchRowId)
		if err != nil {
			return nil, err
		}
		found = append(found, s)
	}
	return found, rows.Err()
}

// Returns the FTS5 query that matches every word of the search text. The
// words are quoted so that the FTS5 operators in them are matched literally.
func ftsMatchQuery(text string) string {
	var q strings.Builder
	for word := range strings.FieldsSeq(text) {
		if q.Len() > 0 {
			q.WriteString(" AND ")
		}
		q.WriteString(`"` + strings.ReplaceAll(word, `"`, `""`) + `"`)
	}
	return q.String()
}

type searchCommitsCursor struct {
	CommitId        commit.LocalId
	TextSearchRowId int64
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
