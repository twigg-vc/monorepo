CREATE TABLE IF NOT EXISTS repos (
    repoId INTEGER PRIMARY KEY AUTOINCREMENT,
    ownerId INTEGER NOT NULL,
    displayName TEXT NOT NULL,
    description TEXT NOT NULL,
    isGitMirrorEnabled BOOLEAN NOT NULL,
    sanitizedGitMirrorUrl TEXT,
    UNIQUE(ownerId, displayName)
);
CREATE INDEX IF NOT EXISTS repo_by_ownerId_displayName
ON repos (ownerId, displayName);

CREATE TABLE IF NOT EXISTS archived_repos (
    repoId INTEGER PRIMARY KEY NOT NULL,
    ownerId INTEGER NOT NULL,
    displayName TEXT NOT NULL,
    description TEXT NOT NULL,
    archivedDate  TEXT NOT NULL,
    isGitMirrorEnabled BOOLEAN NOT NULL,
    sanitizedGitMirrorUrl TEXT
);
