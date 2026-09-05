package usersettings

import (
	"context"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
	"time"
)

func AddHandlers(
	userS UserService,
	repoS RepoService,
	keyService KeysService,
	trackQueue TrackQueue,
	userMux wrappers.UserMux,
	userWithSubMux wrappers.UserWithSubMux) {
	h := handler{
		userS:       userS,
		repoS:       repoS,
		keysService: keyService,
		trackQueue:  trackQueue,
	}
	userMux.HandleFuncR("GET "+routes.SetUsernamePath,
		h.handleGetSetUsernamePage)
	userMux.HandleFuncW("POST "+routes.SetUsernamePath,
		h.handlePostSetUsername)

	userWithSubMux.HandleFuncR(
		"GET "+routes.UserSettings, h.handleGet)

	userWithSubMux.HandleFuncW("POST "+routes.GenerateCLIKey,
		h.handleGenerateCliKey)
	userWithSubMux.HandleFuncW("DELETE "+routes.DeleteCLIKey,
		h.handleDeleteCliKey)
}

type KeysService interface {
	NewRandomCliKey() string
}

type TrackQueue interface {
	GetLimits(ownerId int64, tx context.Context) (maxJobs int,
		maxTimeout time.Duration, err error)
}

type UserService interface {
	ChooseUsernameAndStartTrial(w context.Context, id int64, username string) (user.User, error)
	UpdateCliKey(w context.Context, userId int64, key string) error
	DeleteCliKey(w context.Context, userId int64) error
}

type RepoService interface {
	CreateNew(wl context.Context, ownerId int64, displayName string, description string) (r repo.Repo, isAlreadyExistsErr bool, err error)
	NonArchivedRepoCountIsGreaterThan(rl context.Context, ownerId int64, n int) (bool, error)
}
