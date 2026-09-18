package webdb

import (
	"context"
	"encoding/base64"
	"fmt"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg-web/review"
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
	// Any other search is driven by the commits themselves, unless it asks
	// for a review status.
	// This is done because fts5 is efficient for text macthing, but any other
	// "order by" is O(number of matches), which would make queries like "fix"
	// be very inneficient (as basically all commits would be read).
	match := ftsMatchQuery(f.Message)
	// A commit nobody reviewed yet has no reviews row, so a search for
	// MissingLgtm has to read the commits to find them. Every other status
	// needs a reviews row, so it is driven by the reviews index instead of
	// reading every commit of the repo to find the few that match.
	searchesReviews := match == "" && f.HasReviewStatus &&
		f.ReviewStatus != review.ReviewStatus_MissingLgtm
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
	} else if searchesReviews {
		q.WriteString(`
			SELECT s.commitId, 0 FROM reviews r
			JOIN twigg_commit_search s
				ON s.repoId = r.repoId AND s.commitId = r.commitId
			WHERE r.repoId = ? AND r.reviewStatus = ?`)
		args = append(args, f.RepoId, uint32(f.ReviewStatus))
		if hasCursor {
			q.WriteString(` AND r.commitId < ?`)
			args = append(args, parsedCursor.CommitId)
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

	if f.AuthorUsername != "" {
		q.WriteString(` AND s.authorId = (
			SELECT u.id FROM users2 u WHERE u.username = ?)`)
		args = append(args, f.AuthorUsername)
	}
	if f.ReviewerUsername != "" {
		q.WriteString(` AND EXISTS (
			SELECT 1 FROM review_reviewers rr
			WHERE rr.repoId = s.repoId AND rr.commitId = s.commitId
				AND rr.userId = (
					SELECT u.id FROM users2 u WHERE u.username = ?))`)
		args = append(args, f.ReviewerUsername)
	}
	if f.HasReviewStatus {
		// A submitted commit has no review status.
		q.WriteString(` AND s.isSubmitted = 0`)
		if !searchesReviews {
			// The status of a commit with no reviews row is the zero value.
			q.WriteString(` AND COALESCE((
				SELECT r.reviewStatus FROM reviews r
				WHERE r.repoId = s.repoId AND r.commitId = s.commitId), ?) = ?`)
			args = append(args, uint32(review.ReviewStatus_MissingLgtm),
				uint32(f.ReviewStatus))
		}
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
	} else if searchesReviews {
		// Same as s.commitId, but reading it from the driving index.
		q.WriteString(` ORDER BY r.commitId DESC LIMIT ?`)
	} else {
		q.WriteString(` ORDER BY s.commitId DESC LIMIT ?`)
	}
	// One commit more than the page is read to know whether there is another
	// page, so that the last one says it is the last.
	args = append(args, limit+1)

	// Parse the query results
	queryResults, err := db.getSearchedCommits(r, q.String(), args)
	if err != nil {
		return nil, "", err
	}
	hasMore := len(queryResults) > limit
	if hasMore {
		queryResults = queryResults[:limit]
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
	if !hasMore {
		return commits, "", nil
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
