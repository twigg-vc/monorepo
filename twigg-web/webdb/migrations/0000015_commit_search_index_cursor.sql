CREATE TABLE IF NOT EXISTS commit_search_index_cursor (
	id       INTEGER  NOT NULL PRIMARY KEY,
	repoId   INTEGER  NOT NULL,
	commitId INTEGER  NOT NULL
);
