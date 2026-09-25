CREATE TABLE IF NOT EXISTS bug_event_counts (
    bugId INTEGER NOT NULL,
    kind  INTEGER NOT NULL,
    count INTEGER NOT NULL,
    PRIMARY KEY (bugId, kind)
);
INSERT INTO bug_event_counts (bugId, kind, count)
SELECT bugId, kind, COUNT(*) FROM bug_events GROUP BY bugId, kind;

CREATE TABLE IF NOT EXISTS repo_bug_counts (
    repoId INTEGER NOT NULL,
    status TEXT    NOT NULL,
    count  INTEGER NOT NULL,
    PRIMARY KEY (repoId, status)
);
INSERT INTO repo_bug_counts (repoId, status, count)
SELECT repoId, status, COUNT(*) FROM bugs GROUP BY repoId, status;
