package webdb

import (
	"context"
	"database/sql"
	"io"
	"monorepo/base/iterator"
	"monorepo/data/blobdb"
	"monorepo/twigg-web/education"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/notification"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/secrets"
	"monorepo/twigg-web/user"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
	"monorepo/twigg/treev"
)

// Implements the database used by Twigg Web
type WebDb struct {
	db *webDb
}

// Creates a new instance that stores everything under pathToDir.
// dbFileName specifies the name of the underlying sqlite db file.
// blockSize determines the size of the blocks the blob bytes are split into
// (on disk and, if bs is provided, on the blob storage). bs is optional and
// should be used to reduce disk usage: older blocks are spilled to it and
// downloaded back on demand, keeping up to blobStorageCacheCapacity of them
// on disk at a time.
// enforceQuota refuses blob writes once the quota owner runs out of quota.
// Bytes are tracked either way.
// closeDb must be called when done.
func New(pathToDir, dbFileName string, blockSize int64, bs BlobStorage,
	blobStorageCacheCapacity int, enforceQuota bool) (db WebDb, closeDb func(), err error) {
	return newWebDb(pathToDir, dbFileName, blockSize, bs, blobStorageCacheCapacity, enforceQuota)
}

// Creates a new instance that stores the data in memory. Quota bytes are
// tracked but never enforced.
func NewMem() (db WebDb, closeDb func(), err error) {
	return newMemWebDb(false)
}

// Creates a new instance that stores the data in memory and enforces quota
func NewMemWithQuotaEnforcement() (db WebDb, closeDb func(), err error) {
	return newMemWebDb(true)
}

// Service for storage of blobs; like S3 or Digital Ocean Spaces
type BlobStorage interface {
	Put(keyPrefix, key string, size int64, r io.Reader) error
	Get(keyPrefix, key string, offset int64) (r io.Reader, closeR func(), err error)
}

// Returns a write context that must be used with all other methods that write to the db
func (db WebDb) BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.db.BeginWrite()
}

// Returns a read context that must be used with all other methods that read from the db
func (db WebDb) BeginRead() (readCtx context.Context, closeTx func(), err error) {
	return db.db.BeginRead()
}

// Marks the write transaction bound to writeCtx as one that must not be
// committed, regardless of what else happens on it.
func (db WebDb) PreventCommit(writeCtx context.Context) {
	db.db.PreventCommit(writeCtx)
}

// Reports whether the write transaction bound to writeCtx should be
// committed: at least one write succeeded, none failed, and PreventCommit
// was never called.
func (db WebDb) ShouldCommit(writeCtx context.Context) bool {
	return db.db.ShouldCommit(writeCtx)
}

func (db WebDb) GetRepoNextLocalId(ctx context.Context, repoId uint64) (n uint64, isNotFoundErr bool, e error) {
	return db.db.GetRepoNextLocalId(ctx, repoId)
}
func (db WebDb) SetRepoNextLocalId(ctx context.Context, repoId uint64, n uint64) error {
	return db.db.SetRepoNextLocalId(ctx, repoId, n)
}

func (db WebDb) GetRepoTopCommit(ctx context.Context, repoId uint64) (commitLocalId uint64, isNotFoundErr bool, e error) {
	return db.db.GetRepoTopCommit(ctx, repoId)
}
func (db WebDb) SetRepoTopCommit(ctx context.Context, repoId uint64, commitLocalId uint64) error {
	return db.db.SetRepoTopCommit(ctx, repoId, commitLocalId)
}

func (db WebDb) GetTreeData(ctx context.Context, repoId uint64, treePath string, v uint64) (td treev.TreeDataV, isNotFoundErr bool, e error) {
	return db.db.GetTreeData(ctx, repoId, treePath, v)
}
func (db WebDb) GetTreeBlob(ctx context.Context, repoId uint64, treePath string, v uint64) (r io.Reader, closeR func(), isNotFoundErr bool, e error) {
	return db.db.GetTreeBlob(ctx, repoId, treePath, v)
}
func (db WebDb) GetLastVersionOfRootTree(ctx context.Context, repoId uint64) (v uint64, isNotFoundErr bool, e error) {
	return db.db.GetLastVersionOfRootTree(ctx, repoId)
}
func (db WebDb) SetTreeData(ctx context.Context, quotaOwner string, repoId uint64, treePath string, td treev.TreeDataV) (uint64, error) {
	return db.db.SetTreeData(ctx, quotaOwner, repoId, treePath, td)
}
func (db WebDb) SetTreeBlob(ctx context.Context, quotaOwner string, repoId uint64, treePath string, wt io.WriterTo) (uint64, error) {
	return db.db.SetTreeBlob(ctx, quotaOwner, repoId, treePath, wt)
}

