CREATE INDEX IF NOT EXISTS bugs_by_last_update
ON bugs (repoId, updatedOnUnixMilli DESC, number DESC);

CREATE INDEX IF NOT EXISTS bugs_by_status_last_update
ON bugs (repoId, status, updatedOnUnixMilli DESC, number DESC);
