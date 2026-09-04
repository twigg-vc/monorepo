CREATE TABLE IF NOT EXISTS twigg_repo_next(
    repoId INTEGER PRIMARY KEY,
    nextCommitLocalId INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS twigg_repo_top(
    repoId INTEGER PRIMARY KEY,
    topCommitLocalId INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS twigg_workdir_tree_cache2(
    path TEXT PRIMARY KEY,
    size INTEGER NOT NULL,
    modTimeUnixMilli INTEGER NOT NULL,
    hash BLOB NOT NULL,
    isText BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS twigg_commits(
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    commitVersion INTEGER NOT NULL,
    authorId INTEGER NOT NULL,
    isSubmitted BOOLEAN NOT NULL,
    hasServerCommitId BOOLEAN NOT NULL,
    serverCommitId INTERGER NOT NULL,
    hasServerCommitVersion BOOLEAN NOT NULL,
    serverCommitVersion INTERGER NOT NULL,

    PRIMARY KEY(repoId, commitId, commitVersion)
);
CREATE INDEX IF NOT EXISTS idx_twigg_commits_versions
    ON twigg_commits (repoId, commitId, commitVersion ASC);
CREATE INDEX IF NOT EXISTS idx_twigg_commits_repo_server_ver
ON twigg_commits (
    repoId,
    hasServerCommitId,
    serverCommitId,
    hasServerCommitVersion,
    serverCommitVersion,
    commitVersion DESC
);

CREATE TABLE IF NOT EXISTS twigg_pending_commits(
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    authorId INTEGER NOT NULL,
    UNIQUE(repoId, commitId, authorId)
);
CREATE INDEX IF NOT EXISTS idx_twigg_pending_commits_by_ids
    ON twigg_commits (repoId, commitId ASC);

CREATE TABLE IF NOT EXISTS twigg_submitted_commits(
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    authorId INTEGER NOT NULL,
    PRIMARY KEY(repoId, commitId)
);

CREATE TABLE IF NOT EXISTS sqlarge_blobs (
    IdPrefix           TEXT NOT NULL,
    Id                 TEXT NOT NULL,
    Version            INTEGER NOT NULL,
    IsLatest           BOOLEAN NOT NULL,
    SavedAt            TIMESTAMP NOT NULL,
    IsDeleted          BOOLEAN NOT NULL,
    Datastrip          TEXT NOT NULL,
    Offset             INTEGER NOT NULL,
    DistanceToNonDelta INTEGER NOT NULL,
    CompressedSize     INTEGER NOT NULL,
    UncompressedSize   INTEGER NOT NULL,
    Encoding           INTEGER NOT NULL,
    QuotaOwner         TEXT NOT NULL,
    PRIMARY KEY (IdPrefix, Id, Version)
);
CREATE INDEX IF NOT EXISTS idx_latest
ON sqlarge_blobs (IdPrefix, Id, Version, IsLatest, IsDeleted);