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
		dbWrite, closeDbWrite, commit, err := m.db.BeginWrite()
		defer closeDbWrite()
		if err != nil {
			log.Printf("failed to get tx in cli key auth mux: %q", err)
			http.Error(w, "err getting tx", http.StatusInternalServerError)
			return
		}
		req, ok := m.authenticate(w, r, dbWrite)
		if !ok {
			return
		}
		shouldCommit := handler(w, req, dbWrite)
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

func (m cliKeyAuthMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r CliKeyAuthMuxRequest, dbRead context.Context)) {

	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		xchange.SetTwiggHeaderInResponse(w)
		dbRead, closeDbRead, err := m.db.BeginRead()
		defer closeDbRead()
		if err != nil {
			log.Printf("failed to get read tx in cli key auth mux: %q", err)
			http.Error(w, "err getting tx", http.StatusInternalServerError)
			return
		}
		req, ok := m.authenticate(w, r, dbRead)
		if !ok {
			return
		}
		handler(w, req, dbRead)
	})
}

// Validates the api key, the subscriptions of the user and of the repo owner,
// and that the user is the owner or has write permission in the repo. On any
// failure, writes an error to the response and returns ok=false.
func (m cliKeyAuthMux) authenticate(w http.ResponseWriter, r *http.Request,
	tx context.Context) (req CliKeyAuthMuxRequest, ok bool) {
	key := xchange.GetApiKeyHeader(r)
	if key == "" {
		http.Error(w, xchange.BadApiKeyErrMsg, http.StatusForbidden)
		return CliKeyAuthMuxRequest{}, false
	}
	u, isNotFoundErr, err := m.userSrv.GetByCliKey(tx, key)
	if isNotFoundErr {
		http.Error(w, xchange.BadApiKeyErrMsg, http.StatusForbidden)
		return CliKeyAuthMuxRequest{}, false
	}
	if err != nil {
		log.Printf("failed to get user by cli key: %q", err)
		http.Error(w, "err getting user", http.StatusInternalServerError)
		return CliKeyAuthMuxRequest{}, false
	}
	if !u.HasSub() {
		http.Error(w, "no subscription", http.StatusForbidden)
		return CliKeyAuthMuxRequest{}, false
	}
	hasMoreThanTwoRepos, err := m.repoSrv.NonArchivedRepoCountIsGreaterThan(
		tx, u.Id, 2)
	if err != nil {
		log.Printf("failed to get repo count>2 for userId=%d: %s", u.Id, err)
		http.Error(w, "failed to get repo count", http.StatusInternalServerError)
		return CliKeyAuthMuxRequest{}, false
	}
	if u.MustUpgradeSelfPaidSub(hasMoreThanTwoRepos) {
		http.Error(w, "user must upgrade plan", http.StatusForbidden)
		return CliKeyAuthMuxRequest{}, false
	}

	ownerUsr, rp, ok := resolveRepoAndValidateWritePerm(w, r, tx,
		u.Id, m.db, m.repoSrv, m.userSrv)
	if !ok {
		return CliKeyAuthMuxRequest{}, false
	}
	if !ownerUsr.HasSub() {
		http.Error(w, "no subscription from owner", http.StatusForbidden)
		return CliKeyAuthMuxRequest{}, false
	}
	repoOwnerHasMoreThanTwoRepos, err := m.repoSrv.NonArchivedRepoCountIsGreaterThan(
		tx, ownerUsr.Id, 2)
	if err != nil {
		log.Printf("failed to get repo count>2 for userId=%d: %s", rp.OwnerId, err)
		http.Error(w, "failed to get repo count", http.StatusInternalServerError)
		return CliKeyAuthMuxRequest{}, false
	}
	if ownerUsr.MustUpgradeSelfPaidSub(repoOwnerHasMoreThanTwoRepos) {
		http.Error(w, "repo owner must upgrade plan", http.StatusForbidden)
		return CliKeyAuthMuxRequest{}, false
	}

	return CliKeyAuthMuxRequest{
		Request:                 r,
		UserWithWritePermission: u,
		Repo:                    rp,
		RepoOwnerUsr:            ownerUsr,
		Flags:                   featureflags.GetFlags(m.configName, ownerUsr.Username, u.Username),
	}, true
}
