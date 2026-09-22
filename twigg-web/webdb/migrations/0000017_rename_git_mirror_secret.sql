-- The new name was not reserved before, so drop anything already using it.
DELETE FROM secrets2 WHERE secret_name = 'GIT_MIRROR_SECRET_URL';

UPDATE secrets2
SET secret_name = 'GIT_MIRROR_SECRET_URL'
WHERE secret_name = 'git-mirror-secret-ulr';
