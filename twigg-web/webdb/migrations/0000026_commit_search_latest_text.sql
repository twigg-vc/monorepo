-- Keep the existing latest-message rows and their pagination rowids. The
-- primary key on twigg_commit_search makes each lookup independent of the
-- number of historical versions. No commit blobs need to be read again.
DELETE FROM commit_search_text
WHERE EXISTS (
	SELECT 1 FROM twigg_commit_search s
	WHERE s.repoId = commit_search_text.repoId
		AND s.commitId = commit_search_text.commitId
		AND s.commitVersion > commit_search_text.commitVersion
);
