CREATE TABLE IF NOT EXISTS ci_queue4(
    repoId         INTEGER  NOT NULL,
    commitId       INTEGER  NOT NULL,
    commitVersion  INTEGER  NOT NULL,
    runNumber      INTEGER  NOT NULL,
    trigger        TEXT NOT NULL,
    nonce          TEXT NOT NULL,
    status         TEXT NOT NULL,
    PRIMARY KEY (repoId, commitId, commitVersion, runNumber)
);
CREATE INDEX IF NOT EXISTS ci_queue4_by_repo_commit_version
ON ci_queue4(repoId, commitId, commitVersion, runNumber DESC);