func (db WebDb) SetCommit(ctx context.Context, quotaOwner string, repoId uint64, c commit.Commit) error {
	return db.db.SetCommit(ctx, quotaOwner, repoId, c)
}
func (db WebDb) GetLatestCommitByLocalId(ctx context.Context, repoId uint64, L uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetLatestCommitByLocalId(ctx, repoId, L)
}
func (db WebDb) GetCommitVersionByLocalId(ctx context.Context, repoId uint64, L uint64, v uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetCommitVersionByLocalId(ctx, repoId, L, v)
}
func (db WebDb) GetLatestCommitByServerId(ctx context.Context, repoId uint64, ServerL uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetLatestCommitByServerId(ctx, repoId, ServerL)
}
func (db WebDb) GetCommitVersionByServerId(ctx context.Context, repoId uint64, ServerL uint64, ServerV uint64) (c commit.Commit, isNotFoundErr bool, e error) {
	return db.db.GetCommitVersionByServerId(ctx, repoId, ServerL, ServerV)
}
func (db WebDb) GetCommitChildren(ctx context.Context, repoId uint64, commitL uint64, commitV uint64, maxRowsToRead int) (children []commit.LocalId, childrenVersions []uint64, e error) {
	return db.db.GetCommitChildren(ctx, repoId, commitL, commitV, maxRowsToRead)
}
func (db WebDb) GetPendingCommits(ctx context.Context, ascendingOrder bool, repoId uint64) (iterator.I[commit.Commit], error) {
	return db.db.GetPendingCommits(ctx, ascendingOrder, repoId)
}
func (db WebDb) GetPendingCommitsAfter(ctx context.Context, repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	return db.db.GetPendingCommitsAfter(ctx, repoId, afterId)
}

// Write a blob by its Id. The first version is 0, each write creates
// version latest+1.
func (db WebDb) SetBlob(writeCtx context.Context, quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, e error) {
	return db.db.SetBlob(writeCtx, quotaOwner, idPrefix, id, wt)
}

// Get the latest version of a blob by its id. Returns ErrNotFound if not
// found. closeReader must always be called.
func (db WebDb) GetBlob(readCtx context.Context, idPrefix, id string) (
	m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return db.db.GetBlob(readCtx, idPrefix, id)
}

// Get a version of a blob by its id. Returns ErrNotFound if not found.
// closeReader must always be called.
func (db WebDb) GetBlobVersion(readCtx context.Context, idPrefix, id string, v blobdb.Version) (
	m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return db.db.GetBlobVersion(readCtx, idPrefix, id, v)
}

// Returns the total number of bytes the quota owner owns.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) GetQuota(quotaOwner string) (int64, error) {
	return db.db.quota.GetQuota(quotaOwner)
}

// Returns the total bytes used and bytes tried but quota-limited.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) GetQuotaUsed(quotaOwner string) (success int64, quotaLimitted int64, err error) {
	return db.db.quota.GetQuotaUsed(quotaOwner)
}

// Returns the number of bytes the quota owner can still write.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) GetQuotaLeft(quotaOwner string) (int64, error) {
	return db.db.quota.GetQuotaLeft(quotaOwner)
}

// Adds nBytes to the number of bytes the specified quota owner can write.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) AddQuota(quotaOwner string, nBytes int64) error {
	return db.db.quota.AddQuota(quotaOwner, nBytes)
}

// Sets the total number of bytes the specified quota owner can write.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) SetQuota(quotaOwner string, nBytes int64) error {
	return db.db.quota.SetQuota(quotaOwner, nBytes)
}

// Sets the quota to the current usage.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) FreezeQuota(quotaOwner string) error {
	return db.db.quota.FreezeQuota(quotaOwner)
}

// Sets the quotaLimitted to zero.
// Independent of any read/write context: quota never joins the caller's
// transaction.
func (db WebDb) ClearQuotaLimitted(quotaOwner string) error {
	return db.db.quota.ClearQuotaLimitted(quotaOwner)
}

// Creates a notification for a user (unread by default).
// assetId can be used to link the notification to some resource (repo, org, etc).
func (db WebDb) CreateNotification(writeCtx context.Context, userId int64, message string, assetPath string) error {
	return db.db.CreateNotification(writeCtx, userId, message, assetPath)
}

// Marks a notification as read (sets readAt). No-op if it doesn't exist or is already read.
func (db WebDb) MarkNotificationRead(writeCtx context.Context, userId int64, notificationId int64) error {
	return db.db.MarkNotificationRead(writeCtx, userId, notificationId)
}

// Marks the given notifications as seen (sets seenAt). No-op for IDs already seen or not owned by userId.
func (db WebDb) MarkNotificationSeen(writeCtx context.Context, userId int64, notificationIds []int64) error {
	return db.db.MarkNotificationSeen(writeCtx, userId, notificationIds)
}

// Marks all notifications of a user as seen. No-op if there are none unseen.
func (db WebDb) MarkAllNotificationsSeen(writeCtx context.Context, userId int64) error {
	return db.db.MarkAllNotificationsSeen(writeCtx, userId)
}

// Returns all notifications of a user (newest first).
// Use lastReadNotificationId<=0 to see the latest ones.
func (db WebDb) GetUserNotifications(ctx context.Context, userId int64, lastReadNotificationId int64) (iterator.I[notification.Notification], error) {
	return db.db.GetUserNotifications(ctx, userId, lastReadNotificationId)
}

// Returns the total number of not-seen notifications for a user.
func (db WebDb) GetUnseenNotificationCount(ctx context.Context, userId int64) (int64, error) {
	return db.db.GetUnseenNotificationCount(ctx, userId)
}

// Returns the education state of a user.
// Returns a zero value and nil if not found.
func (db WebDb) GetUserEducation(ctx context.Context, userId int64) (education.UserEducation, error) {
	return db.db.GetUserEducation(ctx, userId)
}

