package twigg

import (
	"context"
	"errors"
	"log"
	"monorepo/twigg-runner/runnerlib"
	perm "monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	reposervice "monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/twiggtoken"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg/commit"
	"monorepo/twigg/server"
	"monorepo/twigg/xchange"
	"net/http"
	"strconv"
	"strings"
)

type handler struct {
	db         webdb.WebDb
	uSrv       userservice.Service
	rSrv       reposervice.Service
	ciq        CiCdQueue
	configName string
	signer     twiggtoken.TokenSigner
}

func (hl handler) handlePull(w http.ResponseWriter, r *http.Request) {
	dbRead, closeDbRead, err := hl.db.BeginRead()
	defer closeDbRead()
	if err != nil {
		http.Error(w, "internal err", http.StatusInternalServerError)
		return
	}

	// If repoName and repoOwnerName are equal and are both numbers,
	// treat them as if they're actually specifying the repoId.
	// This is safe to do because usernames can't be numeric (check the panic
	// below)
	repoName := r.PathValue(routes.RepoNameParamName)
	repoOwnerName := r.PathValue(routes.RepoOwnerParamName)
	repoNameInt, repoNameIntErr := strconv.ParseUint(repoName, 10, 64)
	repoOwnerNameInt, repoOwnerNameIntErr := strconv.ParseUint(repoOwnerName,
		10, 64)
	isId := repoNameIntErr == nil && repoOwnerNameIntErr == nil &&
		repoNameInt == repoOwnerNameInt
	var repo repo.Repo
	var s server.Server
	var ownerUser user.User
	if isId {
		repo, err = hl.rSrv.GetById(dbRead, repoNameInt)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		ownerUser, _, err = hl.uSrv.Get(dbRead, repo.OwnerId)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		s, err = hl.rSrv.GetServerByRepoId(dbRead, repoNameInt)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else {
		var isNotFoundErr bool
		var err error
		ownerUser, isNotFoundErr, err = hl.uSrv.GetByUsername(dbRead,
			repoOwnerName)
		if err != nil && !isNotFoundErr {
			http.Error(w, "could not load owner", http.StatusInternalServerError)
			return
		}
		if isNotFoundErr {
			http.Error(w, "repo owner not found", http.StatusBadRequest)
			return
		}
		s, isNotFoundErr, err = hl.rSrv.GetServer(dbRead, ownerUser.Id, repoName)
		if isNotFoundErr {
			http.Error(w, "repo not found", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		repo, _, err = hl.rSrv.GetByOwnerIdAndRepoName(dbRead,
			ownerUser.Id, repoName)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	ver, _ := newVerifier(r, hl.uSrv, hl.rSrv, dbRead,
		hl.db, ownerUser, repo.Id /*isPull*/, true, hl.signer)
	_ = s.HandlePull(w, r, ver, hl.rSrv.GetServerRead(dbRead))
}

func (hl handler) handlePush(w http.ResponseWriter, r *http.Request) {
	dbWrite, closeDbWrite, commit, err := hl.db.BeginWrite()
	defer closeDbWrite()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	repoName := r.PathValue(routes.RepoNameParamName)
	repoOwnerName := r.PathValue(routes.RepoOwnerParamName)
	ownerUser, isNotFoundErr, err := hl.uSrv.GetByUsername(dbWrite, repoOwnerName)
	if err != nil && !isNotFoundErr {
		http.Error(w, "could not load owner", http.StatusInternalServerError)
		return
	}
	if isNotFoundErr {
		http.Error(w, "repo owner not found", http.StatusBadRequest)
		return
	}

	s, isNotFoundErr, err := hl.rSrv.GetServer(dbWrite, ownerUser.Id, repoName)
	if isNotFoundErr {
		http.Error(w, "repo not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	repo, _, err := hl.rSrv.GetByOwnerIdAndRepoName(dbWrite, ownerUser.Id, repoName)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ver, u := newVerifier(r, hl.uSrv, hl.rSrv, dbWrite,
		hl.db, ownerUser, repo.Id /*isPull*/, false, hl.signer)
	po := pushObserver{
		authorId: u.Id,
	}
	ok := s.HandlePush(w, r, ver, &po, hl.rSrv.GetServerWrite(dbWrite))
	if !ok {
		return
	}

	// Enqueue to CI/CD run
	_, err = hl.ciq.EnqueueCiCdRun(repo.Id,
		po.newCommitId, po.newCommitVersion, runnerlib.OnPush, dbWrite)
	if err != nil {
		log.Printf("failed to enqueue cicd run: %s", err)
		http.Error(w, "internal err enqueing cicd run",
			http.StatusInternalServerError)
		return
	}

	err = commit()
	closeDbWrite()
	if err != nil {
		return
	}
}

// Returns the result of the verifier and the user.
// The user is returned just in case it's needed anywhere else to avoid
// reading it again. Check the verifierResult for errors
func newVerifier(r *http.Request, uSrv userservice.Service, repoSrv reposervice.Service, dbRead context.Context, permSrv webdb.WebDb, repoOwner user.User, repoId uint64, isPull bool, signer twiggtoken.TokenSigner) (verifierResult, user.User) {
	if !repoOwner.HasSub() {
		return verifierResult{
			ok:       false,
			notOkMsg: "no subscription from owner",
			err:      nil,
		}, user.User{}
	}
	moreThanTwoRepos, err := repoSrv.NonArchivedRepoCountIsGreaterThan(
		dbRead, repoOwner.Id, 2)
	if err != nil {
		return verifierResult{
			ok:       false,
			notOkMsg: "",
			err:      errors.New("server error"),
		}, user.User{}
	}
	if repoOwner.MustUpgradeSelfPaidSub(moreThanTwoRepos) {
		return verifierResult{
			ok:       false,
			notOkMsg: "owner must upgrade plan",
			err:      nil,
		}, user.User{}
	}

	key := xchange.GetApiKeyHeader(r)
	keyIsToken := strings.HasPrefix(key, twiggtoken.Prefix)

	if keyIsToken {
		twToken, isExpiredErr, err := twiggtoken.ParseToken(key, signer)
		if isExpiredErr {
			return verifierResult{
				ok:       false,
				notOkMsg: "token expired",
				err:      nil,
			}, user.User{}
		}
		if err != nil {
			return verifierResult{
				ok:       false,
				notOkMsg: xchange.BadTwTokenErrMsg,
				err:      nil,
			}, user.User{}
		}
		if twToken.RepoId != repoId {
			return verifierResult{
				ok:       false,
				notOkMsg: "permission denied: cant use token of another repo",
				err:      nil,
			}, user.User{}
		}
		if isPull {
			if !twToken.Supports(twiggtoken.TokenActionPull, "") {
				return verifierResult{
					ok:       false,
					notOkMsg: "token action must be pull",
					err:      nil,
				}, user.User{}
			}
			return verifierResult{
				ok:       true,
				notOkMsg: "",
				err:      nil,
			}, user.User{}
		} else {
			return verifierResult{
				ok:       false,
				notOkMsg: "permission denied: token auth only works for pull",
				err:      nil,
			}, user.User{}
		}
	}

	u, isNotFoundErr, err := uSrv.GetByCliKey(dbRead, key)
	if isNotFoundErr {
		return verifierResult{
			ok:       false,
			notOkMsg: xchange.BadApiKeyErrMsg,
			err:      nil,
		}, u
	}
	if err != nil {
		return verifierResult{
			ok:       false,
			notOkMsg: "",
			err:      errors.New("server error"),
		}, user.User{}
	}
	if !u.HasSub() {
		return verifierResult{
			ok:       false,
			notOkMsg: "no subscription",
			err:      nil,
		}, u
	}
	if u.Id != repoOwner.Id {
		hasPerm, err := permSrv.HasPermission(dbRead, u.Id, perm.Permission_WriteRepo, perm.RepoAssetId(repoId))
		if err != nil {
			return verifierResult{
				ok:       false,
				notOkMsg: "",
				err:      errors.New("server error"),
			}, user.User{}
		}
		if !hasPerm {
			return verifierResult{
				ok:       false,
				notOkMsg: "permission denied",
				err:      nil,
			}, user.User{}
		}
	}
	return verifierResult{
		ok:       true,
		notOkMsg: "",
		err:      nil,
	}, u
}

type verifierResult struct {
	ok       bool
	notOkMsg string
	err      error
}

func (v verifierResult) PushIsOk(r *http.Request) (ok bool, notOkMsg string, err error) {
	return v.ok, v.notOkMsg, v.err
}
func (v verifierResult) PullIsOk(r *http.Request) (ok bool, notOkMsg string, err error) {
	return v.ok, v.notOkMsg, v.err
}

// PushObserver that sets the authorId of the pushed commits and gets the ids
// of the new commits
type pushObserver struct {
	authorId         int64
	newCommitId      uint64
	newCommitVersion uint64
}

func (p *pushObserver) OnPush(
	newCommit *commit.Commit, hasOld bool, oldCommit commit.Commit) {
	newCommit.AuthorUserId = p.authorId
	p.newCommitId = newCommit.L
	p.newCommitVersion = newCommit.Version
}

// Called just to ensure usernames can't be numeric
var _ int = func() int {
	if userservice.UsernameIsValid("1") ||
		userservice.UsernameIsValid("11") ||
		userservice.UsernameIsValid("111") ||
		userservice.UsernameIsValid("1111111") ||
		userservice.UsernameIsValid("123456789") {
		// The code above assumes that usernames can't be numeric.
		// This is done so that /<number>/<number> are routed to repoId with
		// <number>
		panic("1 is a valid username")
	}
	return 0
}()