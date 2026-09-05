package wrappers

import (
	"context"
	"monorepo/twigg-web/featureflags"
	perm "monorepo/twigg-web/permissions"
	userservice "monorepo/twigg-web/services/user"
	"net/http"
)

type orgOwnerMux struct {
	configName     string
	userWithSubMux UserWithSubMux
	userSrv        userservice.Service
}

func (m orgOwnerMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r OrgOwnerMuxRequest, dbRead context.Context)) {

	m.userWithSubMux.HandleFuncR(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbRead context.Context) {

		if !userWithSubR.HaveOrgParamInRequest {
			http.Error(w, "no org param found in request", http.StatusBadRequest)
			return
		}
		if userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationOwner {
			http.Error(w, "user does not have owner permission", http.StatusForbidden)
			return
		}

		handler(w, OrgOwnerMuxRequest{
			Request:                 userWithSubR.Request,
			UserWithOwnerPermission: userWithSubR.UserWithSub,
			Org:                     userWithSubR.OrgWithSub,
			Flags:                   featureflags.GetFlags(m.configName, "", userWithSubR.UserWithSub.Username),
		}, dbRead)
	})
}
func (m orgOwnerMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r OrgOwnerMuxRequest, dbWrite context.Context) (shouldCommit bool)) {

	m.userWithSubMux.HandleFuncW(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {

		if !userWithSubR.HaveOrgParamInRequest {
			http.Error(w, "no org param found in request", http.StatusBadRequest)
			return
		}
		if userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationOwner {
			http.Error(w, "user does not have owner permission", http.StatusForbidden)
			return
		}

		return handler(w, OrgOwnerMuxRequest{
			Request:                 userWithSubR.Request,
			UserWithOwnerPermission: userWithSubR.UserWithSub,
			Org:                     userWithSubR.OrgWithSub,
			Flags:                   featureflags.GetFlags(m.configName, "", userWithSubR.UserWithSub.Username),
		}, dbWrite)
	})
}

type orgMemberMux struct {
	configName     string
	userWithSubMux UserWithSubMux
	userSrv        userservice.Service
}

func (m orgMemberMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r OrgMemberMuxRequest, dbRead context.Context)) {

	m.userWithSubMux.HandleFuncR(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbRead context.Context) {

		if !userWithSubR.HaveOrgParamInRequest {
			http.Error(w, "no org param found in request", http.StatusBadRequest)
			return
		}
		if userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationMember {
			http.Error(w, "user does not have member permission", http.StatusForbidden)
			return
		}

		handler(w, OrgMemberMuxRequest{
			Request:                  userWithSubR.Request,
			UserWithMemberPermission: userWithSubR.UserWithSub,
			Org:                      userWithSubR.OrgWithSub,
			Flags:                    featureflags.GetFlags(m.configName, "", userWithSubR.UserWithSub.Username),
		}, dbRead)
	})
}
func (m orgMemberMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r OrgMemberMuxRequest, dbWrite context.Context) (shouldCommit bool)) {

	m.userWithSubMux.HandleFuncW(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {

		if !userWithSubR.HaveOrgParamInRequest {
			http.Error(w, "no org param found in request", http.StatusBadRequest)
			return
		}
		if userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationMember {
			http.Error(w, "user does not have member permission", http.StatusForbidden)
			return
		}

		return handler(w, OrgMemberMuxRequest{
			Request:                  userWithSubR.Request,
			UserWithMemberPermission: userWithSubR.UserWithSub,
			Org:                      userWithSubR.OrgWithSub,
			Flags:                    featureflags.GetFlags(m.configName, "", userWithSubR.UserWithSub.Username),
		}, dbWrite)
	})
}

type orgOwnerOrMemberMux struct {
	configName     string
	userWithSubMux UserWithSubMux
	userSrv        userservice.Service
}

func (m orgOwnerOrMemberMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r OrgOwnerOrMemberMuxRequest, dbRead context.Context)) {

	m.userWithSubMux.HandleFuncR(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbRead context.Context) {

		if !userWithSubR.HaveOrgParamInRequest {
			http.Error(w, "no org param found in request", http.StatusBadRequest)
			return
		}
		if userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationOwner &&
			userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationMember {
			http.Error(w, "user does not have owner or member permission", http.StatusForbidden)
			return
		}

		handler(w, OrgOwnerOrMemberMuxRequest{
			Request:                         userWithSubR.Request,
			UserWithOwnerOrMemberPermission: userWithSubR.UserWithSub,
			Org:                             userWithSubR.OrgWithSub,
			Flags:                           featureflags.GetFlags(m.configName, "", userWithSubR.UserWithSub.Username),
		}, dbRead)
	})
}
func (m orgOwnerOrMemberMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r OrgOwnerOrMemberMuxRequest, dbWrite context.Context) (shouldCommit bool)) {

	m.userWithSubMux.HandleFuncW(pattern, func(w http.ResponseWriter,
		userWithSubR UserWithSubMuxRequest, dbWrite context.Context) (shouldCommit bool) {

		if !userWithSubR.HaveOrgParamInRequest {
			http.Error(w, "no org param found in request", http.StatusBadRequest)
			return
		}
		if userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationOwner &&
			userWithSubR.UserPermissionInOrg != perm.Permission_OrganizationMember {
			http.Error(w, "user does not have owner or member permission", http.StatusForbidden)
			return
		}

		return handler(w, OrgOwnerOrMemberMuxRequest{
			Request:                         userWithSubR.Request,
			UserWithOwnerOrMemberPermission: userWithSubR.UserWithSub,
			Org:                             userWithSubR.OrgWithSub,
			Flags:                           featureflags.GetFlags(m.configName, "", userWithSubR.UserWithSub.Username),
		}, dbWrite)
	})
}