// Writes all columns of the user's row, creating it if missing.
func (db WebDb) SetUserEducation(writeCtx context.Context, ed education.UserEducation) error {
	return db.db.SetUserEducation(writeCtx, ed)
}

// Sets whether the welcome was shown to a user, overwriting any previous value.
func (db WebDb) SetWelcomeWasShown(writeCtx context.Context, userId int64, welcomeWasShown bool) error {
	return db.db.SetWelcomeWasShown(writeCtx, userId, welcomeWasShown)
}

// Returns if User has permission grated.
func (db WebDb) HasPermission(ctx context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	return db.db.HasPermission(ctx, userId, p, assetId)
}

// Grant permission to user if it it doesn't already exists
// (does nothing and returns 'true, nil' otherwise)
func (db WebDb) GrantPermissionIfNotExists(ctx context.Context, userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error) {
	return db.db.GrantPermissionIfNotExists(ctx, userId, p, assetId)
}

// Revoke permission to user if the permission exists
// (does nothing and returns nil otherwise)
func (db WebDb) RevokePermissionIfExists(ctx context.Context, userId int64, p permissions.Permission, assetId string) error {
	return db.db.RevokePermissionIfExists(ctx, userId, p, assetId)
}

// Revokes all permissions associated with an asset
func (db WebDb) RevokeAllPermissionsToAsset(ctx context.Context, assetId string) error {
	return db.db.RevokeAllPermissionsToAsset(ctx, assetId)
}

// Returns all permissions of a user
func (db WebDb) GetUserPermissions(ctx context.Context, userId int64) (iterator.I[permissions.Permission], error) {
	return db.db.GetUserPermissions(ctx, userId)
}

// Returns all assets to which the user has at least one of the permissions
func (db WebDb) GetUserAssetIdsWithPermission(ctx context.Context,
	userId int64, p ...permissions.Permission) (iterator.I[string], error) {
	return db.db.GetUserAssetIdsWithPermission(ctx, userId, p...)
}

// Returns all userIds which have a permission
func (db WebDb) GetUsersWithPermission(ctx context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error) {
	return db.db.GetUsersWithPermission(ctx, assetId, p)
}

// number of assets user have permission
func (db WebDb) CountUserAssetsWithPermission(ctx context.Context, userId int64, permission permissions.Permission) (int64, error) {
	return db.db.CountUserAssetsWithPermission(ctx, userId, permission)
}

// number of users that have a specific permission on an asset
func (db WebDb) CountUsersWithPermission(ctx context.Context, assetId string, p permissions.Permission) (int64, error) {
	return db.db.CountUsersWithPermission(ctx, assetId, p)
}

// Records that a review exists for the commit. No-op if it already does.
func (db WebDb) CreateReviewIfNotExists(writeCtx context.Context, repoId uint64, cId commit.LocalId) error {
	return db.db.CreateReviewIfNotExists(writeCtx, repoId, cId)
}

// Reports whether a review was recorded for the commit. Only returns an error
// on failures to read.
func (db WebDb) HasReview(ctx context.Context, repoId uint64, cId commit.LocalId) (bool, error) {
	return db.db.HasReview(ctx, repoId, cId)
}

// Returns the review data of the commit. Returns ErrNotFound if it was
// never set.
func (db WebDb) GetReviewData(ctx context.Context, repoId uint64, cId commit.LocalId) (review.Data, error) {
	return db.db.GetReviewData(ctx, repoId, cId)
}

// Sets the review data of the commit, overwriting any previous one.
func (db WebDb) SetReviewData(writeCtx context.Context, quotaOwner string, repoId uint64, cId commit.LocalId, d review.Data) error {
	return db.db.SetReviewData(writeCtx, quotaOwner, repoId, cId, d)
}

// Returns the thread by its id. Returns ErrNotFound if it was never set.
func (db WebDb) GetReviewThread(ctx context.Context, threadId int64) (review.Thread, error) {
	return db.db.GetReviewThread(ctx, threadId)
}

// Sets the thread contents for a thread id (from CreateReviewThread),
// overwriting any previous one.
func (db WebDb) SetReviewThread(writeCtx context.Context, quotaOwner string, threadId int64, th review.Thread) error {
	return db.db.SetReviewThread(writeCtx, quotaOwner, threadId, th)
}

// Returns the comment by its id. Returns ErrNotFound if it was never set.
func (db WebDb) GetReviewComment(ctx context.Context, commentId int64) (review.Comment, error) {
	return db.db.GetReviewComment(ctx, commentId)
}

// Sets the comment contents for a comment id (from CreateReviewComment),
// overwriting any previous one.
func (db WebDb) SetReviewComment(writeCtx context.Context, quotaOwner string, commentId int64, cm review.Comment) error {
	return db.db.SetReviewComment(writeCtx, quotaOwner, commentId, cm)
}

// Creates a review thread row and returns its id.
func (db WebDb) CreateReviewThread(writeCtx context.Context, repoId uint64, cId commit.LocalId,
	authorId int64, threadType uint32) (threadId int64, err error) {
	return db.db.CreateReviewThread(writeCtx, repoId, cId, authorId, threadType)
}

// Returns the ids of all threads of a commit's review.
func (db WebDb) GetReviewThreadIds(ctx context.Context, repoId uint64, cId commit.LocalId) (iterator.I[int64], error) {
	return db.db.GetReviewThreadIds(ctx, repoId, cId)
}

