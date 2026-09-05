package wrappers

import (
	"context"
	"log"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	"net/http"
	"net/url"
)

type userWithSubMux struct {
	userMux     UserMux
	repoService repo.Service
}

func (m userWithSubMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r UserWithSubMuxRequest, dbRead context.Context)) {
	m.userMux.HandleFuncR(pattern, func(w http.ResponseWriter, r UserMuxRequest, dbRead context.Context) {
		if m.redirectIfNeeded(w, r, dbRead) {
			return
		}
		handler(w, UserWithSubMuxRequest{
			Request:               r.Request,
			UserWithSub:           r.User,
			HaveOrgParamInRequest: r.HaveOrgParamInRequest,
			OrgWithSub:            r.Org,
			UserPermissionInOrg:   r.UserPermissionInOrg,
			Flags:                 r.Flags},
			dbRead,
		)
	})
}
func (m userWithSubMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool)) {
	m.userMux.HandleFuncW(pattern, func(w http.ResponseWriter,
		r UserMuxRequest, dbWrite context.Context) (shouldCommit bool) {
		if m.redirectIfNeeded(w, r, dbWrite) {
			return
		}
		return handler(w, UserWithSubMuxRequest{
			Request:               r.Request,
			UserWithSub:           r.User,
			HaveOrgParamInRequest: r.HaveOrgParamInRequest,
			OrgWithSub:            r.Org,
			UserPermissionInOrg:   r.UserPermissionInOrg,
			Flags:                 r.Flags},
			dbWrite,
		)
	})
}

// Helper that centralizes the logic for redirecting or not
func (m userWithSubMux) redirectIfNeeded(w http.ResponseWriter,
	r UserMuxRequest, dbRead context.Context) (redirected bool) {
	redirected = true
	if r.User.Username == "" {
		http.Redirect(w, r.Request, routes.SetUsernamePath, http.StatusSeeOther)
		return
	}
	if !r.User.HasSub() {
		http.Redirect(w, r.Request, routes.PlansPage, http.StatusSeeOther)
		return
	}
	hasMoreThanTwoRepos, err := m.repoService.NonArchivedRepoCountIsGreaterThan(
		dbRead, r.User.Id, 2)
	if err != nil {
		log.Printf("failed to get repo count>2 for userId=%d: %s", r.User.Id, err)
		http.Error(w, "failed to get repo count", http.StatusInternalServerError)
		return
	}
	if r.User.MustUpgradeSelfPaidSub(hasMoreThanTwoRepos) {
		http.Redirect(w, r.Request, routes.NeedUpgradePage, http.StatusSeeOther)
		return
	}

	if r.HaveOrgParamInRequest && !r.Org.HasSub() {
		q := url.Values{}
		q.Set(routes.IsChoosingPlanForOrgParamName, "true")
		q.Set(routes.OrganizationNameParamName, r.Org.Username)

		redirectURL := routes.PlansPage + "?" + q.Encode()

		http.Redirect(w, r.Request, redirectURL, http.StatusSeeOther)
		return
	}

	redirected = false
	return
}
