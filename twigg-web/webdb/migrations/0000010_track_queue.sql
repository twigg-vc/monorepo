CREATE TABLE IF NOT EXISTS track_queue (
    job_id        TEXT PRIMARY KEY,
    owner_id      BIGINT NOT NULL,
    payload       BLOB NOT NULL,
    status        TEXT NOT NULL,
    created_at_ns BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS track_queue_pick
ON track_queue (status, created_at_ns);

CREATE TABLE IF NOT EXISTS owner_usage2 (
    owner_id               BIGINT PRIMARY KEY,
    running_jobs           INTEGER NOT NULL,
    running_timeout_ms     BIGINT NOT NULL,
    max_running_jobs       INTEGER NOT NULL DEFAULT 1,
    max_running_timeout_ms INTEGER NOT NULL DEFAULT 60000
);
