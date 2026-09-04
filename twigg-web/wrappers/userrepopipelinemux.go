package wrappers

import (
	"context"
	"log"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/jobs"
	"net/http"
)

type userRepoPipelineMux struct {
	u  UserRepoMux
	js jobs.Service
}

func (m userRepoPipelineMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r UserRepoPipelineMuxRequest, dbRead context.Context)) {
	m.u.HandleFuncR(pattern, func(w http.ResponseWriter, r UserRepoMuxRequest, dbRead context.Context) {
		pipelineId := r.Request.PathValue(routes.PipelineIdPathParamName)
		pipeline, err := m.js.GetPipelineById(dbRead, pipelineId)
		if err != nil {
			log.Printf("failed to get pipeline by id: %s", err)
			http.Error(w, "failed to get pipeline", http.StatusInternalServerError)
			return
		}
		if pipeline.RepoId != r.Repo.Id {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusUnauthorized)
			return
		}
		handler(w, UserRepoPipelineMuxRequest{
			Request:                 r.Request,
			UserWithWritePermission: r.UserWithWritePermission,
			Repo:                    r.Repo,
			RepoOwnerUsr:            r.RepoOwnerUsr,
			Pipeline:                pipeline,
			Flags:                   r.Flags,
		}, dbRead)
	})
}
func (m userRepoPipelineMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r UserRepoPipelineMuxRequest, dbWrite context.Context) (shouldCommit bool)) {
	m.u.HandleFuncW(pattern, func(w http.ResponseWriter,
		r UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
		pipelineId := r.Request.PathValue(routes.PipelineIdPathParamName)
		pipeline, err := m.js.GetPipelineById(dbWrite, pipelineId)
		if err != nil {
			log.Printf("failed to get pipeline by id: %s", err)
			http.Error(w, "failed to get pipeline", http.StatusInternalServerError)
			return
		}
		if pipeline.RepoId != r.Repo.Id {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusUnauthorized)
			return
		}
		return handler(w, UserRepoPipelineMuxRequest{
			Request:                 r.Request,
			UserWithWritePermission: r.UserWithWritePermission,
			Repo:                    r.Repo,
			RepoOwnerUsr:            r.RepoOwnerUsr,
			Pipeline:                pipeline,
			Flags:                   r.Flags,
		}, dbWrite)
	})
}
