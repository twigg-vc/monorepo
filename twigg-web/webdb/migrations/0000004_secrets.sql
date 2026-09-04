CREATE TABLE IF NOT EXISTS secrets2 (
    secret_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id     INTEGER NOT NULL,
    secret_name TEXT NOT NULL,
    nonce       BLOB NOT NULL,
    encrypted   BLOB NOT NULL,
    UNIQUE (repo_id, secret_name)
);
CREATE INDEX IF NOT EXISTS idx_secrets2_secret_id_to_repo_id
ON secrets2 (secret_id, repo_id);
