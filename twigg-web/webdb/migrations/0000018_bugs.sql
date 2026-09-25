CREATE TABLE IF NOT EXISTS bugs (
    bugId              INTEGER PRIMARY KEY AUTOINCREMENT,
    repoId             INTEGER NOT NULL,
    number             INTEGER NOT NULL,
    authorId           INTEGER NOT NULL,
    assigneeUserId     INTEGER NOT NULL DEFAULT 0, -- 0 when unassigned
    title              TEXT    NOT NULL,
    body               TEXT    NOT NULL,
    status             TEXT    NOT NULL,
    createdOnUnixMilli INTEGER NOT NULL,
    updatedOnUnixMilli INTEGER NOT NULL,
    UNIQUE (repoId, number)
);
CREATE INDEX IF NOT EXISTS bugs_by_status
ON bugs (repoId, status, number DESC);
