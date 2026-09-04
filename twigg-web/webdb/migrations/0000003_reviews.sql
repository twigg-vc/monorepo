CREATE TABLE IF NOT EXISTS reviews (
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    PRIMARY KEY (repoId, commitId)
);

CREATE TABLE IF NOT EXISTS threads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    authorId INTEGER NOT NULL,
    threadType INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS thread_by_repo_and_commit
ON threads (repoId, commitId);
CREATE INDEX IF NOT EXISTS thread_by_repo_commit_author_type
ON threads (repoId, commitId, authorId, threadType);

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repoId INTEGER NOT NULL,
    commitId INTEGER NOT NULL,
    threadId INTEGER NOT NULL,
    authorId INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS comment_by_repo_commit_thread
ON comments (repoId, commitId, threadId);
