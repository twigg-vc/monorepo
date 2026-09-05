package admindash

import (
	"context"
	"monorepo/squeue"
	"monorepo/twigg-web/metrics"
	"monorepo/twigg-web/routes"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/pprof"
)

func AddHandlers(
	m metrics.Service,
	userService userservice.Service,
	queueStorage squeue.SqliteStorage,
	queueRunner squeue.Runner,
	adminMux wrappers.AdminUserMux) {
	h := handler{
		m,
		userService,
		queueStorage,
		queueRunner,
	}
	adminMux.HandleFuncR("GET "+routes.AdminDash, h.handleGetDash)
	adminMux.HandleFuncR("GET "+routes.AdminDashRequestCounts, h.handleGetRequestCounts)
	adminMux.HandleFuncR("GET "+routes.AdminDashLogs, h.handleGetLogs)
	adminMux.HandleFuncR("GET "+routes.AdminDashMetricTimeSeries, h.handleGetMetricTs)
	adminMux.HandleFuncW("POST "+routes.AdminDashRequeueDeadLetter, h.handleRequeueDeadLetter)

	adminMux.HandleFuncR("GET "+routes.AdminDash+"/pprof", adaptHandlerFunc(pprof.Index))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/cmdline", adaptHandlerFunc(pprof.Cmdline))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/profile", adaptHandlerFunc(pprof.Profile))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/symbol", adaptHandlerFunc(pprof.Symbol))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/trace", adaptHandlerFunc(pprof.Trace))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/goroutine", adaptHandler(pprof.Handler("goroutine")))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/heap", adaptHandler(pprof.Handler("heap")))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/threadcreate", adaptHandler(pprof.Handler("threadcreate")))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/block", adaptHandler(pprof.Handler("block")))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/allocs", adaptHandler(pprof.Handler("allocs")))
	adminMux.HandleFuncR("GET "+routes.AdminDash+"/mutex", adaptHandler(pprof.Handler("mutex")))
}

func adaptHandlerFunc(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbRead context.Context) {
	return func(w http.ResponseWriter, r wrappers.AdminUserMuxRequest, dbRead context.Context) {
		f(w, r.Request)
	}
}

func adaptHandler(h http.Handler) func(w http.ResponseWriter,
	r wrappers.AdminUserMuxRequest, dbRead context.Context) {
	return func(w http.ResponseWriter, r wrappers.AdminUserMuxRequest, dbRead context.Context) {
		h.ServeHTTP(w, r.Request)
	}
}