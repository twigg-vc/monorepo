package wrappers

import (
	"context"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/metrics"
	perm "monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	jobsservice "monorepo/twigg-web/services/jobs"
	reposervice "monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/session"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/twiggtoken"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"net/http"
)

// ########## Creates a mux with a rate limit
func NewRateLimitted(maxQps float64, maxQpsBurst int,
	mService metrics.Service,
	sessionService session.Service, mux Mux) RlMux {
	return newRateLimitted(maxQps, maxQpsBurst, mService, sessionService, mux)
}

type Mux interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request))
}

type RlMux interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request))
}

// ##########

// ########## Creates a mux for requests that expect authenticated users
func NewAuthMux(sessionService session.Service, configName string, mux RlMux) AuthMux {
	return authMux{sessionService, configName, mux}
}

type AuthMux interface {
	HandleFunc(pattern string, handler func(w http.ResponseWriter, r AuthMuxRequest))
}
type AuthMuxRequest struct {
	*http.Request
	Username string
	UserId   int64
	Flags    featureflags.Flags
}

type CliKeyAuthMuxDb interface {
	UserRepoMuxDb
	BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error)
}

// ########## Creates a mux for requests from the CLI, authenticated with the
// user's api key. It validates that the user has a subscription and is the
// repo owner or has write permission in the repo (same checks as the
// session-based repo muxes).
func NewCliKeyAuthMux(configName string, db CliKeyAuthMuxDb,
	repoSrv reposervice.Service, userSrv userservice.Service,
	mux RlMux) CliKeyAuthMux {
	return cliKeyAuthMux{configName, db, repoSrv, userSrv, mux}
}

type CliKeyAuthMux interface {
	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r CliKeyAuthMuxRequest, dbWrite context.Context) (shouldCommit bool))
}

// CliKeyAuthMuxRequest is intentionally a different type from the session
// based requests (e.g. AuthMuxRequest, UserRepoMuxRequest): a handler that
// takes it explicitly serves api-key authenticated CLI requests, and won't
// compile if it's wired to a different mux.
type CliKeyAuthMuxRequest struct {
	*http.Request
	UserWithWritePermission user.User
	Repo                    repo.Repo
	RepoOwnerUsr            user.User
	Flags                   featureflags.Flags
}

// ##########

type UserMuxDb interface {
	BeginRead() (readCtx context.Context, closeTx func(), err error)
	BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error)
	HasPermission(ctx context.Context, userId int64, p perm.Permission, assetId string) (bool, error)
}

// ########## Creates a mux for request that expect a user
func NewUserMux(authMux AuthMux,
	stripeClient stripeclient.StripeClient,
	db UserMuxDb,
	userService userservice.Service,
) UserMux {
	return userMux{authMux, stripeClient, db, userService}
}

type UserMux interface {
	// HandleFuncR registers a handler for routes that require a read-only
	// operations. After the handler returns, dbRead is automatically closed.
	HandleFuncR(pattern string,
		handler func(w http.ResponseWriter, r UserMuxRequest, dbRead context.Context))

	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string,
		handler func(w http.ResponseWriter, r UserMuxRequest,
			dbWrite context.Context) (shouldCommit bool))
}
type UserMuxRequest struct {
	*http.Request
	User                  user.User
	HaveOrgParamInRequest bool
	Org                   user.User
	UserPermissionInOrg   perm.Permission
	Flags                 featureflags.Flags
}

// ##########

// ########## Creates a mux for admin users
func NewAdminUserMux(userMux UserMux,
	adminUserEmails []string) AdminUserMux {
	return adminUserMux{userMux, adminUserEmails}
}

type AdminUserMux interface {
	HandleFuncR(pattern string, handler func(w http.ResponseWriter,
		r AdminUserMuxRequest, dbRead context.Context))
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r AdminUserMuxRequest,
		dbWrite context.Context) (shouldCommit bool))
}
type AdminUserMuxRequest struct {
	*http.Request
	AdminUser user.User
	Flags     featureflags.Flags
}

// ########## Creates a mux for users with self paid or guest sub
func NewUserWithSubMux(userMux UserMux, repoService reposervice.Service) UserWithSubMux {
	return userWithSubMux{userMux, repoService}
}

