CREATE TABLE IF NOT EXISTS permissions(
    userId INTEGER NOT NULL,
    permission INTEGER NOT NULL,
    assetId TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS permission_by_userId_perm_assetId
ON permissions (userId, permission, assetId);
