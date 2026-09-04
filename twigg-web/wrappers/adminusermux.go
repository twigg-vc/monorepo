package wrappers

import (
	"context"
	"monorepo/twigg-web/routes"
	"net/http"
)

type adminUserMux struct {
	userMux     UserMux
	adminEmails []string
}

func (m adminUserMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r AdminUserMuxRequest, dbRead context.Context)) {
	m.userMux.HandleFuncR(pattern, func(w http.ResponseWriter, r UserMuxRequest, dbRead context.Context) {
		ok := false
		for _, email := range m.adminEmails {
			if r.User.Email == email {
				ok = true
				break
			}
		}
		if !ok {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusUnauthorized)
			return
		}
		handler(w, AdminUserMuxRequest{
			Request:   r.Request,
			AdminUser: r.User,
			Flags:     r.Flags},
			dbRead,
		)
	})
}
func (m adminUserMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r AdminUserMuxRequest, dbWrite context.Context) (shouldCommit bool)) {
	m.userMux.HandleFuncW(pattern, func(w http.ResponseWriter,
		r UserMuxRequest, dbWrite context.Context) (shouldCommit bool) {
		ok := false
		for _, email := range m.adminEmails {
			if r.User.Email == email {
				ok = true
				break
			}
		}
		if !ok {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusUnauthorized)
			return
		}
		return handler(w, AdminUserMuxRequest{
			Request:   r.Request,
			AdminUser: r.User,
			Flags:     r.Flags},
			dbWrite,
		)
	})
}
