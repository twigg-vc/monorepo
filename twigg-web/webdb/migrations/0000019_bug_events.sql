-- Each event kind keeps its details in its own table, keyed by eventId.
CREATE TABLE IF NOT EXISTS bug_events (
    eventId            INTEGER PRIMARY KEY AUTOINCREMENT,
    bugId              INTEGER NOT NULL,
    kind               INTEGER NOT NULL,
    authorId           INTEGER NOT NULL,
    createdOnUnixMilli INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS bug_events_by_bug
ON bug_events (bugId, eventId);
CREATE INDEX IF NOT EXISTS bug_events_by_bug_kind
ON bug_events (bugId, kind);

CREATE TABLE IF NOT EXISTS bug_comments (
    eventId INTEGER PRIMARY KEY,
    body    TEXT NOT NULL
);
