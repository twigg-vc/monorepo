CREATE TABLE IF NOT EXISTS sqlarge_blob_versions (
    IdPrefix            TEXT NOT NULL,
    Id                  TEXT NOT NULL,
    LastGrabbedVersion INTEGER NOT NULL,
    PRIMARY KEY (IdPrefix, Id)
);
INSERT INTO sqlarge_blob_versions (IdPrefix, Id, LastGrabbedVersion)
SELECT IdPrefix, Id, MAX(Version) FROM sqlarge_blobs GROUP BY IdPrefix, Id;
