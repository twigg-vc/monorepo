package webdb

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/commitsearch"
	"monorepo/twigg/commit"
	"strings"
)

func (db webDb) searchCommits(r context.Context, f commitsearch.Filter) (
	iterator.I[commit.Commit], error) {
	var q strings.Builder
	q.WriteString(`
		SELECT s.commitId FROM twigg_commit_search s
		WHERE s.repoId = ?`)
	args := []any{f.RepoId}

	if f.HasAfterCommitId {
		q.WriteString(` AND s.commitId < ?`)
		args = append(args, f.AfterCommitId)
	}
	q.WriteString(` ORDER BY s.commitId DESC LIMIT ?`)
	args = append(args, f.Limit)

	rows, err := db.s.Query(r, q.String(), args...)
	if err != nil {
		return nil, err
	}
	return commitIter{db: db, ctx: r, repoId: f.RepoId, commitIds: rows}, nil
}