// Returns the id of the user's most recent LGTM thread (add or remove) on
// the commit. If there is none, isNotFoundErr is true and err is ErrNotFound.
func (db WebDb) GetReviewUserLastLgtmThreadId(ctx context.Context, repoId uint64, cId commit.LocalId,
	userId int64, addLgtmType, removeLgtmType uint32) (threadId int64, isNotFoundErr bool, err error) {
	return db.db.GetReviewUserLastLgtmThreadId(ctx, repoId, cId, userId, addLgtmType, removeLgtmType)
}

// Returns the authors that currently hold an LGTM on the commit: those with
// an odd count of add/remove LGTM threads.
func (db WebDb) GetReviewLgtmAuthorIds(ctx context.Context, repoId uint64, cId commit.LocalId,
	addLgtmType, removeLgtmType uint32) (iterator.I[int64], error) {
	return db.db.GetReviewLgtmAuthorIds(ctx, repoId, cId, addLgtmType, removeLgtmType)
}

// Creates a review comment row and returns its id.
func (db WebDb) CreateReviewComment(writeCtx context.Context, repoId uint64, cId commit.LocalId,
	threadId int64, authorId int64) (commentId int64, err error) {
	return db.db.CreateReviewComment(writeCtx, repoId, cId, threadId, authorId)
}

// Returns the ids of all comments of a thread.
func (db WebDb) GetReviewCommentIds(ctx context.Context, repoId uint64, cId commit.LocalId,
	threadId int64) (iterator.I[int64], error) {
	return db.db.GetReviewCommentIds(ctx, repoId, cId, threadId)
}

// Reports whether the repo has a secret with that name.
func (db WebDb) HasRepoSecret(ctx context.Context, repoId uint64, secretName string) (bool, error) {
	return db.db.HasRepoSecret(ctx, repoId, secretName)
}

// Creates a secret row (nonce + ciphertext) and returns its id. Fails if the
// repo already has a secret with that name.
func (db WebDb) InsertRepoSecret(writeCtx context.Context, repoId uint64,
	secretName string, nonce, encrypted []byte) (secretId uint64, err error) {
	return db.db.InsertRepoSecret(writeCtx, repoId, secretName, nonce, encrypted)
}

// Overwrites the nonce and ciphertext of an existing secret and returns its
// id. If there is none, isNotFoundErr is true and err is ErrNotFound.
func (db WebDb) UpdateRepoSecret(writeCtx context.Context, repoId uint64,
	secretName string, nonce, encrypted []byte) (secretId uint64, isNotFoundErr bool, err error) {
	return db.db.UpdateRepoSecret(writeCtx, repoId, secretName, nonce, encrypted)
}

// Returns the nonce and ciphertext of a secret. If there is none,
// isNotFoundErr is true and err is ErrNotFound.
func (db WebDb) GetRepoSecretEncrypted(ctx context.Context, repoId uint64,
	secretName string) (nonce, encrypted []byte, isNotFoundErr bool, err error) {
	return db.db.GetRepoSecretEncrypted(ctx, repoId, secretName)
}

// Deletes the secret. No-op if it doesn't exist.
func (db WebDb) DeleteRepoSecret(writeCtx context.Context, repoId uint64, secretName string) error {
	return db.db.DeleteRepoSecret(writeCtx, repoId, secretName)
}

// Returns the number of secrets of a repo.
func (db WebDb) CountRepoSecrets(ctx context.Context, repoId uint64) (int64, error) {
	return db.db.CountRepoSecrets(ctx, repoId)
}

// Returns up to limit secrets (id and name only, no ciphertext) of the repo
// with secret_id > afterSecretId, ordered by secret_id.
func (db WebDb) GetRepoSecretsPage(ctx context.Context, repoId uint64,
	afterSecretId uint64, limit int64) (iterator.I[secrets.SecretRef], error) {
	return db.db.GetRepoSecretsPage(ctx, repoId, afterSecretId, limit)
}

// Creates a repo row (mirror disabled, no mirror url) and returns its id.
func (db WebDb) CreateRepo(writeCtx context.Context, ownerId int64,
	displayName, description string) (repoId uint64, err error) {
	return db.db.CreateRepo(writeCtx, ownerId, displayName, description)
}

// Returns the repo by its id. Returns ErrNotFound if there is none.
func (db WebDb) GetRepoById(ctx context.Context, repoId uint64) (repo.Repo, error) {
	return db.db.GetRepoById(ctx, repoId)
}

// Returns the repo of an owner by display name. If there is none,
// isNotFoundErr is true and err is ErrNotFound.
func (db WebDb) GetRepoByOwnerIdAndName(ctx context.Context,
	ownerId int64, displayName string) (r repo.Repo, isNotFoundErr bool, err error) {
	return db.db.GetRepoByOwnerIdAndName(ctx, ownerId, displayName)
}

// Returns all repos of an owner.
func (db WebDb) GetReposByOwnerId(ctx context.Context, ownerId int64) (iterator.I[repo.Repo], error) {
	return db.db.GetReposByOwnerId(ctx, ownerId)
}

// Moves the repo row to archived_repos, stamping the archive date.
// Returns ErrNotFound if the repo doesn't exist.
func (db WebDb) ArchiveRepo(writeCtx context.Context, ownerId int64, repoId uint64) error {
	return db.db.ArchiveRepo(writeCtx, ownerId, repoId)
}

