package payments

import (
	"context"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/session"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"net/http"

	"github.com/stripe/stripe-go/v82"
)

func AddHandlers(
	db Db,
	sessionS session.Service,
	userS user.Service,
	stripeClient StripeClient,
	orgHelper OrgHelper,
	mux wrappers.RlMux,
	userMux wrappers.UserMux,
	userWithSubMux wrappers.UserWithSubMux) {
	h := handler{
		db,
		sessionS,
		userS,
		stripeClient,
		orgHelper,
	}

	mux.HandleFunc("POST "+routes.StripeWebhook, h.handleStripeWebhook)
	userMux.HandleFuncW("POST "+routes.SubscribeWithStripeUrl,
		h.cancelTrialIfNeededAndCreateStripeSession)
	userMux.HandleFuncW("POST "+routes.SubscribeTrialUrl, h.handleSubscribeTrial)
	userWithSubMux.HandleFuncR("GET "+routes.ManageSubscriptionWithStripeUrl,
		h.HandleManageSubscription)
	userWithSubMux.HandleFuncR("GET "+routes.StripeSuccessPaymentPath,
		h.handleSuccessPayment)
}

type Db interface {
	BeginWrite() (writeCtx context.Context, closeTx func(), commit func() error, err error)
	HasPermission(rl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error)
}

type UserService interface {
	HandleManualSubscriptionDeleted(w context.Context, userId int64) error
	GetUserForPaymentWithStripe(w context.Context, id int64, priceId stripeclient.PriceId, quantity int64, forceCurrency string) (u user.User, isNotFoundErr bool, err error)
	HandleManualPaymentSuccess(w context.Context, userId int64, sub user.SubscriptionPlan, subQuantity int64) (user.User, error)
	HandleStripeCheckoutSessionSuccess(w context.Context, id int64, stripeSubscriptionID, stripeSessionId string, priceId stripeclient.PriceId, quantity int64) (user.User, error)
	HandlesSubscriptionDeleted(w context.Context, stripeId, subscriptionId string) (user.User, error)
	HandleSubscriptionQuantityUpdated(w context.Context, stripeId, subscriptionId string, updatedQuantity int64) (user.User, error)
	GetByStripeId(r context.Context, stripeId string) (u user.User, isNotFoundErr bool, err error)
	GetByUsername(r context.Context, username string) (u user.User, isNotFoundErr bool, err error)
}

type StripeClient interface {
	GetLatestTeamPriceId() stripeclient.PriceId
	GetLatestSoloPriceId() stripeclient.PriceId
	ResolvePriceId(priceId stripeclient.PriceId) (product stripeclient.Product, isOk bool)
	GetNewCustomerPortalSession(userID int64, stripeCustomerID string) (portalSessionUrl string, err error)
	GetSessionLineItems(sessionId string) (stripe.LineItemList, error)
	GetWebhookEvent(w http.ResponseWriter, r *http.Request) (ev stripe.Event, ok bool)
}

type OrgHelper interface {
	EnforceOrgSeatLimit(wl context.Context, orgAssetId string, numberOfSeats int64) error
}
