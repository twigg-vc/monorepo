package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/session"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"os"
	"strconv"

	"github.com/stripe/stripe-go/v82"
)

type handler struct {
	db           Db
	sessionS     session.Service
	userS        UserService
	stripeClient StripeClient
	orgHelper    OrgHelper
}

// Start a stripe checkout session for user then redirect him to stripe checkout
// Assumes user is authenticated and r.Method == http.MethodPost
func (h handler) cancelTrialIfNeededAndCreateStripeSession(
	w http.ResponseWriter, r wrappers.UserMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	isChoosingPlanForOrg, org, userHasOwnersPermission, ok := h.parseAndValidateOrganizationParam(w, r, dbWrite)
	if !ok {
		return
	}
	if isChoosingPlanForOrg && !userHasOwnersPermission {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}

	userToCreateStripeSession := r.User
	if isChoosingPlanForOrg {
		userToCreateStripeSession = org
	}

	if userToCreateStripeSession.SelfPaidSubscription == user.Subscription_Trial {
		err := h.userS.HandleManualSubscriptionDeleted(dbWrite, userToCreateStripeSession.Id)
		if err != nil {
			log.Printf("failed to delete trial of userId=%d: %s", userToCreateStripeSession.Id, err)
			http.Error(w, "internal err deleting trial", http.StatusInternalServerError)
			return
		}
	}

	// Parse price id
	priceId := r.FormValue(routes.StripePriceIdParamName)
	if priceId == "" {
		log.Printf("%s is invalid price id", priceId)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	product, isOk := h.stripeClient.ResolvePriceId(stripeclient.PriceId(priceId))
	if !isOk {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var stripePriceId stripeclient.PriceId
	switch product {
	case stripeclient.Product_Subscription_Team:
		stripePriceId = h.stripeClient.GetLatestTeamPriceId()
	case stripeclient.Product_Subscription_Solo:
		stripePriceId = h.stripeClient.GetLatestSoloPriceId()
	default:
		log.Printf("%s is invalid stripe price id", stripePriceId)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	quantity, err :=
		strconv.ParseInt(r.FormValue(routes.StripeQuantityParamName), 10, 64)
	if err != nil {
		log.Printf("%s is invalid stripe quantity", r.FormValue(routes.StripeQuantityParamName))
		http.Error(w, "bad request, invalid quantity", http.StatusBadRequest)
		return
	}

	if product == stripeclient.Product_Subscription_Solo && quantity != 1 {
		http.Error(w, "bad request, invalid quantity for solo", http.StatusBadRequest)
		return
	}

	forceCurrency := ""
	if r.URL.Query().Get("usd") != "" {
		forceCurrency = "usd"
	}
	if r.URL.Query().Get("brl") != "" {
		forceCurrency = "brl"
	}
	u, _, err := h.userS.GetUserForPaymentWithStripe(
		dbWrite, userToCreateStripeSession.Id, stripePriceId, quantity, forceCurrency)
	if err != nil {
		log.Printf("failed to get user for payment with stripe %q", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r.Request, u.StripeSessionUrl, http.StatusSeeOther)

	shouldCommit = true
	return
}

func (h handler) handleSubscribeTrial(
	w http.ResponseWriter, r wrappers.UserMuxRequest,
	dbWrite context.Context) (shouldCommit bool) {

	isChoosingPlanForOrg, org, userHasOwnersPermission, ok := h.parseAndValidateOrganizationParam(w, r, dbWrite)
	if !ok {
		return
	}
	if isChoosingPlanForOrg && !userHasOwnersPermission {
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}

	userToSubscribeToTrial := r.User
	redirectPathOnSuccess := routes.Home
	if isChoosingPlanForOrg {
		userToSubscribeToTrial = org
		redirectPathOnSuccess = routes.PathToOrganization(org.Username)
	}

	if userToSubscribeToTrial.HasSub() {
		http.Error(w, fmt.Sprintf("already on sub %d", userToSubscribeToTrial.SelfPaidSubscription), http.StatusBadRequest)
		return
	}
	_, err := h.userS.HandleManualPaymentSuccess(dbWrite, userToSubscribeToTrial.Id, user.Subscription_Trial, 1)
	if err != nil {
		log.Printf("failed to handle payment of trial sub %q", err)
		http.Error(w, "server err paying sub", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r.Request, redirectPathOnSuccess, http.StatusSeeOther)
	shouldCommit = true
	return
}

// handles incoming Stripe webhook events
func (h handler) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	event, ok := h.stripeClient.GetWebhookEvent(w, r)
	if !ok {
		return
	}
	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	// TODO: handle more evnts
	case "checkout.session.completed":
		h.handleCheckoutCompleted(w, event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(w, event)
	case "customer.subscription.updated":
		// https://docs.stripe.com/billing/subscriptions/webhooks
		// "Sent when a subscription starts or changes. For example, renewing a
		// subscription, adding a coupon, applying a discount, adding an
		// invoice item, and changing plans all trigger this event."
		h.handleSubscriptionUpdated(w, event)
	default:
		fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
	}
}

func (h handler) handleCheckoutCompleted(w http.ResponseWriter, e stripe.Event) {
	wl, ul, commit, err := h.db.BeginWrite()
	defer ul()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var s stripe.CheckoutSession
	if e.Data == nil {
		http.Error(w, "empty event Data", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(e.Data.Raw, &s); err != nil {
		http.Error(w, "failed to parse checkout.session.completed", http.StatusBadRequest)
		return
	}

	if s.ID == "" {
		http.Error(w, "missing client reference id", http.StatusBadRequest)
		return
	}
	if s.ClientReferenceID == "" {
		http.Error(w, "missing client reference id", http.StatusBadRequest)
		return
	}
	userId, err := strconv.ParseInt(s.ClientReferenceID, 10, 64)
	if err != nil {
		http.Error(w, "invalid client reference id", http.StatusBadRequest)
		return
	}
	if s.Subscription == nil {
		http.Error(w, "empty subscripton in checkout session", http.StatusBadRequest)
		return
	}
	subId := s.Subscription.ID
	if subId == "" {
		http.Error(w, "missing subscription id", http.StatusBadRequest)
		return
	}

	// Retrieve the Checkout Session again from Stripe, this time asking it
	// to include the line_items field. By default, the session object in the
	// webhook event does NOT include the line items — only the session ID
	// and references to other objects.
	sessionLineItems, err := h.stripeClient.GetSessionLineItems(s.ID)
	if err != nil {
		http.Error(w, "unable to fetch checkout session details", http.StatusInternalServerError)
		return
	}

	if len(sessionLineItems.Data) != 1 {
		http.Error(w, "invalid session line items data", http.StatusBadRequest)
		return
	}
	if sessionLineItems.Data[0] == nil {
		http.Error(w, "session line item is empty", http.StatusBadRequest)
		return
	}
	if sessionLineItems.Data[0].Price == nil {
		http.Error(w, "session line item price is empty", http.StatusBadRequest)
		return
	}

	priceId := stripeclient.PriceId(sessionLineItems.Data[0].Price.ID)

	quantity := sessionLineItems.Data[0].Quantity

	u, err := h.userS.HandleStripeCheckoutSessionSuccess(
		wl, userId, subId, s.ID, priceId, quantity)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if u.IsOrganization {
		err = h.orgHelper.EnforceOrgSeatLimit(wl, permissions.OrganizationAssetId(u.Id), quantity)
		if err != nil {
			http.Error(w, "internal error enforcing seats", http.StatusInternalServerError)
			return
		}
	}

	err = commit()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (h handler) handleSubscriptionDeleted(w http.ResponseWriter, e stripe.Event) {
	wl, ul, commit, err := h.db.BeginWrite()
	defer ul()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var subscription stripe.Subscription
	if err := json.Unmarshal(e.Data.Raw, &subscription); err != nil {
		http.Error(w, "failed to parse customer.subscription.deleted", http.StatusBadRequest)
		return
	}

	u, err := h.userS.HandlesSubscriptionDeleted(wl,
		subscription.Customer.ID, subscription.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if u.IsOrganization {
		err = h.orgHelper.EnforceOrgSeatLimit(
			wl,
			permissions.OrganizationAssetId(u.Id),
			1, // numberOfSeats
		)
		if err != nil {
			http.Error(w, "internal error enforcing seats", http.StatusInternalServerError)
			return
		}
	}

	err = commit()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (h handler) handleSubscriptionUpdated(w http.ResponseWriter, e stripe.Event) {
	wl, ul, commit, err := h.db.BeginWrite()
	defer ul()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var updatedSubscription stripe.Subscription
	if err := json.Unmarshal(e.Data.Raw, &updatedSubscription); err != nil {
		http.Error(w, "failed to parse customer.subscription.updated", http.StatusBadRequest)
		return
	}

	changed, newQuantity, err := h.subscriptionQuantityChanged(wl, updatedSubscription)
	if changed {
		u, err := h.userS.HandleSubscriptionQuantityUpdated(wl,
			updatedSubscription.Customer.ID, updatedSubscription.ID, newQuantity)
		if err != nil {
			log.Println("failed to HandleSubscriptionQuantityUpdated() got: ", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if u.IsOrganization {
			err = h.orgHelper.EnforceOrgSeatLimit(wl, permissions.OrganizationAssetId(u.Id), newQuantity)
			if err != nil {
				http.Error(w, "internal error enforcing seats", http.StatusInternalServerError)
				return
			}
		}
	}

	err = commit()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (h handler) handleSuccessPayment(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbRead context.Context) {
	http.Redirect(w, r.Request, routes.WelcomePage, http.StatusSeeOther)
}

// Start a stripe portal session for user then redirect him to it. Assumes user
// is authenticated and payment plan is active.
func (h handler) HandleManageSubscription(w http.ResponseWriter,
	r wrappers.UserWithSubMuxRequest, dbRead context.Context) {
	if r.UserWithSub.StripeSubscriptionID == "" {
		http.Error(w, "bad request: no stripe subscription",
			http.StatusBadRequest)
		return
	}
	portalSessionUrl, err := h.stripeClient.GetNewCustomerPortalSession(
		r.UserWithSub.Id, r.UserWithSub.StripeId)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r.Request, portalSessionUrl, http.StatusSeeOther)
}

// Returns if subscription changed for user: changed, newQuantity, err
func (h handler) subscriptionQuantityChanged(dbRead context.Context,
	updatedSubscription stripe.Subscription) (bool, int64, error) {

	u, isNotFoundErr, err := h.userS.GetByStripeId(dbRead, updatedSubscription.Customer.ID)
	if isNotFoundErr {
		log.Printf("called subscriptionQuantityChanged() with unknown customer stripe id=%v", updatedSubscription.Customer.ID)
		return false, 0, fmt.Errorf("got err=%w trying to read user in subscriptionQuantityChanged()", err)
	}
	if err != nil {
		return false, 0, err
	}

	if len(updatedSubscription.Items.Data) == 0 {
		return false, 0, fmt.Errorf("got updatedSubscription with no data in subscriptionQuantityChanged()")
	}

	newQuantity := updatedSubscription.Items.Data[0].Quantity

	if u.SelfPaidSubscriptionQuantity == newQuantity {
		return false, newQuantity, nil
	}

	return true, newQuantity, nil
}

// if !ok error already written in ResponseWriter
func (h handler) parseAndValidateOrganizationParam(
	w http.ResponseWriter,
	r wrappers.UserMuxRequest,
	dbRead context.Context,
) (isChoosingPlanForOrg bool, org user.User, userHasOwnersPermission, ok bool) {

	isChoosingPlanForOrgStr := r.FormValue(routes.IsChoosingPlanForOrgParamName)
	if isChoosingPlanForOrgStr != "" {
		var err error
		isChoosingPlanForOrg, err = strconv.ParseBool(isChoosingPlanForOrgStr)
		if err != nil {
			ok = false
			http.Error(w, fmt.Sprintf("invalid param:%q, got:%q", routes.IsChoosingPlanForOrgParamName, isChoosingPlanForOrgStr), http.StatusBadRequest)
			return
		}
	}

	if isChoosingPlanForOrg {
		organizationName := r.FormValue(routes.OrganizationNameParamName)
		if organizationName == "" {
			ok = false
			http.Error(w, "Got isChoosingPlanForOrg=true but empty orgName", http.StatusBadRequest)
			return
		}
		var isNotFoundErr bool
		var err error
		org, isNotFoundErr, err = h.userS.GetByUsername(dbRead, organizationName)
		if isNotFoundErr {
			ok = false
			http.Error(w, fmt.Sprintf("invalid param:%q, got:%q", routes.OrganizationNameParamName, organizationName), http.StatusBadRequest)
			return
		}
		if err != nil {
			ok = false
			log.Printf("failed to get org user: %s", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !org.IsOrganization {
			ok = false
			http.Error(w, fmt.Sprintf("invalid param:%q, got:%q", routes.OrganizationNameParamName, organizationName), http.StatusBadRequest)
			return
		}
		userHasOwnersPermission, err = h.db.HasPermission(
			dbRead,
			r.User.Id,
			permissions.Permission_OrganizationOwner,
			permissions.OrganizationAssetId(org.Id),
		)
		if err != nil {
			ok = false
			log.Printf("failed check permission in parseAndValidateOrganizationParam: %s", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	ok = true
	return
}
