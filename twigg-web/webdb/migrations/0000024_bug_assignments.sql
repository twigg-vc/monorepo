CREATE TABLE IF NOT EXISTS bug_assignments (
    eventId           INTEGER PRIMARY KEY,
    newAssigneeUserId INTEGER NOT NULL -- 0 when unassigned
);