// Returns the ids of the archived repos of an owner.
func (db WebDb) GetArchivedRepoIds(ctx context.Context, ownerId int64) (iterator.I[uint64], error) {
	return db.db.GetArchivedRepoIds(ctx, ownerId)
}

// Makes the repo public, so that anyone can read it.
func (db WebDb) SetRepoPublic(writeCtx context.Context,
	ownerId int64, displayName string) error {
	return db.db.SetRepoPublic(writeCtx, ownerId, displayName)
}

// Makes the repo private, so that only users with permission can read it.
func (db WebDb) SetRepoPrivate(writeCtx context.Context,
	ownerId int64, displayName string) error {
	return db.db.SetRepoPrivate(writeCtx, ownerId, displayName)
}

// Sets the description of the repo.
func (db WebDb) SetRepoDescription(writeCtx context.Context,
	ownerId int64, displayName, description string) error {
	return db.db.SetRepoDescription(writeCtx, ownerId, displayName, description)
}

// Sets whether the git mirror is enabled for the repo.
func (db WebDb) SetRepoGitMirrorEnabled(writeCtx context.Context,
	ownerId int64, displayName string, enabled bool) error {
	return db.db.SetRepoGitMirrorEnabled(writeCtx, ownerId, displayName, enabled)
}

// Sets the sanitized (credential-masked) git mirror url shown to users.
func (db WebDb) SetRepoSanitizedGitMirrorUrl(writeCtx context.Context,
	ownerId int64, displayName, sanitizedUrl string) error {
	return db.db.SetRepoSanitizedGitMirrorUrl(writeCtx, ownerId, displayName, sanitizedUrl)
}

// Returns the highest run number of a commit version's CI/CD queue runs.
// If there is none, isNotFoundErr is true and err is ErrNotFound.
func (db WebDb) GetCiCdQueueLastRunNumber(ctx context.Context,
	repoId, commitId, commitVersion uint64) (runNumber int64, isNotFoundErr bool, err error) {
	return db.db.GetCiCdQueueLastRunNumber(ctx, repoId, commitId, commitVersion)
}

// Creates a CI/CD queue run row.
func (db WebDb) InsertCiCdQueueRun(writeCtx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64,
	trigger, nonce, status string) error {
	return db.db.InsertCiCdQueueRun(writeCtx, repoId, commitId, commitVersion,
		runNumber, trigger, nonce, status)
}

// Returns the status of the CI/CD queue run with the highest run number of a
// commit version. If there is none, isNotFoundErr is true and err is ErrNotFound.
func (db WebDb) GetCiCdQueueLatestRunStatus(ctx context.Context,
	repoId, commitId, commitVersion uint64) (status string, isNotFoundErr bool, err error) {
	return db.db.GetCiCdQueueLatestRunStatus(ctx, repoId, commitId, commitVersion)
}

// Returns the trigger and status of a CI/CD queue run, matched by its full key
// including the nonce. If there is none, isNotFoundErr is true and err is
// ErrNotFound.
func (db WebDb) GetCiCdQueueRunTriggerAndStatus(ctx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64,
	nonce string) (trigger, status string, isNotFoundErr bool, err error) {
	return db.db.GetCiCdQueueRunTriggerAndStatus(ctx, repoId, commitId, commitVersion,
		runNumber, nonce)
}

// Sets the status of a CI/CD queue run, matched by its full key including the
// nonce. No-op if there is none.
func (db WebDb) SetCiCdQueueRunStatus(writeCtx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64,
	nonce, status string) error {
	return db.db.SetCiCdQueueRunStatus(writeCtx, repoId, commitId, commitVersion,
		runNumber, nonce, status)
}

// Reports whether a CI/CD run was marked as published.
func (db WebDb) CiCdRunExists(ctx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64) (bool, error) {
	return db.db.CiCdRunExists(ctx, repoId, commitId, commitVersion, runNumber)
}

// Marks a CI/CD run as published.
func (db WebDb) InsertCiCdRun(writeCtx context.Context,
	repoId, commitId, commitVersion uint64, runNumber int64, nonce string) error {
	return db.db.InsertCiCdRun(writeCtx, repoId, commitId, commitVersion,
		runNumber, nonce)
}

// Reports whether a job row with that key exists.
func (db WebDb) JobExists(ctx context.Context, repoId, commitId, commitVersion uint64,
	path, name string, runNumber int64) (bool, error) {
	return db.db.JobExists(ctx, repoId, commitId, commitVersion, path, name, runNumber)
}

// Creates a job row and returns the internal id assigned to it.
func (db WebDb) InsertJob(writeCtx context.Context, j job.Job) (internalJobId int64, err error) {
	return db.db.InsertJob(writeCtx, j)
}

// Adapts the read context to the twigg server read interface.
func (db WebDb) GetServerRead(ctx context.Context) server.Read {
	return db.Bind(ctx)
}

// Adapts the write context to the twigg server write interface.
func (db WebDb) GetServerWrite(writeCtx context.Context) server.Write {
	return db.Bind(writeCtx)
}

// DEPRECATED
// This method should not be used. It only exists while we're migrating off a
// bad legacy implementation
func (db WebDb) Exec(writeCtx context.Context, query string, args ...any) (sql.Result, error) {
	return db.db.s.Exec(writeCtx, query, args...)
}

// DEPRECATED
// This method should not be used. It only exists while we're migrating off a
// bad legacy implementation
func (db WebDb) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.db.s.QueryRow(ctx, query, args...)
}

