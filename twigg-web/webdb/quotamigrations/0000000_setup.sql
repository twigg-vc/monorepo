CREATE TABLE IF NOT EXISTS quota (
    QuotaOwner TEXT PRIMARY KEY,
    Bytes  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS usage (
    QuotaOwner        TEXT PRIMARY KEY,
    SuccessfullBytes  INTEGER NOT NULL,
    QuotaLimittedBytes INTEGER NOT NULL
);
