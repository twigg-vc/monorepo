package twigg

import (
	"context"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/twiggtoken"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/client"
	"net/http"
)

// Registers the handlers required for ag clients to pull/push.
// All handlers will be wrapped with the provided `wrap` if its not nil.
func AddHandlers(
	db Db,
	uSrv userservice.Service,
	rSrv repo.Service,
	ciq CiCdQueue,
	mux wrappers.RlMux,
	configName string,
	signer twiggtoken.TokenSigner,
	wrap func(http.HandlerFunc) http.HandlerFunc) {
	if wrap == nil {
		wrap = func(r http.HandlerFunc) http.HandlerFunc {
			return r
		}
	}
	h := handler{db, uSrv, rSrv, ciq, configName, signer}
	mux.HandleFunc(client.PullMethod+" "+
		routes.RepoPullPattern, wrap(h.handlePull))
	mux.HandleFunc(client.PushMethod+" "+
		routes.RepoPushPattern, wrap(h.handlePush))
}

type Db interface {
	BeginRead() (readCtx context.Context, closeTx func(), err error)
	BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error)
	HasPermission(ctx context.Context, userId int64, p permissions.Permission, assetId string) (bool, error)
}

type CiCdQueue interface {
	EnqueueCiCdRun(repoId, commitId, commitVersion uint64,
		trigger runnerlib.JobTrigger, w context.Context) (int64, error)
}
