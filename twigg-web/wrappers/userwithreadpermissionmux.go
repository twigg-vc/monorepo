package wrappers

import (
	"context"
	"log"
	"monorepo/twigg-web/featureflags"
	perm "monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/session"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webdb"
	"net/http"
)

type userWithReadPermissionMux struct {
	configName     string
	mux            RlMux
	sessionService session.Service
	db             webdb.WebDb
	repoSrv        repo.Service
	userSrv        user.Service
}

func (m userWithReadPermissionMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r UserWithReadPermissionMuxRequest, dbRead context.Context)) {

	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		dbRead, closeDbRead, err := m.db.BeginRead()
		defer closeDbRead()
		if err != nil {
			log.Printf("failed to get tx in read permission mux: %q", err)
			http.Error(w, "err getting tx", http.StatusInternalServerError)
			return
		}
		// Resolve the session if there is one. Requests without a valid
		// session proceed as an anonymous user (zero-value user).
		var usr user.User
		isLoggedIn := false
		userId, _, _, sessionOk := m.sessionService.ReadSessionCookie(r)
		if sessionOk {
			var isUserNotFoundErr bool
			var err error
			usr, isUserNotFoundErr, err = m.userSrv.Get(dbRead, userId)
			if err != nil {
				http.Error(w, "error getting user", http.StatusInternalServerError)
				return
			}
			if !isUserNotFoundErr {
				isLoggedIn = true
			}
		}
		// Get owner
		ownerName := r.PathValue(routes.RepoOwnerParamName)
		if ownerName == "" {
			http.Error(w, "invalid repo", http.StatusBadRequest)
			return
		}
		ownerUsr, isNotFoundErr, err := m.userSrv.GetByUsername(dbRead, ownerName)
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
		repo, isNotFoundErr, err := m.repoSrv.GetByOwnerIdAndRepoName(dbRead, ownerUsr.Id, repoName)
		if isNotFoundErr {
			http.Error(w, "repo no found", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "error getting repo", http.StatusInternalServerError)
			return
		}
		// Validate permission. Public repos are readable by anyone, including
		// anonymous users. Private repos require a logged in user that is the
		// owner or has read or write permission.
		if !repo.IsPublic {
			if !isLoggedIn {
				http.Redirect(w, r, routes.LoginPage, http.StatusSeeOther)
				return
			}
			if usr.Id != ownerUsr.Id {
				hasPerm, err := m.db.HasPermission(dbRead,
					usr.Id,
					perm.Permission_ReadRepo,
					perm.RepoAssetId(repo.Id))
				if err != nil {
					http.Error(w, "could not validate permission",
						http.StatusInternalServerError)
					return
				}
				if !hasPerm {
					hasPerm, err = m.db.HasPermission(dbRead,
						usr.Id,
						perm.Permission_WriteRepo,
						perm.RepoAssetId(repo.Id))
					if err != nil {
						http.Error(w, "could not validate permission",
							http.StatusInternalServerError)
						return
					}
				}
				if !hasPerm {
					http.Error(w, "user does not have permission",
						http.StatusForbidden)
					return
				}
			}
		}
		handler(w, UserWithReadPermissionMuxRequest{
			Request:                     r,
			MaybeUserWithReadPermission: &usr,
			IsLoggedIn:                  isLoggedIn,
			Repo:                        repo,
			RepoOwnerUsr:                ownerUsr,
			Flags:                       featureflags.GetFlags(m.configName, ownerUsr.Username, usr.Username),
		}, dbRead)
	})
}