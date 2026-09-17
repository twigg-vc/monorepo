CREATE TABLE IF NOT EXISTS twigg_commit_search (
	repoId             INTEGER  NOT NULL,
	commitId           INTEGER  NOT NULL,
	commitVersion      INTEGER  NOT NULL,
	authorId           INTEGER  NOT NULL,
	isSubmitted        BOOLEAN  NOT NULL,
	createdOnUnixMilli INTEGER  NOT NULL,
	message            TEXT     NOT NULL,
	isWip              BOOLEAN  NOT NULL,
	isArchived         BOOLEAN  NOT NULL,
	PRIMARY KEY        (repoId, commitId)
);
CREATE INDEX IF NOT EXISTS twigg_commit_search_by_author
ON twigg_commit_search (repoId, authorId, commitId DESC);
CREATE INDEX IF NOT EXISTS twigg_commit_search_by_state
ON twigg_commit_search (repoId, isSubmitted, commitId DESC);

ALTER TABLE reviews ADD COLUMN reviewStatus INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS reviews_by_status
ON reviews (repoId, reviewStatus, commitId DESC);

CREATE TABLE IF NOT EXISTS review_reviewers (
	repoId      INTEGER  NOT NULL,
	commitId    INTEGER  NOT NULL,
	userId      INTEGER  NOT NULL,
	PRIMARY KEY (repoId, commitId, userId)
);
CREATE INDEX IF NOT EXISTS review_reviewers_by_user
ON review_reviewers (userId, repoId, commitId DESC);