// DEPRECATED
// This method should not be used. It only exists while we're migrating off a
// bad legacy implementation
func (db WebDb) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.db.s.Query(ctx, query, args...)
}

// Returns a new context that contains a read/write context and the db itself.
// This allows for methods to be executed directly on it; without haivng to pass
// the db around. It's needed for the twigg methods.
func (db WebDb) Bind(ctx context.Context) Ctx {
	return Ctx{db: db, ctx: ctx}
}

// Binds a db instance and a read/write context to expose methods that don't
// require a context
type Ctx struct {
	db  WebDb
	ctx context.Context
}

func (c Ctx) GetRepoNextLocalId(repoId uint64) (n uint64, isNotFoundErr bool, e error) {
	return c.db.GetRepoNextLocalId(c.ctx, repoId)
}
func (c Ctx) SetRepoNextLocalId(repoId uint64, n uint64) error {
	return c.db.SetRepoNextLocalId(c.ctx, repoId, n)
}

func (c Ctx) GetRepoTopCommit(repoId uint64) (commitLocalId uint64, isNotFoundErr bool, e error) {
	return c.db.GetRepoTopCommit(c.ctx, repoId)
}
func (c Ctx) SetRepoTopCommit(repoId uint64, commitLocalId uint64) error {
	return c.db.SetRepoTopCommit(c.ctx, repoId, commitLocalId)
}

func (c Ctx) PreventCommit() {
	c.db.PreventCommit(c.ctx)
}
func (c Ctx) ShouldCommit() bool {
	return c.db.ShouldCommit(c.ctx)
}

func (c Ctx) GetTreeData(repoId uint64, treePath string, v uint64) (td treev.TreeDataV, isNotFoundErr bool, e error) {
	return c.db.GetTreeData(c.ctx, repoId, treePath, v)
}
func (c Ctx) GetTreeBlob(repoId uint64, treePath string, v uint64) (r io.Reader, closeR func(), isNotFoundErr bool, e error) {
	return c.db.GetTreeBlob(c.ctx, repoId, treePath, v)
}
func (c Ctx) GetLastVersionOfRootTree(repoId uint64) (v uint64, isNotFoundErr bool, e error) {
	return c.db.GetLastVersionOfRootTree(c.ctx, repoId)
}
func (c Ctx) SetTreeData(quotaOwner string, repoId uint64, treePath string, td treev.TreeDataV) (uint64, error) {
	return c.db.SetTreeData(c.ctx, quotaOwner, repoId, treePath, td)
}
func (c Ctx) SetTreeBlob(quotaOwner string, repoId uint64, treePath string, wt io.WriterTo) (uint64, error) {
	return c.db.SetTreeBlob(c.ctx, quotaOwner, repoId, treePath, wt)
}

func (c Ctx) SetCommit(quotaOwner string, repoId uint64, cm commit.Commit) error {
	return c.db.SetCommit(c.ctx, quotaOwner, repoId, cm)
}
func (c Ctx) GetLatestCommitByLocalId(repoId uint64, L uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetLatestCommitByLocalId(c.ctx, repoId, L)
}
func (c Ctx) GetCommitVersionByLocalId(repoId uint64, L uint64, v uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetCommitVersionByLocalId(c.ctx, repoId, L, v)
}
func (c Ctx) GetLatestCommitByServerId(repoId uint64, ServerL uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetLatestCommitByServerId(c.ctx, repoId, ServerL)
}
func (c Ctx) GetCommitVersionByServerId(repoId uint64, ServerL uint64, ServerV uint64) (cm commit.Commit, isNotFoundErr bool, e error) {
	return c.db.GetCommitVersionByServerId(c.ctx, repoId, ServerL, ServerV)
}
func (c Ctx) GetCommitChildren(repoId uint64, commitL uint64, commitV uint64, maxRowsToRead int) (children []commit.LocalId, childrenVersions []uint64, e error) {
	return c.db.GetCommitChildren(c.ctx, repoId, commitL, commitV, maxRowsToRead)
}
func (c Ctx) GetPendingCommits(ascendingOrder bool, repoId uint64) (iterator.I[commit.Commit], error) {
	return c.db.GetPendingCommits(c.ctx, ascendingOrder, repoId)
}
func (c Ctx) GetPendingCommitsAfter(repoId uint64, afterId commit.LocalId) (iterator.I[commit.Commit], error) {
	return c.db.GetPendingCommitsAfter(c.ctx, repoId, afterId)
}

func (c Ctx) SetBlob(quotaOwner string, idPrefix, id string, wt io.WriterTo) (v blobdb.Version, e error) {
	return c.db.SetBlob(c.ctx, quotaOwner, idPrefix, id, wt)
}
func (c Ctx) GetBlob(idPrefix, id string) (m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return c.db.GetBlob(c.ctx, idPrefix, id)
}
func (c Ctx) GetBlobVersion(idPrefix, id string, v blobdb.Version) (m blobdb.BlobData, r io.Reader, closeReader func(), e error) {
	return c.db.GetBlobVersion(c.ctx, idPrefix, id, v)
}

