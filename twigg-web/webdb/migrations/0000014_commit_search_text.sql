CREATE VIRTUAL TABLE IF NOT EXISTS commit_search_text USING fts5(
	message,
	repoId UNINDEXED,
	commitId UNINDEXED,
	commitVersion UNINDEXED,
	tokenize="unicode61 remove_diacritics 2"
);

ALTER TABLE twigg_commit_search DROP COLUMN message;
