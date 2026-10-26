CREATE TABLE IF NOT EXISTS bug_priority_changes (
    eventId     INTEGER PRIMARY KEY,
    newPriority TEXT    NOT NULL -- '' when unset
);
