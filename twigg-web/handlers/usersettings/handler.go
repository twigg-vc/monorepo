package usersettings

import (
	"context"
	"log"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/keys"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/webdb"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strings"
)

type handler struct {
	userS       UserService
	repoS       RepoService
	keysService keys.Service
	trackQueue  TrackQueue
	db          webdb.WebDb
}

func (hl handler) handleGetSetUsernamePage(w http.ResponseWriter,
	r wrappers.UserMuxRequest, dbRead context.Context) {
	if r.User.State != user.UserState_NoUsername {
		http.Redirect(w, r.Request, routes.Home, http.StatusSeeOther)
		return
	}

	twiggwc.Page( /*hideNavBar=*/ true,
		r.Flags,
		twiggwc.SetUsernamePage()).Render(w)
}
func (hl handler) handlePostSetUsername(w http.ResponseWriter,
	r wrappers.UserMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	if r.User.State != user.UserState_NoUsername {
		http.Error(w, "user not allowed to alter its username",
			http.StatusBadRequest)
		return
	}

	username := r.FormValue(routes.SetUsernameParamName)
	username = strings.ToLower(username)
	if !userservice.UsernameIsValid(username) {
		http.Error(w, "invalid username", http.StatusBadRequest)
		return
	}

	_, err := hl.userS.ChooseUsernameAndStartTrial(dbWrite, r.User.Id, username)
	if err != nil {
		http.Error(w, "internal error updating username", http.StatusInternalServerError)
		return
	}

	hasRepos, err := hl.repoS.NonArchivedRepoCountIsGreaterThan(dbWrite, r.User.Id, 0)
	if err != nil {
		log.Printf("failed to count repos: %s", err)
		http.Error(w, "internal error counting repos", http.StatusInternalServerError)
		return
	}
	if !hasRepos {
		_, _, err = hl.repoS.CreateNew(dbWrite, r.User.Id,
			repo.DemoRepoName, repo.DemoRepoDescription)
		if err != nil {
			log.Printf("failed to create demo repo: %s", err)
			http.Error(w, "internal error creating demo repo", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r.Request, routes.Home, http.StatusSeeOther)
	shouldCommit = true
	return
}

func (hl handler) handleGet(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbRead context.Context) {
	maxJobs, maxTimeout, err := hl.trackQueue.GetLimits(r.UserWithSub.Id, dbRead)
	if err != nil {
		log.Printf("failed to get job limits: %s", err)
		http.Error(w, "failed to get job limits",
			http.StatusInternalServerError)
		return
	}
	twiggwc.Page( /*hideNavBar=*/ false,
		r.Flags,
		twiggwc.UserSettings(r.UserWithSub,
			maxJobs, maxTimeout.Milliseconds()),
	).Render(w)
}

func (hl handler) handleGenerateCliKey(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {

	key := hl.keysService.NewRandomCliKey()
	err := hl.userS.UpdateCliKey(dbWrite, r.UserWithSub.Id, key)
	if err != nil {
		http.Error(w, "internal error updating key",
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, err = w.Write([]byte(key))
	if err != nil {
		http.Error(w, "could not write response",
			http.StatusInternalServerError)
		return
	}
	shouldCommit = true
	return
}

func (h handler) handleDeleteCliKey(w http.ResponseWriter, r wrappers.UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	key := r.URL.Query().Get(routes.CliKeyParamName)
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	err := h.userS.DeleteCliKey(dbWrite, r.UserWithSub.Id)
	if err != nil {
		http.Error(w, "internal error deleting key",
			http.StatusInternalServerError)
		return
	}
	shouldCommit = true
	return
}
