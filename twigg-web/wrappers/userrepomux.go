package wrappers

import (
	"context"
	"monorepo/twigg-web/featureflags"
	perm "monorepo/twigg-web/permissions"
	webrepo "monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webdb"
	"net/http"
)

type userRepoMux struct {
	configName     string
	userWithSubMux UserWithSubMux
	permSrv        webdb.WebDb
	repoSrv        repo.Service
	userSrv        user.Service
}

func (m userRepoMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r UserRepoMuxRequest, dbRead context.Context)) {

	m.userWithSubMux.HandleFuncR(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbRead context.Context) {
		ownerUsr, repo, ok := resolveRepoAndValidateWritePerm(w,
			userWithSubR.Request, dbRead, userWithSubR.UserWithSub.Id,
			m.permSrv, m.repoSrv, m.userSrv)
		if !ok {
			return
		}
		handler(w, UserRepoMuxRequest{
			Request:                 userWithSubR.Request,
			UserWithWritePermission: userWithSubR.UserWithSub,
			Repo:                    repo,
			RepoOwnerUsr:            ownerUsr,
			Flags:                   featureflags.GetFlags(m.configName, ownerUsr.Username, userWithSubR.UserWithSub.Username),
		}, dbRead)
	})
}
func (m userRepoMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool)) {

	m.userWithSubMux.HandleFuncW(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {
		ownerUsr, repo, ok := resolveRepoAndValidateWritePerm(w,
			userWithSubR.Request, dbWrite, userWithSubR.UserWithSub.Id,
			m.permSrv, m.repoSrv, m.userSrv)
		if !ok {
			return
		}
		return handler(w, UserRepoMuxRequest{
			Request:                 userWithSubR.Request,
			UserWithWritePermission: userWithSubR.UserWithSub,
			Repo:                    repo,
			RepoOwnerUsr:            ownerUsr,
			Flags:                   featureflags.GetFlags(m.configName, ownerUsr.Username, userWithSubR.UserWithSub.Username),
		}, dbWrite)
	})
}

// Resolves the repo owner and the repo from the request path and validates
// that the user is the repo owner or has write permission in the repo.
// If ok is false, this function already wrote the HTTP response and the
// caller must return immediately without writing anything else.
func resolveRepoAndValidateWritePerm(w http.ResponseWriter, r *http.Request,
	dbRead context.Context, userId int64, permSrv webdb.WebDb,
	repoSrv repo.Service, userSrv user.Service) (
	ownerUsr user.User, repo webrepo.Repo, ok bool) {

	// Get owner
	ownerName := r.PathValue(routes.RepoOwnerParamName)
	if ownerName == "" {
		http.Error(w, "invalid repo", http.StatusBadRequest)
		return
	}
	ownerUsr, isNotFoundErr, err := userSrv.GetByUsername(dbRead, ownerName)
	if isNotFoundErr {
		http.Error(w, "repo no found", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "error getting repo", http.StatusInternalServerError)
		return
	}
	// Get repo
	repoName := r.PathValue(routes.RepoNameParamName)
	if repoName == "" {
		http.Error(w, "invalid repo name", http.StatusBadRequest)
		return
	}
	repo, isNotFoundErr, err = repoSrv.GetByOwnerIdAndRepoName(dbRead, ownerUsr.Id, repoName)
	if isNotFoundErr {
		http.Error(w, "repo no found", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "error getting repo", http.StatusInternalServerError)
		return
	}
	// Validate permission
	if userId != ownerUsr.Id {
		hasPerm, err := permSrv.HasPermission(dbRead,
			userId,
			perm.Permission_WriteRepo,
			perm.RepoAssetId(repo.Id))
		if err != nil {
			http.Error(w, "could not validate permission",
				http.StatusInternalServerError)
			return
		}
		if !hasPerm {
			http.Error(w, "user does not have permission",
				http.StatusForbidden)
			return
		}
	}
	ok = true
	return
}