func (c Ctx) GetQuota(quotaOwner string) (int64, error) {
	return c.db.GetQuota(quotaOwner)
}
func (c Ctx) GetQuotaUsed(quotaOwner string) (success int64, quotaLimitted int64, err error) {
	return c.db.GetQuotaUsed(quotaOwner)
}
func (c Ctx) GetQuotaLeft(quotaOwner string) (int64, error) {
	return c.db.GetQuotaLeft(quotaOwner)
}
func (c Ctx) AddQuota(quotaOwner string, nBytes int64) error {
	return c.db.AddQuota(quotaOwner, nBytes)
}
func (c Ctx) SetQuota(quotaOwner string, nBytes int64) error {
	return c.db.SetQuota(quotaOwner, nBytes)
}
func (c Ctx) FreezeQuota(quotaOwner string) error {
	return c.db.FreezeQuota(quotaOwner)
}
func (c Ctx) ClearQuotaLimitted(quotaOwner string) error {
	return c.db.ClearQuotaLimitted(quotaOwner)
}

func (c Ctx) HasPermission(userId int64, p permissions.Permission, assetId string) (bool, error) {
	return c.db.HasPermission(c.ctx, userId, p, assetId)
}
func (c Ctx) GrantPermissionIfNotExists(userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error) {
	return c.db.GrantPermissionIfNotExists(c.ctx, userId, p, assetId)
}
func (c Ctx) RevokePermissionIfExists(userId int64, p permissions.Permission, assetId string) error {
	return c.db.RevokePermissionIfExists(c.ctx, userId, p, assetId)
}
func (c Ctx) RevokeAllPermissionsToAsset(assetId string) error {
	return c.db.RevokeAllPermissionsToAsset(c.ctx, assetId)
}
func (c Ctx) GetUserPermissions(userId int64) (iterator.I[permissions.Permission], error) {
	return c.db.GetUserPermissions(c.ctx, userId)
}
func (c Ctx) GetUserAssetIdsWithPermission(userId int64, p ...permissions.Permission) (iterator.I[string], error) {
	return c.db.GetUserAssetIdsWithPermission(c.ctx, userId, p...)
}
func (c Ctx) GetUsersWithPermission(assetId string, p permissions.Permission) (iterator.I[int64], error) {
	return c.db.GetUsersWithPermission(c.ctx, assetId, p)
}
func (c Ctx) CountUserAssetsWithPermission(userId int64, permission permissions.Permission) (int64, error) {
	return c.db.CountUserAssetsWithPermission(c.ctx, userId, permission)
}
func (c Ctx) CountUsersWithPermission(assetId string, p permissions.Permission) (int64, error) {
	return c.db.CountUsersWithPermission(c.ctx, assetId, p)
}

// DEPRECATED
func (c Ctx) Exec(query string, args ...any) (sql.Result, error) {
	return c.db.Exec(c.ctx, query, args...)
}

// DEPRECATED
func (c Ctx) QueryRow(query string, args ...any) *sql.Row {
	return c.db.QueryRow(c.ctx, query, args...)
}

// DEPRECATED
func (c Ctx) Query(query string, args ...any) (*sql.Rows, error) {
	return c.db.Query(c.ctx, query, args...)
}

// Writes the mutable columns of the user row. The
// quota fields are not written: they live in the quota db.
func (db WebDb) UpdateUser(writeCtx context.Context, u user.User) error {
	return db.db.UpdateUser(writeCtx, u)
}

// Returns the user by its id. Returns ErrNotFound if there is none.
func (db WebDb) GetUser(ctx context.Context,
	userId int64) (u user.User, isNotFoundErr bool, err error) {
	return db.db.GetUser(ctx, userId)
}

// Returns the user with the email. Returns ErrNotFound if there is none.
func (db WebDb) GetUserByEmail(ctx context.Context,
	email string) (u user.User, isNotFoundErr bool, err error) {
	return db.db.GetUserByEmail(ctx, email)
}

// Returns the user with the username. Returns ErrNotFound if there is none.
func (db WebDb) GetUserByUsername(ctx context.Context,
	username string) (u user.User, isNotFoundErr bool, err error) {
	return db.db.GetUserByUsername(ctx, username)
}

// Returns the user with the stripe id. Returns ErrNotFound if there is none.
func (db WebDb) GetUserByStripeId(ctx context.Context,
	stripeId string) (u user.User, isNotFoundErr bool, err error) {
	return db.db.GetUserByStripeId(ctx, stripeId)
}

// Returns the user with the cli key hash. Returns ErrNotFound if there is none.
func (db WebDb) GetUserByCliKeyHash(ctx context.Context,
	cliKeyHash string) (u user.User, isNotFoundErr bool, err error) {
	return db.db.GetUserByCliKeyHash(ctx, cliKeyHash)
}

// Returns the total number of users.
func (db WebDb) CountUsers(ctx context.Context) (int64, error) {
	return db.db.CountUsers(ctx)
}

// Returns every user, newest first.
func (db WebDb) GetAllUsers(ctx context.Context) (iterator.I[user.User], error) {
	return db.db.GetAllUsers(ctx)
}

// Writes the stripe subscription row, replacing it if the subscription id is
// already stored.
func (db WebDb) UpsertStripeSubscription(writeCtx context.Context,
	stripeSubscriptionId string, userId int64, isActive bool) error {
	return db.db.UpsertStripeSubscription(writeCtx, stripeSubscriptionId, userId, isActive)
}