type UserWithSubMux interface {
	// HandleFuncR registers a handler for routes that require a read-only
	// operations. After the handler returns, dbRead is automatically closed.
	HandleFuncR(pattern string, handler func(w http.ResponseWriter,
		r UserWithSubMuxRequest, dbRead context.Context))

	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r UserWithSubMuxRequest,
		dbWrite context.Context) (shouldCommit bool))
}
type UserWithSubMuxRequest struct {
	*http.Request
	UserWithSub           user.User
	HaveOrgParamInRequest bool
	OrgWithSub            user.User
	UserPermissionInOrg   perm.Permission
	Flags                 featureflags.Flags
}

// ##########

type UserRepoMuxDb interface {
	HasPermission(ctx context.Context, userId int64, p perm.Permission, assetId string) (bool, error)
}

// ########## Creates a mux for users that are have permission in repo
func NewUserRepoMux(configName string, userWithSubMux UserWithSubMux,
	permSrv UserRepoMuxDb,
	repoSrv reposervice.Service,
	userSrv userservice.Service) UserRepoMux {
	return userRepoMux{configName, userWithSubMux, permSrv, repoSrv, userSrv}
}

type UserRepoMux interface {
	// HandleFuncR registers a handler for routes that require a read-only
	// operations. After the handler returns, dbRead is automatically closed.
	HandleFuncR(pattern string, handler func(w http.ResponseWriter,
		r UserRepoMuxRequest, dbRead context.Context))

	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool))
}
type UserRepoMuxRequest struct {
	*http.Request
	UserWithWritePermission user.User
	Repo                    repo.Repo
	RepoOwnerUsr            user.User
	Flags                   featureflags.Flags
}

// ##########

type UserWithReadPermissionMuxDb interface {
	UserRepoMuxDb
	BeginRead() (readCtx context.Context, closeTx func(), err error)
}

// ########## Creates a mux for users that can read a repo. Public repos are
// readable by anyone, including anonymous (logged out) users; private repos
// require a logged in user that is the owner or has read or write permission
// in the repo.
func NewUserWithReadPermissionMux(configName string, mux RlMux,
	sessionService session.Service,
	db UserWithReadPermissionMuxDb,
	repoSrv reposervice.Service,
	userSrv userservice.Service) UserWithReadPermissionMux {
	return UserWithReadPermissionMux{
		userWithReadPermissionMux{configName, mux, sessionService, db, repoSrv, userSrv}}
}

type UserWithReadPermissionMux struct {
	m userWithReadPermissionMux
}

// HandleFuncR registers a handler for routes that require a read-only
// operations. After the handler returns, dbRead is automatically closed.
func (m UserWithReadPermissionMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r UserWithReadPermissionMuxRequest, dbRead context.Context)) {
	m.m.HandleFuncR(pattern, handler)
}

type UserWithReadPermissionMuxRequest struct {
	*http.Request
	// MaybeUserWithReadPermission is the zero-value user when IsLoggedIn is false.
	MaybeUserWithReadPermission *user.User
	IsLoggedIn                  bool
	Repo                        repo.Repo
	RepoOwnerUsr                user.User
	Flags                       featureflags.Flags
}

// ##########

// ########## Creates a mux for users that are have permission in repo and in a jpb pipeline
func NewUserRepoPipelineMux(u UserRepoMux, js jobsservice.Service) UserRepoPipelineMux {
	return UserRepoPipelineMux{userRepoPipelineMux{u, js}}
}

type UserRepoPipelineMux struct {
	m userRepoPipelineMux
}

func (m UserRepoPipelineMux) HandleFuncR(pattern string, handler func(w http.ResponseWriter,
	r UserRepoPipelineMuxRequest, dbRead context.Context)) {
	m.m.HandleFuncR(pattern, handler)
}
func (m UserRepoPipelineMux) HandleFuncW(pattern string, handler func(w http.ResponseWriter,
	r UserRepoPipelineMuxRequest, dbWrite context.Context) (shouldCommit bool)) {
	m.m.HandleFuncW(pattern, handler)
}

type UserRepoPipelineMuxRequest struct {
	*http.Request
	UserWithWritePermission user.User
	Repo                    repo.Repo
	RepoOwnerUsr            user.User
	Pipeline                job.Pipeline
	Flags                   featureflags.Flags
}

// ##########

// ########## Creates a mux for requests from track server only
func NewServerKeyAuthTrackMux(twiggServerKey string, mux RlMux) ServerKeyAuthTrackMux {
	return ServerKeyAuthTrackMux{serverKeyAuthTrackMux{twiggServerKey, mux}}
}

