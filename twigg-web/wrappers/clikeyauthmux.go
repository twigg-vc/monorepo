package wrappers

import (
	"context"
	"log"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/services/repo"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg/xchange"
	"net/http"
)

type cliKeyAuthMux struct {
	configName string
	db         CliKeyAuthMuxDb
	repoSrv    repo.Service
	userSrv    userservice.Service
	mux        RlMux
}

func (m cliKeyAuthMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r CliKeyAuthMuxRequest, dbWrite context.Context) (shouldCommit bool)) {

	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		xchange.SetTwiggHeaderInResponse(w)
		key := xchange.GetApiKeyHeader(r)
		if key == "" {
			http.Error(w, xchange.BadApiKeyErrMsg, http.StatusForbidden)
			return
		}
		dbWrite, closeDbWrite, commit, err := m.db.BeginWrite()
		defer closeDbWrite()
		if err != nil {
			log.Printf("failed to get tx in cli key auth mux: %q", err)
			http.Error(w, "err getting tx", http.StatusInternalServerError)
			return
		}
		u, isNotFoundErr, err := m.userSrv.GetByCliKey(dbWrite, key)
		if isNotFoundErr {
			http.Error(w, xchange.BadApiKeyErrMsg, http.StatusForbidden)
			return
		}
		if err != nil {
			log.Printf("failed to get user by cli key: %q", err)
			http.Error(w, "err getting user", http.StatusInternalServerError)
			return
		}
		if !u.HasSub() {
			http.Error(w, "no subscription", http.StatusForbidden)
			return
		}
		hasMoreThanTwoRepos, err := m.repoSrv.NonArchivedRepoCountIsGreaterThan(
			dbWrite, u.Id, 2)
		if err != nil {
			log.Printf("failed to get repo count>2 for userId=%d: %s", u.Id, err)
			http.Error(w, "failed to get repo count", http.StatusInternalServerError)
			return
		}
		if u.MustUpgradeSelfPaidSub(hasMoreThanTwoRepos) {
			http.Error(w, "user must upgrade plan", http.StatusForbidden)
			return
		}

		ownerUsr, rp, ok := resolveRepoAndValidateWritePerm(w, r, dbWrite,
			u.Id, m.db, m.repoSrv, m.userSrv)
		if !ok {
			return
		}
		if !ownerUsr.HasSub() {
			http.Error(w, "no subscription from owner", http.StatusForbidden)
			return
		}
		repoOwnerHasMoreThanTwoRepos, err := m.repoSrv.NonArchivedRepoCountIsGreaterThan(
			dbWrite, ownerUsr.Id, 2)
		if err != nil {
			log.Printf("failed to get repo count>2 for userId=%d: %s", rp.OwnerId, err)
			http.Error(w, "failed to get repo count", http.StatusInternalServerError)
			return
		}
		if ownerUsr.MustUpgradeSelfPaidSub(repoOwnerHasMoreThanTwoRepos) {
			http.Error(w, "repo owner must upgrade plan", http.StatusForbidden)
			return
		}

		shouldCommit := handler(w, CliKeyAuthMuxRequest{
			Request:                 r,
			UserWithWritePermission: u,
			Repo:                    rp,
			RepoOwnerUsr:            ownerUsr,
			Flags:                   featureflags.GetFlags(m.configName, ownerUsr.Username, u.Username),
		}, dbWrite)
		if shouldCommit {
			err = commit()
			if err != nil {
				log.Printf("err commiting tx in cli key auth mux: %q", err)
				http.Error(w, "err committing write tx",
					http.StatusInternalServerError)
				return
			}
		}
	})
}
