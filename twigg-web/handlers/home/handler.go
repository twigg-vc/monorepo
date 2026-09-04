package home

import (
	"context"
	perm "monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
	"net/http"
)

type handler struct {
	db   webdb.WebDb
	rSrv repo.Service
	rt   routes.Router
	uSrv user.Service
}

func newHandler(db webdb.WebDb,
	rSrv repo.Service,
	rt routes.Router,
	uSrv user.Service) handler {
	return handler{
		db:   db,
		rSrv: rSrv,
		rt:   rt,
		uSrv: uSrv,
	}
}

func (hl handler) handleGet(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbRead context.Context) {

	myRepos, err := hl.rSrv.GetAllByOwnerId(dbRead,
		r.UserWithSub.Id)
	if err != nil {
		http.Error(w, "internal error getting all user repositories",
			http.StatusInternalServerError)
		return
	}
	const maxMyRepos = 100
	var myReposList []webcomponents.FrontendRepo
	for myRepos.Next() {
		myRepo, err := myRepos.Get()
		if err != nil {
			http.Error(w, "internal error getting my repo",
				http.StatusInternalServerError)
			return
		}
		myReposList = append(myReposList,
			webcomponents.NewFrontendRepo(
				r.UserWithSub.Username, myRepo))
		if len(myReposList) > maxMyRepos {
			http.Error(w, "too many my repos", http.StatusInternalServerError)
			return
		}
	}
	err = myRepos.Err()
	if err != nil {
		http.Error(w, "internal error iterating my repos",
			http.StatusInternalServerError)
		return
	}

	const maxSharedRepos = 100
	var sharedReposList []webcomponents.FrontendRepo
	sharedRepos, err := hl.db.GetUserAssetIdsWithPermission(
		dbRead, r.UserWithSub.Id,
		perm.Permission_ReadRepo, perm.Permission_WriteRepo)
	if err != nil {
		http.Error(w, "internal error getting shared repos",
			http.StatusInternalServerError)
		return
	}
	for sharedRepos.Next() {
		sharedRepoId, err := sharedRepos.Get()
		if err != nil {
			http.Error(w, "internal error getting shared repo id",
				http.StatusInternalServerError)
			return
		}
		sharedRepo, err := hl.rSrv.GetById(
			dbRead, perm.ParseRepoAssetIdOrDie(sharedRepoId))
		if err != nil {
			http.Error(w, "internal error getting shared repo",
				http.StatusInternalServerError)
			return
		}
		ownerUser, _, err := hl.uSrv.Get(dbRead, sharedRepo.OwnerId)
		if err != nil {
			http.Error(w, "internal error getting shared repo owner",
				http.StatusInternalServerError)
			return
		}
		sharedReposList = append(sharedReposList,
			webcomponents.NewFrontendRepo(ownerUser.Username, sharedRepo))
		if len(myReposList) > maxSharedRepos {
			http.Error(w, "too many shared repos", http.StatusInternalServerError)
			return
		}
	}
	err = sharedRepos.Err()
	if err != nil {
		http.Error(w, "internal error iterating shared repos",
			http.StatusInternalServerError)
		return
	}

	webcomponents.Page( /*hideNavBar=*/ false,
		r.Flags,
		webcomponents.Home(r.UserWithSub,
			myReposList, sharedReposList),
	).Render(w)
}
