package webdb

import (
	"context"
	"fmt"
	"monorepo/twigg-web/review"
	"monorepo/twigg/commit"
)

// A version older than the indexed one is ignored.
func (db webDb) indexCommitForSearch(w context.Context, repoId uint64,
	c commit.Commit) error {
	_, err := db.s.Exec(w, `
		INSERT INTO twigg_commit_search
			(repoId, commitId, commitVersion, authorId, isSubmitted,
			createdOnUnixMilli, isWip, isArchived)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repoId, commitId) DO UPDATE SET
			commitVersion = EXCLUDED.commitVersion,
			authorId = EXCLUDED.authorId,
			isSubmitted = EXCLUDED.isSubmitted,
			createdOnUnixMilli = EXCLUDED.createdOnUnixMilli,
			isWip = EXCLUDED.isWip,
			isArchived = EXCLUDED.isArchived
		WHERE EXCLUDED.commitVersion >= twigg_commit_search.commitVersion
	`, repoId, c.L, c.Version, c.AuthorUserId, c.IsSubmitted,
		c.CreatedOn.UnixMilli(),
		review.MessageIsWip(c.Message), review.MessageIsArchived(c.Message))
	if err != nil {
		return err
	}
	return db.indexCommitTextForSearch(w, repoId, c.L, c.Version, c.Message)
}

// The message of a commit is only stored in the text index, one row per
// version so that the message of an older version is searchable too.
func (db webDb) indexCommitTextForSearch(w context.Context, repoId uint64,
	cId commit.LocalId, cVersion uint64, message string) error {
	_, err := db.s.Exec(w, `
		DELETE FROM commit_search_text
		WHERE repoId = ? AND commitId = ? AND commitVersion = ?
	`, repoId, cId, cVersion)
	if err != nil {
		return err
	}
	_, err = db.s.Exec(w, `
		INSERT INTO commit_search_text (message, repoId, commitId, commitVersion)
		VALUES (?, ?, ?, ?)
	`, message, repoId, cId, cVersion)
	return err
}

// The reviewers of a commit are replaced, not merged.
func (db webDb) indexReviewForSearch(w context.Context, repoId uint64,
	cId commit.LocalId, d review.Data) error {
	// this never happens. just checking bc we iterate on d.ReviewersUserIds
	if len(d.ReviewersUserIds) > review.MaxReviewers {
		panic(fmt.Sprintf("commit has %d reviewers", len(d.ReviewersUserIds)))
	}
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
