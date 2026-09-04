package wrappers

import (
	"context"
	"log"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/stripeclient"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/webdb"
	"net/http"
)

type userMux struct {
	authMux      AuthMux
	stripeClient stripeclient.StripeClient
	db           webdb.WebDb
	userService  userservice.Service
}

func (m userMux) HandleFuncR(pattern string,
	handler func(w http.ResponseWriter, r UserMuxRequest, dbRead context.Context)) {
	m.authMux.HandleFunc(pattern, func(w http.ResponseWriter, r AuthMuxRequest) {
		// In some rare occasions (such as an unhandled stripe plan update),
		// we must update the user before handling a request.
		// We first do that whole pre-work completely and then proceed as usual.
		err := m.updateUserIfNeeded(r.UserId)
		if err != nil {
			log.Printf("failed to update user with name %s and id %d: %q",
				r.Username, r.UserId, err)
			http.Error(w, "err updating user", http.StatusInternalServerError)
			return
		}

		haveOrgParamInRequest, orgId, ok := m.parseAndValidateOrgParamsInRequestIfExist(w, r)
		if !ok {
			return
		}

		dbRead, closeDbRead, err := m.db.BeginRead()
		defer closeDbRead()
		if err != nil {
			log.Printf("failed to get tx in user mux: %q", err)
			http.Error(w, "err getting tx", http.StatusInternalServerError)
			return
		}
		usr, isUserNotFoundErr, err := m.userService.Get(dbRead, r.UserId)
		if isUserNotFoundErr {
			http.Redirect(w, r.Request, routes.LoginPage, http.StatusSeeOther)
			return
		}
		if err != nil {
			log.Printf("failed to get user by id %d: %q", r.UserId, err)
			http.Error(w, "err getting user", http.StatusInternalServerError)
			return
		}
		if usr.IsOrganization {
			log.Printf("invalid organization user (id:%d)", r.UserId)
			http.Error(w, "invalid organization user", http.StatusBadRequest)
			return
		}

		// Get organization if needed
		org, userPermissionInOrg, ok := m.getOrgAndUserPermissionInOrgIfNeeded(
			w,
			r,
			dbRead,
			haveOrgParamInRequest,
			orgId,
		)
		if !ok {
			return
		}

		handler(w, UserMuxRequest{
			Request:               r.Request,
			User:                  usr,
			HaveOrgParamInRequest: haveOrgParamInRequest,
			Org:                   org,
			UserPermissionInOrg:   userPermissionInOrg,
			Flags:                 r.Flags,
		}, dbRead)
	})
}
func (m userMux) HandleFuncW(pattern string,
	handler func(w http.ResponseWriter, r UserMuxRequest,
		dbWrite context.Context) (shouldCommit bool)) {

	m.authMux.HandleFunc(pattern, func(w http.ResponseWriter, r AuthMuxRequest) {
		// In some rare occasions (such as an unhandled stripe plan update),
		// we must update the user before handling a request.
		// We first do that whole pre-work completely and then proceed as usual.
		err := m.updateUserIfNeeded(r.UserId)
		if err != nil {
			log.Printf("failed to update user with name %s and id %d: %q",
				r.Username, r.UserId, err)
			http.Error(w, "err updating user",
				http.StatusInternalServerError)
			return
		}

		haveOrgParamInRequest, orgId, ok := m.parseAndValidateOrgParamsInRequestIfExist(w, r)
		if !ok {
			return
		}

		dbWrite, closeDbWrite, commit, err := m.db.BeginWrite()
		defer closeDbWrite()
		if err != nil {
			log.Printf("failed to get tx in user mux: %q", err)
			http.Error(w, "err getting write tx",
				http.StatusInternalServerError)
			return
		}
		usr, isUserNotFoundErr, err := m.userService.Get(dbWrite, r.UserId)
		if isUserNotFoundErr {
			http.Redirect(w, r.Request, routes.LoginPage, http.StatusSeeOther)
			return
		}
		if err != nil {
			log.Printf("failed to get user by id %d: %q", r.UserId, err)
			http.Error(w, "err getting user",
				http.StatusInternalServerError)
			return
		}

		// Get organization if needed
		org, userPermissionInOrg, ok := m.getOrgAndUserPermissionInOrgIfNeeded(
			w,
			r,
			dbWrite,
			haveOrgParamInRequest,
			orgId,
		)
		if !ok {
			return
		}

		shouldCommit := handler(w,
			UserMuxRequest{
				Request:               r.Request,
				User:                  usr,
				HaveOrgParamInRequest: haveOrgParamInRequest,
				Org:                   org,
				UserPermissionInOrg:   userPermissionInOrg,
				Flags:                 r.Flags,
			},
			dbWrite)
		if shouldCommit {
			err = commit()
			if err != nil {
				log.Printf("err commiting tx: %q", err)
				http.Error(w, "err committing write tx",
					http.StatusInternalServerError)
				return
			}
		}
	})
}
func (m userMux) updateUserIfNeeded(userId int64) (err error) {
	// Check if the user needs an update with a read lock.
	// If an update is in fact needed we'll release this lock to then grab a
	// write lock, but getting a write lock right away would be terrible because
	// write locks are too restrictive and in the vast majority of the time we
	// won't need to update the user
	dbRead, closeDbRead, err := m.db.BeginRead()
	defer closeDbRead()
	if err != nil {
		return
	}
	usr, _, err := m.userService.Get(dbRead, userId)
	if err != nil {
		return
	}

	// Check if user must be updated because their stripe session changed
	mustUpdateUser := false
	var subscriptionId string
	var priceId stripeclient.PriceId
	var quantity int64
	if usr.State == user.UserState_PayingWithStripe {
		var st stripeclient.SessionPaymentStatus
		st, subscriptionId, priceId,
			quantity, err = m.stripeClient.GetSessionStatus(usr.StripeSessionId)
		if err != nil {
			return
		}
		if st == stripeclient.SessionPaymentStatus_PaidOrNoPaymentRequired {
			mustUpdateUser = true
		}
	}
	if !mustUpdateUser {
		return
	}

	// If the user must be updated, we must get a write lock.
	closeDbRead()
	// The user must be read again to avoid race conditions
	dbWrite, closeDbWrite, commit, err := m.db.BeginWrite()
	defer closeDbWrite()
	if err != nil {
		return
	}
	usr, _, err = m.userService.Get(dbWrite, userId)
	if err != nil {
		return
	}

	// Check if user still needs update
	mustUpdateUser = false
	if usr.State == user.UserState_PayingWithStripe {
		var st stripeclient.SessionPaymentStatus
		st, subscriptionId, priceId,
			quantity, err = m.stripeClient.GetSessionStatus(usr.StripeSessionId)
		if err != nil {
			return
		}
		if st == stripeclient.SessionPaymentStatus_PaidOrNoPaymentRequired {
			mustUpdateUser = true
		}
	}
	if !mustUpdateUser {
		return
	}

	_, err = m.userService.HandleStripeCheckoutSessionSuccess(
		dbWrite, userId, subscriptionId, usr.StripeSessionId, priceId, quantity)
	if err != nil {
		return
	}
	err = commit()
	return
}

