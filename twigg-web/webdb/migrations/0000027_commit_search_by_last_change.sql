DROP INDEX IF EXISTS twigg_commit_search_by_author;
DROP INDEX IF EXISTS twigg_commit_search_by_state;
CREATE INDEX IF NOT EXISTS twigg_commit_search_by_change
ON twigg_commit_search (repoId, createdOnUnixMilli DESC, commitId DESC);
CREATE INDEX IF NOT EXISTS twigg_commit_search_by_author
ON twigg_commit_search (repoId, authorId, createdOnUnixMilli DESC, commitId DESC);
CREATE INDEX IF NOT EXISTS twigg_commit_search_by_state
ON twigg_commit_search (repoId, isSubmitted, createdOnUnixMilli DESC, commitId DESC);
