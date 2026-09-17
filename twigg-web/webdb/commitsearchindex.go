package webdb

import (
	"context"
	"monorepo/twigg-web/review"
	"monorepo/twigg/commit"
)

// A version older than the indexed one is ignored.
func (db webDb) indexCommitForSearch(w context.Context, repoId uint64,
	c commit.Commit) error {
	_, err := db.s.Exec(w, `
		INSERT INTO twigg_commit_search
			(repoId, commitId, commitVersion, authorId, isSubmitted,
			createdOnUnixMilli, message, isWip, isArchived)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repoId, commitId) DO UPDATE SET
			commitVersion = EXCLUDED.commitVersion,
			authorId = EXCLUDED.authorId,
			isSubmitted = EXCLUDED.isSubmitted,
			createdOnUnixMilli = EXCLUDED.createdOnUnixMilli,
			message = EXCLUDED.message,
			isWip = EXCLUDED.isWip,
			isArchived = EXCLUDED.isArchived
		WHERE EXCLUDED.commitVersion >= twigg_commit_search.commitVersion
	`, repoId, c.L, c.Version, c.AuthorUserId, c.IsSubmitted,
		c.CreatedOn.UnixMilli(), c.Message,
		review.MessageIsWip(c.Message), review.MessageIsArchived(c.Message))
	return err
}

// The reviewers of a commit are replaced, not merged.
func (db webDb) indexReviewForSearch(w context.Context, repoId uint64,
	cId commit.LocalId, d review.Data) error {
	_, err := db.s.Exec(w, `
		INSERT INTO reviews (repoId, commitId, reviewStatus)
		VALUES (?, ?, ?)
		ON CONFLICT(repoId, commitId) DO UPDATE SET
			reviewStatus = EXCLUDED.reviewStatus
	`, repoId, cId, uint32(d.ReviewStatus))
	if err != nil {
		return err
	}
	_, err = db.s.Exec(w, `
		DELETE FROM review_reviewers WHERE repoId = ? AND commitId = ?
	`, repoId, cId)
	if err != nil {
		return err
	}
	for _, userId := range d.ReviewersUserIds {
		_, err = db.s.Exec(w, `
			INSERT INTO review_reviewers (repoId, commitId, userId)
			VALUES (?, ?, ?)
			ON CONFLICT(repoId, commitId, userId) DO NOTHING
		`, repoId, cId, userId)
		if err != nil {
			return err
		}
	}
	return nil
}
