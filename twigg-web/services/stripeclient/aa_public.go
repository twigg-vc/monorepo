package stripeclient

import (
	"net/http"
	"testing"

	"github.com/stripe/stripe-go/v82"
)

// A Product may be associated with multiple Stripe PriceIds over time,
// since Stripe creates a new Price whenever pricing details
// (amount, currency, billing interval, etc.) are changed.
type Product int

const (
	Product_None Product = iota
	Product_Subscription_Solo
	Product_Subscription_Team
)

type PriceId string

type StripeClient interface {
	// Creates a new customer in Stripe and returns its unique ID. The returned
	// stripeCustomerID serves as a permanent identifier for the user in Stripe.
	// Once created, this ID should never be modified or reassigned.
	GetNewStripeCustomer() (stripeCustomerID string, err error)

	// GetNewStripeSession creates a new Stripe Checkout session for the
	// given user. Stripe needs customer id and price id. The userID parameter
	// is your own application's user identifier. We pass it to Stripe so that
	// when Stripe fires a `checkout.session.completed` webhook event, we can
	// know which of our users initiated the checkout. Without it, the webhook
	// would only contain the Stripe customer ID, and we'd have to look up the
	// mapping between Stripe's customer id and our own user id.
	//
	// The priceId must be one of the 2 supported
	// values corresponding to product prices in Stripe:
	//
	// 		- PriceId_Team
	// 		- PriceId_Solo**
	//
	// If an invalid priceId is provided, the function panics. On success,
	// returns a session id that unique identifies a session. and a
	// stripeSessionURL that points to the Stripe Checkout page where the
	// customer can complete the purchase of the chosen plan.
	// The `quantity` parameter specifies how many how many units (e.g., seats)
	// the customer is purchasing in this session. **For Solo plan quantity can
	// only be 1, any other value cause these function to a panic.
	//
	// See your Stripe dashboard for product configuration:
	// https://dashboard.stripe.com/acct_YOUR_ACCOUNT/products
	//
	// forceCurrency can be one of the supported currencies (brl/usd). It's
	// ignored otherwise
	GetNewStripeSession(userID int64, stripeCustomerID string, priceId PriceId,
		quantity int64, forceCurrency string) (stripeClientReferenceID, stripeSessionID, stripeSessionUrl string, err error)

	// Expires an existing Checkout Session in Stripe.
	// Once expired, a user can no longer pay the session.
	// Calling this function many times is ok (should return nil).
	ExpireStripeSession(stripeSessionID string) error

	// Returns the status of a payment session.
	// subscriptionID, priceId and quantity are set only for sessions that
	// were paid (i.e. PaidOrNoPaymentRequired)
	GetSessionStatus(sessionId string) (st SessionPaymentStatus,
		subscriptionID string, priceId PriceId, quantity int64,
		err error)

	// PriceId represents a specific product configured in your Stripe account.
	// It is used to tell Stripe which subscription plan the customer is purchasing.
	GetLatestTeamPriceId() PriceId
	GetLatestSoloPriceId() PriceId

	ResolvePriceId(priceId PriceId) (product Product, isOk bool)

	// Creates a stripe billing customer portal session for a given customer.
	// The stripe billing customer portal is a stripe-hosted UI for subscription
	// and billing management.
	// more: https://docs.stripe.com/api/customer_portal/sessions
	//
	// On success, returns a portalSessionUrl that points to the stripe billing
	// customer portal page where customer can menage subscription and billing.
	GetNewCustomerPortalSession(userID int64, stripeCustomerID string) (portalSessionUrl string, err error)

	// Returns the line items of a session
	GetSessionLineItems(sessionId string) (stripe.LineItemList, error)

	// Returns the event that is posted to a webhook.
	// On any errors, write the error response and returns ok=false.
	GetWebhookEvent(w http.ResponseWriter, r *http.Request) (ev stripe.Event, ok bool)
}

// Returns production stripe client
func NewStripeClient(serverRootUrl, stripeSecretKey, endpointSecret string) StripeClient {
	return newStripeClient(serverRootUrl, stripeSecretKey, endpointSecret)
}

// Returns a test client that makes requests to stripe test environment
func NewTestStripeClient(serverRootUrl, stripeSecretKey, endpointSecret string) StripeClient {
	return newTestStripeClient(serverRootUrl, stripeSecretKey, endpointSecret)
}

// ========= Mock: Doesn't make a http request to stripe test mode  =========
type StripeClientMock interface {
	StripeClient

	// Fails test if not found or not expired status
	CheckSessionWasExpired(sessionId string, t testing.TB)
	// Fails test if not found or not unpaid status
	CheckSessionIsUnpaid(sessionId string, t testing.TB)
	// Fails test if not found or status is not paid/no-payment-required
	CheckSessionIsPaid(sessionId string, t testing.TB)

	// Returns the most recent result of GetNewStripeCustomer
	GetLastStripeCustomer() (stripeCustomerID string)
	// Returns most recent result of by GetNewStripeSession
	GetLastStripeSession() (stripeClientReferenceID, stripeSessionID, stripeSessionUrl string)
	// Returns the most recent result subscription created
	GetLastStripeSubscription() (subscriptionId, stripeCustomerID string)

	SessionUserID(sId string) int64   // If session dees not exist it panics
	SessionQuantity(sId string) int64 // If session dees not exist it panics

	// Mocks that the provided session id was paid in stripe.
	// Returns the subscription id created.
	// Next calls to GetSessionStatus will return that this session was paid.
	// Next time GetWebhookEvent is called, an event equivalent to this
	// payment will be provided.
	MockSessionPaid(sessionId string, priceId PriceId, quantity int64) (subscriptionId string)

	// Mock that the provided subscription was canceled.
	// Next time GetWebhookEvent is called, it'll return an event indicating
	// this cancelation
	MockSubscriptionCanceled(subscriptionId, stripeCustomerID string)

	// Mock that subscription quantity was updated. Next time GetWebhookEvent
	// is called, it'll return an event indicating this update
	MockSubscriptionUpdatedQuantity(subscriptionId, stripeCustomerID string, quantity int64)
}

func NewMockStripeClient() StripeClientMock {
	return newMockStripeClient()
}

type SessionPaymentStatus int

const (
	SessionPaymentStatus_Unpaid SessionPaymentStatus = iota
	SessionPaymentStatus_PaidOrNoPaymentRequired
	SessionPaymentStatus_Expired
)
