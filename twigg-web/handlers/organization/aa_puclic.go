package organization

import (
	"context"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
	"time"
)

// NoSeatsLeftErrMsg is the error message used to indicate the organization has no seats left.
const NoSeatsLeftErrMsg = "organization has no available seats"

// PermissionAlreadyExitsErrMsg is the error message used to indicate the user already has the permission
const PermissionAlreadyExitsErrMsg = "permission already exists"

type handler struct {
	userS        UserService
	permSrv      PermissionsService
	stripeClient StripeClient
	trackQueue   TrackQueue
	orgHelper    OrgHelper
	repoService  RepoService
}

func AddHandlers(
	userWithSubMux wrappers.UserWithSubMux,
	orgOwnerMux wrappers.OrgOwnerMux,
	orgOwnerOrMemberMux wrappers.OrgOwnerOrMemberMux,
	userS UserService,
	permSrv PermissionsService,
	stripeClient StripeClient,
	trackQueue TrackQueue,
	orgHelper OrgHelper,
	repoService RepoService,
) {
	h := handler{userS, permSrv, stripeClient, trackQueue, orgHelper, repoService}

	userWithSubMux.HandleFuncR("GET "+routes.OrganizationsPattern, h.handleGetUserOrganizations)
	orgOwnerOrMemberMux.HandleFuncR("GET "+routes.OrganizationPattern, h.handleGetOrganizationPage)
	userWithSubMux.HandleFuncR("GET "+routes.NewOrganizationPattern, h.handleGetCreateOrganization)
	userWithSubMux.HandleFuncW("POST "+routes.NewOrganizationPattern, h.handlePostCreateOrganization)
	orgOwnerMux.HandleFuncW("POST "+routes.GrantOwnerOrMemberPermToUserPattern, h.handlePostGrantOwnerOrMemberPermToUser)
	orgOwnerMux.HandleFuncW("POST "+routes.RevokeOwnerOrMemberPermToUserPattern, h.handlePostRevokeOwnerOrMemberPermToUser)
	orgOwnerMux.HandleFuncR("GET "+routes.ManageOrgSubscriptionWithStripe, h.handleManageOrgSubscription)
}

type PermissionsService interface {
	GrantPermissionIfNotExists(wl context.Context, userId int64, p permissions.Permission, assetId string) (alreadyExists bool, err error)
	HasPermission(rl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error)
	GetUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (iterator.I[int64], error)
	RevokePermissionIfExists(wl context.Context, userId int64, p permissions.Permission, assetId string) error
	GetUserAssetIdsWithPermission(rl context.Context, userId int64, p ...permissions.Permission) (iterator.I[string], error)
	CountUserAssetsWithPermission(rl context.Context, userId int64, p permissions.Permission) (int64, error)
	CountUsersWithPermission(rl context.Context, assetId string, p permissions.Permission) (int64, error)
}

type UserService interface {
	CreateNewOrganizationUser(w context.Context, organizationUsername string) (user.User, error)
	GetByUsername(r context.Context, username string) (u user.User, isNotFoundErr bool, err error)
	Get(r context.Context, id int64) (u user.User, isNotFoundErr bool, err error)
}

type StripeClient interface {
	GetNewCustomerPortalSession(userID int64, stripeCustomerID string) (portalSessionUrl string, err error)
}

type TrackQueue interface {
	GetLimits(ownerId int64, tx context.Context) (maxJobs int, maxTimeout time.Duration, err error)
}

type RepoService interface {
	GetAllByOwnerId(rl context.Context, ownerId int64) (iterator.I[repo.Repo], error)
}

type OrgHelper interface {
	GrantUserPermissionToOrgIfNotExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (alreadyExists bool, err error)
	RevokeUserPermissionFromOrgIfExist(wl context.Context, userId int64, orgAssetId string, p permissions.Permission) (noOwnersLeftErr bool, err error)
	OrgCanAddOwnerOrMember(org user.User, numberOfOwners int64, numberOfMembers int64) (bool, error)
}