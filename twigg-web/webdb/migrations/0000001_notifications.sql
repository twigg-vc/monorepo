CREATE TABLE IF NOT EXISTS notifications(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    userId INTEGER NOT NULL,
    message TEXT NOT NULL,
    assetPath TEXT NOT NULL,
    createdAt TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    seenAt TEXT NOT NULL DEFAULT '',
    readAt TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS notifications_by_user_createdAt
ON notifications (userId, id DESC);

CREATE TABLE IF NOT EXISTS user_unseen_notify_count (
    userId INTEGER PRIMARY KEY,
    unseenCount INTEGER NOT NULL DEFAULT 0
);