// Returns whether the stripe subscription is active. Returns ErrNotFound if
// the subscription is not stored.
func (db WebDb) GetStripeSubscriptionIsActive(ctx context.Context,
	stripeSubscriptionId string) (isActive bool, isNotFoundErr bool, err error) {
	return db.db.GetStripeSubscriptionIsActive(ctx, stripeSubscriptionId)
}

// Returns the name that identifies the user in the quota methods.
func (db WebDb) UserQuotaOwnerName(userId int64) string {
	return db.db.UserQuotaOwnerName(userId)
}

// Returns only the username of the user. Returns ErrNotFound if there is none.
func (db WebDb) GetUsername(ctx context.Context,
	userId int64) (username string, isNotFoundErr bool, err error) {
	return db.db.GetUsername(ctx, userId)
}

// Returns whether the user is an organization. Returns ErrNotFound if there is
// no such user.
func (db WebDb) GetUserIsOrganization(ctx context.Context,
	userId int64) (isOrganization bool, isNotFoundErr bool, err error) {
	return db.db.GetUserIsOrganization(ctx, userId)
}

// Inserts a user row and returns the id assigned to it. The columns that are
// not taken here keep their table default.
func (db WebDb) CreateUser(writeCtx context.Context, email string,
	state user.UserState, isOrganization bool, username, passwordHash string,
	selfPaidSubscription user.SubscriptionPlan,
	selfPaidSubscriptionQuantity int64) (userId int64, err error) {
	return db.db.CreateUser(writeCtx, email, state, isOrganization, username,
		passwordHash, selfPaidSubscription, selfPaidSubscriptionQuantity)
}

// Writes only the stripe id of the user row.
func (db WebDb) SetUserStripeId(writeCtx context.Context, userId int64,
	stripeId string) error {
	return db.db.SetUserStripeId(writeCtx, userId, stripeId)
}

// Adds a job to the track queue. Does nothing if the job is already queued.
func (db WebDb) InsertTrackQueueJobIfNotExists(writeCtx context.Context, jobId string,
	ownerId int64, payload []byte, status string, createdAtNs int64) error {
	return db.db.InsertTrackQueueJobIfNotExists(writeCtx, jobId, ownerId, payload, status,
		createdAtNs)
}

// Returns how many jobs are in the track queue.
func (db WebDb) CountTrackQueueJobs(ctx context.Context) (int64, error) {
	return db.db.CountTrackQueueJobs(ctx)
}

// Starts tracking the owner's usage. Does nothing if the owner is already
// tracked, so the limits already set for it are kept.
func (db WebDb) InsertZeroTrackOwnerUsageIfNotExists(writeCtx context.Context,
	ownerId int64) error {
	return db.db.InsertZeroTrackOwnerUsageIfNotExists(writeCtx, ownerId)
}

// Returns the limits set for the owner. Returns ErrNotFound if the owner is
// not tracked.
func (db WebDb) GetTrackOwnerLimits(ctx context.Context,
	ownerId int64) (maxJobs int64, maxTimeoutMs int64, isNotFoundErr bool,
	err error) {
	return db.db.GetTrackOwnerLimits(ctx, ownerId)
}

// Sets the limits of the owner. Starts tracking the owner with zero usage if
// it is not tracked yet; the usage of an already tracked owner is kept.
func (db WebDb) SetTrackOwnerLimits(writeCtx context.Context, ownerId int64,
	maxJobs int64, maxTimeoutMs int64) error {
	return db.db.SetTrackOwnerLimits(writeCtx, ownerId, maxJobs, maxTimeoutMs)
}

// Returns the owner and the payload of the queued job. Returns ErrNotFound if
// the job is not queued.
func (db WebDb) GetTrackQueueJobOwnerAndPayload(ctx context.Context,
	jobId string) (ownerId int64, payload []byte, isNotFoundErr bool, err error) {
	return db.db.GetTrackQueueJobOwnerAndPayload(ctx, jobId)
}

// Removes the job from the track queue.
func (db WebDb) DeleteTrackQueueJob(writeCtx context.Context, jobId string) error {
	return db.db.DeleteTrackQueueJob(writeCtx, jobId)
}

// Adds the deltas to the usage the owner is currently running. The deltas are
// negative when a job stops running.
func (db WebDb) AddTrackOwnerUsage(writeCtx context.Context, ownerId int64,
	runningJobsDelta int64, runningTimeoutMsDelta int64) error {
	return db.db.AddTrackOwnerUsage(writeCtx, ownerId, runningJobsDelta,
		runningTimeoutMsDelta)
}

// Returns the oldest job with the status whose owner is running less than the
// limits set for it. Returns ErrNotFound when no job is within the limits.
func (db WebDb) GetOldestTrackQueueJobWithinOwnerLimits(ctx context.Context,
	status string) (jobId string, ownerId int64, payload []byte,
	isNotFoundErr bool, err error) {
	return db.db.GetOldestTrackQueueJobWithinOwnerLimits(ctx, status)
}

// Sets the status of the queued job.
func (db WebDb) SetTrackQueueJobStatus(writeCtx context.Context, jobId string,
	status string) error {
	return db.db.SetTrackQueueJobStatus(writeCtx, jobId, status)
}

// Returns the ids of the jobs with the status, oldest first.
func (db WebDb) GetTrackQueueJobIdsByStatus(ctx context.Context,
	status string) (iterator.I[string], error) {
	return db.db.GetTrackQueueJobIdsByStatus(ctx, status)
}
