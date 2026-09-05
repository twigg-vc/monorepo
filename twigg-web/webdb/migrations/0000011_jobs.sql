CREATE TABLE IF NOT EXISTS cicdruns (
	repoId        INTEGER  NOT NULL,
	commitId      INTEGER  NOT NULL,
	commitVersion INTEGER  NOT NULL,
	runNumber     INTEGER  NOT NULL,
	nonce         TEXT     NOT NULL,
	PRIMARY KEY   (repoId, commitId, commitVersion, runNumber)
);

CREATE TABLE IF NOT EXISTS jobs3 (
	internalJobId INTEGER PRIMARY KEY AUTOINCREMENT,
	repoId        INTEGER  NOT NULL,
	commitId      INTEGER  NOT NULL,
	commitVersion INTEGER  NOT NULL,
	path          TEXT     NOT NULL,
	name          TEXT     NOT NULL,
	runNumber     INTEGER  NOT NULL,
	status        TEXT     NOT NULL,
	createdTime   TEXT     NOT NULL,
	UNIQUE (repoId, commitId, commitVersion, path, name, runNumber)
);
CREATE INDEX IF NOT EXISTS jobs3_by_id
ON jobs3 (repoId, commitId, commitVersion, path, name, runNumber);

CREATE TABLE IF NOT EXISTS jobPipelines (
	internalJobPipelineId  INTEGER PRIMARY KEY AUTOINCREMENT,
	repoId                 INTEGER  NOT NULL,
	commitId               INTEGER  NOT NULL,
	commitVersion          INTEGER  NOT NULL,
	path                   TEXT     NOT NULL,
	name                   TEXT     NOT NULL,
	runNumber              INTEGER  NOT NULL,
	numberOfStages         INTEGER  NOT NULL,
	status                 TEXT     NOT NULL,
	createdTime            TEXT     NOT NULL,
	isCreatedByUser        BOOLEAN  NOT NULL,
	createdByUserId        INTEGER  NOT NULL,
	UNIQUE (repoId, commitId, commitVersion, path, name, runNumber)
);
CREATE INDEX IF NOT EXISTS jobPipelines_by_id
ON jobPipelines (repoId, commitId, commitVersion, path, name, runNumber);
CREATE INDEX IF NOT EXISTS jobPipelines_by_path_name_internalId
ON jobPipelines (repoId, path, name, internalJobPipelineId);

CREATE TABLE IF NOT EXISTS jobPipelineStages (
	jobPipelineId   TEXT  NOT NULL,
	stage           INTEGER  NOT NULL,
	name            TEXT     NOT NULL,
	createdTime     TEXT     NOT NULL,
	status          TEXT     NOT NULL,
	isResumedByUser BOOLEAN NOT NULL DEFAULT FALSE,
	resumedByUserId INTEGER  NOT NULL DEFAULT 0,
	PRIMARY KEY (jobPipelineId, stage)
);

CREATE TABLE IF NOT EXISTS jobPipelineRefs (
	repoId      INTEGER  NOT NULL,
	path        TEXT     NOT NULL,
	name        TEXT     NOT NULL,
	isArchived  BOOLEAN NOT NULL DEFAULT FALSE,
	PRIMARY KEY (repoId, path, name)
);
CREATE INDEX IF NOT EXISTS jobPipelineRefs_by_repoId
ON jobPipelineRefs (repoId, isArchived);
