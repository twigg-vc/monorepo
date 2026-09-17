package webdb

import "context"

func (db webDb) indexCommitsForSearch(w context.Context,
	after IndexCommitsForSearchCursor, limit int) (
	next IndexCommitsForSearchCursor, done bool, err error) {
	// The batch is read before any of it is written because the rows of a
	// query can't be iterated while the same transaction writes.
	batch, err := db.getBatchOfCommitsToIndex(w, after, limit)
	if err != nil {
		return after, false, err
	}
	if len(batch) == 0 {
		return after, true, nil
	}
	err = db.indexCommitBatch(w, batch)
	if err != nil {
		return after, false, err
	}
	return batch[len(batch)-1], false, nil
}

func (db webDb) getBatchOfCommitsToIndex(w context.Context,
	after IndexCommitsForSearchCursor, limit int) (
	batch []IndexCommitsForSearchCursor, err error) {
	rows, err := db.s.Query(w, `
		SELECT DISTINCT repoId, commitId FROM twigg_commits
		WHERE (repoId, commitId) > (?, ?)
		ORDER BY repoId, commitId
		LIMIT ?
	`, after.RepoId, after.CommitId, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id IndexCommitsForSearchCursor
		err = rows.Scan(&id.RepoId, &id.CommitId)
		if err != nil {
			return nil, err
		}
		batch = append(batch, id)
	}
	return batch, rows.Err()
}

func (db webDb) indexCommitBatch(w context.Context,
	batch []IndexCommitsForSearchCursor) error {
	for _, id := range batch {
		err := db.indexCommitFromBlobs(w, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db webDb) indexCommitFromBlobs(w context.Context,
	id IndexCommitsForSearchCursor) error {
	// Only repairs a row holding a version that no commit has.
	const deleteIndexedCommitRows = false
	if deleteIndexedCommitRows {
		_, err := db.s.Exec(w, `
			DELETE FROM twigg_commit_search WHERE repoId = ? AND commitId = ?
		`, id.RepoId, id.CommitId)
		if err != nil {
			return err
		}
	}
	c, _, err := db.GetLatestCommitByLocalId(w, id.RepoId, id.CommitId)
	if err != nil {
		return err
	}
	err = db.indexCommitForSearch(w, id.RepoId, c)
	if err != nil {
		return err
	}
	hasReview, err := db.HasReview(w, id.RepoId, id.CommitId)
	if err != nil || !hasReview {
		return err
	}
	d, err := db.GetReviewData(w, id.RepoId, id.CommitId)
	if err != nil {
		return err
	}
	return db.indexReviewForSearch(w, id.RepoId, id.CommitId, d)
}
