package organization

import (
	"context"
	"log"
	"monorepo/base/iterator"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strconv"
)

// Abuse check, not a hard limit, change if needed
const orgCreationOwnerLimit = 50

func (h handler) handleGetCreateOrganization(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbRead context.Context) {
	if !r.Flags.OrganizationFeatureIsEnabled {
		log.Printf("user.id=%d, got enabled error trying to create organization", r.UserWithSub.Id)
		http.Error(w, "feature is not enabled", http.StatusServiceUnavailable)
		return
	}
	err := webcomponents.Page(
		false, // hideNavBar
		r.Flags,
		webcomponents.NewOrganization(),
	).Render(w)
	if err != nil {
		log.Printf("failed render NewOrganization: %s", err)
		http.Error(w, "failed render response", http.StatusInternalServerError)
		return
	}
}

func (h handler) handlePostCreateOrganization(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.OrganizationFeatureIsEnabled {
		log.Printf("user.id=%d, got enabled error trying to create organization", r.UserWithSub.Id)
		http.Error(w, "feature is not enabled", http.StatusServiceUnavailable)
		return
	}

	newOrgName := r.FormValue(routes.NewOrganizationNameParamName)

	if !userservice.UsernameIsValid(newOrgName) {
		http.Error(w, "invalid new org name", http.StatusBadRequest)
		return
	}

	// Abuse check
	orgsUserIsOwner, err := h.permSrv.CountUserAssetsWithPermission(dbWrite, r.UserWithSub.Id, permissions.Permission_OrganizationOwner)
	if err != nil {
		log.Printf("internal errors checking number of orgs User.id=%d id owner", r.UserWithSub.Id)
		http.Error(w, "internal error checking user", http.StatusInternalServerError)
		return
	}
	if orgsUserIsOwner >= orgCreationOwnerLimit {
		log.Printf("user.id=%d tried to create organization but exceeded owner limit", r.UserWithSub.Id)
		http.Error(w, "user already owns too many organizations", http.StatusBadRequest)
		return
	}

	org, err := h.userS.CreateNewOrganizationUser(dbWrite, newOrgName)
	if err != nil {
		log.Printf("internal errors creating org for User.id=%d", r.UserWithSub.Id)
		http.Error(w, "internal error creating org", http.StatusInternalServerError)
		return
	}

	alreadyExists, err := h.permSrv.GrantPermissionIfNotExists(dbWrite, r.UserWithSub.Id, permissions.Permission_OrganizationOwner, permissions.OrganizationAssetId(org.Id))
	if err != nil {
		log.Printf("internal errors creating org for User.id=%d", r.UserWithSub.Id)
		http.Error(w, "internal error creating org", http.StatusInternalServerError)
		return
	}
	if alreadyExists {
		panic("can not already have permission for a just created organization")
	}
	return true
}

