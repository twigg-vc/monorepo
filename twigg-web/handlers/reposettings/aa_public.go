package reposettings

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/secrets"
	"monorepo/twigg-web/services/mirror"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/server"
)

type handler struct {
	userS     UserService
	repoS     RepoService
	mirrorSrv mirror.GitMirrorService
	secrets   Secrets
	db        Db
}

func AddHandlers(userRepoMux wrappers.UserRepoMux,
	cliKeyAuthMux wrappers.CliKeyAuthMux, userS UserService,
	db Db, repoS RepoService,
	queueR Queue, mirrorSrv mirror.GitMirrorService, secrets Secrets) {

	h := handler{
		userS:     userS,
		repoS:     repoS,
		mirrorSrv: mirrorSrv,
		secrets:   secrets,
		db:        db,
	}
	userRepoMux.HandleFuncR("GET "+routes.RepoSettingsPattern, h.handleGetRepoSettings)
	// Permissions
	userRepoMux.HandleFuncW("POST "+routes.RepoAddPermissionPattern, h.handlePostAddRepoPermission)
	userRepoMux.HandleFuncW("POST "+routes.RepoRemovePermissionPattern, h.handlePostRemoveRepoPermission)
	// Archive
	userRepoMux.HandleFuncW("POST "+routes.RepoArchivePattern, h.handlePostArchive)
	// Visibility
	userRepoMux.HandleFuncW("POST "+routes.RepoSetPublicPattern, h.handlePostSetPublic)
	userRepoMux.HandleFuncW("POST "+routes.RepoSetPrivatePattern, h.handlePostSetPrivate)
	// Description
	userRepoMux.HandleFuncW("POST "+routes.RepoSetDescriptionPattern, h.handlePostSetDescription)
	// Server id (called by the CLI)
	cliKeyAuthMux.HandleFuncW("POST "+routes.RepoSetServerIdPattern, h.handlePostSetServerId)
	// Git Mirror
	userRepoMux.HandleFuncW("POST "+routes.RepoGitMirrorEnabledPattern, h.handlePostGitMirrorEnabled)
	userRepoMux.HandleFuncW("POST "+routes.RepoGitMirrorUrlPattern, h.handlePostGitMirrorUrl)
	// Secrets
	userRepoMux.HandleFuncW("PUT "+routes.RepoSettingsSecret, h.handlePutSetRepoSecret)
	userRepoMux.HandleFuncW("POST "+routes.RepoSettingsSecret, h.handlePostSetRepoSecret)
	userRepoMux.HandleFuncW("DELETE "+routes.RepoSettingsSecret, h.handleDeleteRepoSecret)

	onHandleQueuePushToGitMirrorDeadLetter := func(p []byte) error {
		return nil
	}
	queueR.Register(pushToGitMirrorPayloadType,
		h.handleQueuePushToGitMirror,
		h.getPayloadDisplayString, onHandleQueuePushToGitMirrorDeadLetter)
}

func PushTopToGitMirrorPayload(repoId uint64,
	gitRepoUrl string) (payloadType string, payload []byte, err error) {
	return pushToGitMirrorPayload(repoId, gitRepoUrl)
}

type Queue interface {
	Register(payloadType string,
		handler func(payload []byte) error,
		decoder func(payload []byte) string,
		onDeadLetter func(payload []byte) error)
}

type Secrets interface {
	SetRepoIdSecret(wl context.Context, repoId uint64, secretName string, secret string) (secrets.SecretRef, error)
	GetRepoIdSecretsPage(rl context.Context, repoId uint64, afterSecretId uint64) ([]secrets.SecretRef, error)
	DeleteRepoIdSecretIfExists(wl context.Context, repoId uint64, secretName string) error
	RepoIdHasSecret(rl context.Context, repoId uint64, secretName string) (bool, error)
}

type UserService interface {
	Get(rl context.Context, id int64) (user.User, bool, error)
	GetByUsername(rl context.Context, username string) (user.User, bool, error)
}

type RepoService interface {
	ArchiveRepo(wl context.Context, ownerId int64, repoId uint64) error
	SetPublic(wl context.Context, ownerId int64, displayName string) error
	SetPrivate(wl context.Context, ownerId int64, displayName string) error
	SetGitMirrorEnabled(wl context.Context, ownerId int64, displayName string, enabled bool) error
	SetGitMirrorUrl(wl context.Context, repoId uint64, ownerId int64, displayName, url string) error
	GetServerByRepoId(rl context.Context, repoId uint64) (server.Server, error)
	GetServer(rl context.Context, ownerId int64, displayName string) (s server.Server, isNotFoundErr bool, err error)
	GetServerRead(rl context.Context) server.Read
	GetServerWrite(wl context.Context) server.Write
}

type Db interface {
	BeginRead() (readCtx context.Context, closeTx func(), err error)
	SetRepoDescription(wl context.Context, ownerId int64, displayName, description string) error
	HasPermission(rl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error)
	GrantPermissionIfNotExists(wl context.Context, userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error)
	RevokePermissionIfExists(wl context.Context, userId int64, p permissions.Permission, assetId string) error
	RevokeAllPermissionsToAsset(wl context.Context, assetId string) error
	GetUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error)
}