type ServerKeyAuthTrackMux struct {
	serverKeyAuthTrackMux
}

// Register a handler for authenticated requests from the track server
func (tm ServerKeyAuthTrackMux) HandleFunc(
	pattern string, handler func(w http.ResponseWriter, r ServerKeyAuthTrackMuxRequest)) {
	tm.serverKeyAuthTrackMux.HandleFunc(pattern, handler)
}

type ServerKeyAuthTrackMuxRequest struct {
	*http.Request
}

// ########## Creates a mux for requests from track server (TwiggServerKey + token)
func NewServerKeyAndTokenAuthTrackMux(twiggServerKey string, signer twiggtoken.TokenSigner, mux RlMux) ServerKeyAndTokenAuthTrackMux {
	return ServerKeyAndTokenAuthTrackMux{serverKeyAndTokenAuthTrackMux{twiggServerKey, signer, mux}}
}

type ServerKeyAndTokenAuthTrackMux struct {
	serverKeyAndTokenAuthTrackMux
}

// Register a handler for requests from the track server authenticated by TwiggServerKey and twigg token
func (tm ServerKeyAndTokenAuthTrackMux) HandleFunc(
	pattern string,
	handler func(w http.ResponseWriter, r ServerKeyAndTokenAuthTrackMuxRequest)) {
	tm.serverKeyAndTokenAuthTrackMux.HandleFunc(pattern, handler)
}

type ServerKeyAndTokenAuthTrackMuxRequest struct {
	*http.Request
	TwiggToken twiggtoken.ParsedToken
}

// ##########

// ########## Creates a mux for users that have owner permission in organization
func NewOrgOwnerMux(configName string, userWithSubMux UserWithSubMux,
	userSrv userservice.Service) OrgOwnerMux {
	return orgOwnerMux{configName, userWithSubMux, userSrv}
}

type OrgOwnerMux interface {
	// HandleFuncR registers a handler for routes that require a read-only
	// operations. After the handler returns, dbRead is automatically closed.
	HandleFuncR(pattern string, handler func(w http.ResponseWriter,
		r OrgOwnerMuxRequest, dbRead context.Context))

	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r OrgOwnerMuxRequest, dbWrite context.Context) (shouldCommit bool))
}
type OrgOwnerMuxRequest struct {
	*http.Request
	UserWithOwnerPermission user.User
	Org                     user.User
	Flags                   featureflags.Flags
}

// ##########

// ########## Creates a mux for users that have member permission in organization
func NewOrgMemberMux(configName string, userWithSubMux UserWithSubMux,
	userSrv userservice.Service) OrgMemberMux {
	return orgMemberMux{configName, userWithSubMux, userSrv}
}

type OrgMemberMux interface {
	// HandleFuncR registers a handler for routes that require a read-only
	// operations. After the handler returns, dbRead is automatically closed.
	HandleFuncR(pattern string, handler func(w http.ResponseWriter,
		r OrgMemberMuxRequest, dbRead context.Context))

	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r OrgMemberMuxRequest, dbWrite context.Context) (shouldCommit bool))
}
type OrgMemberMuxRequest struct {
	*http.Request
	UserWithMemberPermission user.User
	Org                      user.User
	Flags                    featureflags.Flags
}

// ##########

// ########## Creates a mux for users that have owner or member permission in organization
func NewOrgOwnerOrMemberMux(configName string, userWithSubMux UserWithSubMux,
	userSrv userservice.Service) OrgOwnerOrMemberMux {
	return orgOwnerOrMemberMux{configName, userWithSubMux, userSrv}
}

type OrgOwnerOrMemberMux interface {
	// HandleFuncR registers a handler for routes that require a read-only
	// operations. After the handler returns, dbRead is automatically closed.
	HandleFuncR(pattern string, handler func(w http.ResponseWriter,
		r OrgOwnerOrMemberMuxRequest, dbRead context.Context))

	// HandleFuncW registers a handler for routes that perform write
	// operations. dbWrite is automatically closed and shouldCommit bool
	// indicates whether the write transaction should be committed, after
	// handler returns.
	HandleFuncW(pattern string, handler func(w http.ResponseWriter,
		r OrgOwnerOrMemberMuxRequest, dbWrite context.Context) (shouldCommit bool))
}
type OrgOwnerOrMemberMuxRequest struct {
	*http.Request
	UserWithOwnerOrMemberPermission user.User
	Org                             user.User
	Flags                           featureflags.Flags
}

// ##########