func (h handler) handlePostGrantOwnerOrMemberPermToUser(
	w http.ResponseWriter,
	r wrappers.OrgOwnerMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.OrganizationFeatureIsEnabled {
		log.Printf("user.id=%d, got enabled error trying to grant perm to organization=%q", r.UserWithOwnerPermission.Id, r.Org.Username)
		http.Error(w, "feature is not enabled", http.StatusServiceUnavailable)
		return
	}

	userToGrantPermissionUsername := r.FormValue(routes.UsernameParameterName)
	if userToGrantPermissionUsername == "" {
		http.Error(w, "empty username param", http.StatusBadRequest)
		return
	}

	userToGrantPermission, isNotFoundErr, err := h.userS.GetByUsername(dbWrite, userToGrantPermissionUsername)
	if isNotFoundErr {
		http.Error(w, "User to grant permission not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Internal err getting usr", http.StatusInternalServerError)
		return
	}
	if userToGrantPermission.IsOrganization {
		http.Error(w, "Can not grant permission to a organization user", http.StatusBadRequest)
		return
	}

	permissionInt, err := strconv.ParseInt(r.FormValue(routes.PermissionParamName), 10, 64)
	if err != nil {
		http.Error(w, "invalid permission param form value", http.StatusBadRequest)
		return
	}

	var permission permissions.Permission
	var hasConflictingPermission bool

	switch permissions.Permission(permissionInt) {

	case permissions.Permission_OrganizationOwner:
		permission = permissions.Permission_OrganizationOwner
		hasConflictingPermission, err = h.permSrv.HasPermission(dbWrite, userToGrantPermission.Id, permissions.Permission_OrganizationMember, permissions.OrganizationAssetId(r.Org.Id))
		if err != nil {
			http.Error(w, "Failed to check existing permissions", http.StatusInternalServerError)
			return
		}

	case permissions.Permission_OrganizationMember:
		permission = permissions.Permission_OrganizationMember
		hasConflictingPermission, err = h.permSrv.HasPermission(dbWrite, userToGrantPermission.Id, permissions.Permission_OrganizationOwner, permissions.OrganizationAssetId(r.Org.Id))
		if err != nil {
			http.Error(w, "Failed to check existing permissions", http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, "invalid permission", http.StatusBadRequest)
		return
	}

	if hasConflictingPermission {
		http.Error(w, "user already has conflicting organization role", http.StatusBadRequest)
		return
	}

	orgAssetId := permissions.OrganizationAssetId(r.Org.Id)
	numberOfOwners, err := h.permSrv.CountUsersWithPermission(dbWrite, orgAssetId, permissions.Permission_OrganizationOwner)
	if err != nil {
		http.Error(w, "internal error checking org capacity", http.StatusInternalServerError)
		return
	}
	numberOfMembers, err := h.permSrv.CountUsersWithPermission(dbWrite, orgAssetId, permissions.Permission_OrganizationMember)
	if err != nil {
		http.Error(w, "internal error checking org capacity", http.StatusInternalServerError)
		return
	}
	canAdd, err := h.orgHelper.OrgCanAddOwnerOrMember(r.Org, numberOfOwners, numberOfMembers)
	if err != nil {
		http.Error(w, "internal error checking org capacity", http.StatusInternalServerError)
		return
	}
	if !canAdd {
		http.Error(w, NoSeatsLeftErrMsg, http.StatusPaymentRequired)
		return
	}

	alreadyExists, err := h.orgHelper.GrantUserPermissionToOrgIfNotExist(
		dbWrite,
		userToGrantPermission.Id,
		permissions.OrganizationAssetId(r.Org.Id),
		permission,
	)
	if err != nil {
		http.Error(w, "Failed to grant permission", http.StatusInternalServerError)
		return
	}
	if alreadyExists {
		w.Write([]byte(PermissionAlreadyExitsErrMsg))
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	shouldCommit = err == nil
	return
}

func (h handler) handlePostRevokeOwnerOrMemberPermToUser(
	w http.ResponseWriter,
	r wrappers.OrgOwnerMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {
	if !r.Flags.OrganizationFeatureIsEnabled {
		log.Printf("user.id=%d, got enabled error trying to revoke perm to organization=%q", r.UserWithOwnerPermission.Id, r.Org.Username)
		http.Error(w, "feature is not enabled", http.StatusServiceUnavailable)
		return
	}

	userToRevokePermissionUsername := r.FormValue(routes.UsernameParameterName)
	if userToRevokePermissionUsername == "" {
		http.Error(w, "empty username param", http.StatusBadRequest)
		return
	}
	userToRevokePermission, isNotFoundErr, err := h.userS.GetByUsername(dbWrite, userToRevokePermissionUsername)
	if isNotFoundErr {
		http.Error(w, "Could not field user", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Internal err getting usr", http.StatusInternalServerError)
		return
	}

	if userToRevokePermission.IsOrganization {
		http.Error(w, "Can not revoke permission from organization user", http.StatusBadRequest)
		return
	}

	permissionInt, err := strconv.ParseInt(r.FormValue(routes.PermissionParamName), 10, 64)
	if err != nil {
		http.Error(w, "invalid permission param form value", http.StatusBadRequest)
		return
	}

	var permission permissions.Permission

	switch permissions.Permission(permissionInt) {

	case permissions.Permission_OrganizationOwner:
		permission = permissions.Permission_OrganizationOwner

	case permissions.Permission_OrganizationMember:
		permission = permissions.Permission_OrganizationMember

	default:
		http.Error(w, "invalid permission to revoke", http.StatusBadRequest)
		return
	}

	// Check permission exist
	hasPerm, err := h.permSrv.HasPermission(dbWrite, userToRevokePermission.Id, permission, permissions.OrganizationAssetId(r.Org.Id))
	if err != nil {
		log.Printf("internal error checking if User.id=%d has permission=%d", userToRevokePermission.Id, permission)
		http.Error(w, "Failed to check permission", http.StatusInternalServerError)
		return
	}
	if !hasPerm {
		http.Error(w, "Does not have permission to revoke", http.StatusBadRequest)
		return
	}

	noOwnersLeftErr, err := h.orgHelper.RevokeUserPermissionFromOrgIfExist(dbWrite, userToRevokePermission.Id, permissions.OrganizationAssetId(r.Org.Id), permission)
	if noOwnersLeftErr {
		http.Error(w, "organization needs a least one owner", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("internal error: %v", err)
		http.Error(w, "Failed to revoke permission", http.StatusInternalServerError)
		return
	}

	const okMsg = "ok"
	_, err = w.Write([]byte(okMsg))
	shouldCommit = err == nil
	return
}

func (h handler) handleManageOrgSubscription(
	w http.ResponseWriter,
	r wrappers.OrgOwnerMuxRequest,
	dbRead context.Context) {

	if r.Org.StripeSubscriptionID == "" {
		http.Error(w, "bad request: no stripe subscription", http.StatusBadRequest)
		return
	}
	portalSessionUrl, err := h.stripeClient.GetNewCustomerPortalSession(r.Org.Id, r.Org.StripeId)
	if err != nil {
		http.Error(w, "something is wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r.Request, portalSessionUrl, http.StatusSeeOther)
}

func (h handler) handleGetUserOrganizations(
	w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest,
	dbRead context.Context,
) {
	orgAssetIdsIt, err := h.permSrv.GetUserAssetIdsWithPermission(
		dbRead,
		r.UserWithSub.Id,
		permissions.Permission_OrganizationOwner,
		permissions.Permission_OrganizationMember,
	)
	if err != nil {
		http.Error(w, "failed to get organizations", http.StatusInternalServerError)
		return
	}

	maxNumberOfOrgsToShow := 20
	orgs := make([]user.User, 0, maxNumberOfOrgsToShow)
	for orgAssetIdsIt.Next() {
		if len(orgs) >= maxNumberOfOrgsToShow {
			break
		}
		orgAssetId, err := orgAssetIdsIt.Get()
		if err != nil {
			http.Error(w, "failed to get organization asset id", http.StatusInternalServerError)
			return
		}

		orgId := permissions.ParseOrganizationAssetIdOrDie(orgAssetId)

		org, isNotFoundErr, err := h.userS.Get(dbRead, orgId)
		if isNotFoundErr {
			panic("organization should always be found if it was saved in permission")
		}
		if err != nil {
			http.Error(w, "failed to get organization", http.StatusInternalServerError)
			return
		}
		if !org.IsOrganization {
			panic("it is not possible for a user to have a Owner or Member permission to another user")
		}
		orgs = append(orgs, org)
	}
	if err = orgAssetIdsIt.Err(); err != nil {
		http.Error(w, "failed iterating organizations", http.StatusInternalServerError)
		return
	}

	err = webcomponents.Page(
		/*hideNavBar*/ false,
		r.Flags,
		webcomponents.OrganizationsPage(orgs),
	).Render(w)
	if err != nil {
		log.Printf("failed render OrganizationsPage: %s", err)
		http.Error(w, "failed render response", http.StatusInternalServerError)
		return
	}
}

func (h handler) handleGetOrganizationPage(
	w http.ResponseWriter,
	r wrappers.OrgOwnerOrMemberMuxRequest,
	dbRead context.Context,
) {
	if !r.Flags.OrganizationFeatureIsEnabled {
		log.Printf("user.id=%d, got enabled error trying to get organization page", r.UserWithOwnerOrMemberPermission.Id)
		http.Error(w, "feature is not enabled", http.StatusServiceUnavailable)
		return
	}

	orgMaxTrackJobs, orgMaxTrackTimeout, err := h.trackQueue.GetLimits(r.Org.Id, dbRead)
	if err != nil {
		log.Printf("failed to get org job limits: %s", err)
		http.Error(w, "failed to get org job limits", http.StatusInternalServerError)
		return
	}

	// Safety limit
	const maxUsersWithPermission = 200

	usersWithOwnerPermission, ok := h.getOwners(w, r, dbRead, maxUsersWithPermission/2)
	if !ok {
		return
	}
	// Members
	usersWithMemberPermission, ok := h.getMembers(w, r, dbRead, maxUsersWithPermission/2)
	if !ok {
		return
	}

	currentUserIsOrgOwner, err := h.permSrv.HasPermission(
		dbRead,
		r.UserWithOwnerOrMemberPermission.Id,
		permissions.Permission_OrganizationOwner,
		permissions.OrganizationAssetId(r.Org.Id),
	)
	if err != nil {
		log.Printf("failed check if currentUserIsOrgOwner: %s", err)
		http.Error(w, "failed get current user permission", http.StatusInternalServerError)
		return
	}

	orgReposIt, err := h.repoService.GetAllByOwnerId(dbRead, r.Org.Id)
	if err != nil {
		log.Printf("failed to get org repos: %s", err)
		http.Error(w, "failed to get org repos", http.StatusInternalServerError)
		return
	}
	const maxOrgRepos = 100
	orgRepos, err := iterator.GetFirstN(maxOrgRepos, orgReposIt)
	if err != nil {
		log.Printf("failed to iterate org repos: %s", err)
		http.Error(w, "failed to iterate org repos", http.StatusInternalServerError)
		return
	}

	err = webcomponents.Page(
		/*hideNavBar*/ false,
		r.Flags,
		webcomponents.OrganizationPage(
			r.Org,
			orgMaxTrackJobs,
			orgMaxTrackTimeout.Milliseconds(),
			usersWithOwnerPermission,
			usersWithMemberPermission,
			currentUserIsOrgOwner,
			orgRepos,
		),
	).Render(w)
	if err != nil {
		log.Printf("failed render OrganizationPage: %s", err)
		http.Error(w, "failed render response", http.StatusInternalServerError)
		return
	}
}

// If !ok already writes error to ResponseWriter and log
func (h handler) getOwners(w http.ResponseWriter, r wrappers.OrgOwnerOrMemberMuxRequest, dbRead context.Context, maxLen int) ([]user.User, bool) {
	usersIdWithOwnerPermissionIt, err := h.permSrv.GetUsersWithPermission(
		dbRead,
		permissions.OrganizationAssetId(r.Org.Id),
		permissions.Permission_OrganizationOwner,
	)
	if err != nil {
		log.Printf("failed to get usersIdWithOwnerPermissionIt: %s", err)
		http.Error(w, "failed to get owners", http.StatusInternalServerError)
		return []user.User{}, false
	}

	mapFunc := func(userId int64) (user.User, error) {
		u, isNotFoundErr, err := h.userS.Get(dbRead, userId)
		if isNotFoundErr {
			panic("should be impossible to have a permission for a user that does not exist")
		}
		return u, err
	}
	usersWithOwnerPermission, err := iterator.GetFirstNWithMapFunc(maxLen, usersIdWithOwnerPermissionIt, mapFunc)
	if err != nil {
		log.Printf("failed to iterate through usersIdWithOwnerPermissionIt: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return []user.User{}, false
	}

	return usersWithOwnerPermission, true
}

// If !ok already writes error to ResponseWriter and log
func (h handler) getMembers(w http.ResponseWriter, r wrappers.OrgOwnerOrMemberMuxRequest, dbRead context.Context, maxLen int) ([]user.User, bool) {
	usersIdWithMemberPermissionIt, err := h.permSrv.GetUsersWithPermission(
		dbRead,
		permissions.OrganizationAssetId(r.Org.Id),
		permissions.Permission_OrganizationMember,
	)
	if err != nil {
		log.Printf("failed to get usersIdWithMemberPermissionIt: %s", err)
		http.Error(w, "failed to get members", http.StatusInternalServerError)
		return []user.User{}, false
	}

	mapFunc := func(userId int64) (user.User, error) {
		u, isNotFoundErr, err := h.userS.Get(dbRead, userId)
		if isNotFoundErr {
			panic("should be impossible to have a permission for a user that does not exist")
		}
		return u, err
	}
	usersWithMemberPermission, err := iterator.GetFirstNWithMapFunc(maxLen, usersIdWithMemberPermissionIt, mapFunc)
	if err != nil {
		log.Printf("failed to iterate through usersIdWithMemberPermissionIt: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return []user.User{}, false
	}
	return usersWithMemberPermission, true
}