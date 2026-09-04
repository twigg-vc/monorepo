CREATE TABLE IF NOT EXISTS twigg_commit_children(
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    commitVersion INTEGER NOT NULL,
    childCommitId INTEGER NOT NULL,
    childCommitVersion INTEGER NOT NULL,

    PRIMARY KEY(repoId, commitId, commitVersion,
        childCommitId, childCommitVersion)
);

CREATE INDEX IF NOT EXISTS twigg_commit_children_by_id
    ON twigg_commit_children (repoId, commitId, commitVersion);