// Parses the organization name parameter,
// validates that the organization exists, and ensures the
// organization user state is synchronized if necessary.
//
// If the request does not contain an organization route parameter,
// haveOrgParamInRequest will be false and ok will be true.
//
// If ok is false, this function already wrote the HTTP response and the caller
// must return immediately without writing anything else.
func (m userMux) parseAndValidateOrgParamsInRequestIfExist(
	w http.ResponseWriter,
	r AuthMuxRequest,
) (haveOrgParamInRequest bool, orgId int64, ok bool) {

	orgUsername := r.PathValue(routes.OrganizationNameParamName)
	if orgUsername == "" {
		orgUsername = r.FormValue(routes.OrganizationNameParamName)
	}
	if orgUsername == "" {
		haveOrgParamInRequest = false
		ok = true
		return
	}
	haveOrgParamInRequest = true

	dbRead, closeDbRead, err := m.db.BeginRead()
	defer closeDbRead()
	if err != nil {
		ok = false
		log.Printf("failed read tx parsing org err:%q", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	org, isNotFoundErr, err := m.userService.GetByUsername(dbRead, orgUsername)
	if isNotFoundErr {
		ok = false
		http.Error(w, "org no found", http.StatusBadRequest)
		return
	}
	if err != nil {
		ok = false
		log.Printf("error getting org, orgUsername:%q err:%q", orgUsername, err)
		http.Error(w, "error getting org", http.StatusInternalServerError)
		return
	}
	if !org.IsOrganization {
		ok = false
		log.Printf("resolved organization username to not org username:%q", orgUsername)
		http.Error(w, "invalid org", http.StatusBadRequest)
		return
	}

	// CLose read to make it easier to
	closeDbRead()

	err = m.updateUserIfNeeded(org.Id)
	if err != nil {
		ok = false
		log.Printf("failed to update user org with name %s and id %d: %q", r.Username, r.UserId, err)
		http.Error(w, "err updating user", http.StatusInternalServerError)
		return
	}

	orgId = org.Id
	ok = true
	return
}

// If ok is false, this function already wrote the HTTP response and the caller
// must return immediately without writing anything else.
func (m userMux) getOrgAndUserPermissionInOrgIfNeeded(
	w http.ResponseWriter,
	r AuthMuxRequest,
	dbRead context.Context,
	haveOrgParamInRequest bool,
	orgId int64,
) (org userservice.User, userPermissionInOrg permissions.Permission, ok bool) {
	if !haveOrgParamInRequest {
		ok = true
		return
	}
	org, _, err := m.userService.Get(dbRead, orgId)
	if err != nil {
		log.Printf("failed to get org user by id %d: %q", r.UserId, err)
		http.Error(w, "err getting org", http.StatusInternalServerError)
		return
	}

	// UserPermissionInOrg will be the first permission that the user
	// has in this list
	permsOrderedByPriority := []permissions.Permission{
		permissions.Permission_OrganizationOwner,
		permissions.Permission_OrganizationMember,
	}
	var foundPerm bool
	for _, perm := range permsOrderedByPriority {
		hasPerm, err := m.db.HasPermission(dbRead, r.UserId, perm, permissions.OrganizationAssetId(orgId))
		if err != nil {
			log.Printf("error checking if user:%d has owner perm in org:%d", r.UserId, orgId)
			http.Error(w, "Failed to check permission", http.StatusInternalServerError)
			return
		}
		if hasPerm {
			foundPerm = true
			userPermissionInOrg = perm
			break
		}
	}

	if !foundPerm {
		http.Error(w, "User does not have owner or member permission", http.StatusForbidden)
		return
	}

	ok = true
	return
}
