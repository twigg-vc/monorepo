package stripeclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"monorepo/base/queue"
	"monorepo/twigg-web/routes"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/stripe/stripe-go/v82"
	bpConfig "github.com/stripe/stripe-go/v82/billingportal/configuration"
	bpSession "github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Serves as a test or the real stripe client
type stripeClient_ struct {
	stripeSecretKey           string
	teamProductId             string
	soloProductId             string
	teamLatestPriceId         PriceId
	soloLatestPriceId         PriceId
	portalConfigIsUpdated     bool
	portalConfigId            string
	sessionSuccessRedirectUrl string
	cancelSessionUrl          string
	portalRedirectUrl         string
	endpointSecret            string
	isTestClient              bool
}

// Returns a test client that makes http requests to stripe test environment
func newTestStripeClient(serverRootUrl string, stripeSecretKey, endpointSecret string) *stripeClient_ {
	sc := stripeClient_{
		// Test secret key source:
		// https://dashboard.stripe.com/acct_YOUR_ACCOUNT/test/apikeys
		stripeSecretKey:           stripeSecretKey,
		teamProductId:             "prod_T6XplETRCZGyzx",
		soloProductId:             "prod_T6jRgl3BuXnENa",
		teamLatestPriceId:         "price_1Swo2XFT2kvpBfQT660suFpi",
		soloLatestPriceId:         "price_1SAVyJFT2kvpBfQTVmMyTpDn",
		portalConfigIsUpdated:     false,
		portalConfigId:            "bpc_1SDXjaFT2kvpBfQTOgbn4wF4",
		sessionSuccessRedirectUrl: serverRootUrl + routes.StripeSuccessPaymentPath,
		cancelSessionUrl:          serverRootUrl + routes.Home,
		portalRedirectUrl:         serverRootUrl + routes.Home,
		endpointSecret:            endpointSecret,
		isTestClient:              true,
	}
	stripe.Key = sc.stripeSecretKey
	return &sc
}

// Returns the actual stripe client that makes requests to stripe's prod environment
func newStripeClient(serverRootUrl, stripeSecretKey, endpointSecret string) *stripeClient_ {
	sc := stripeClient_{
		stripeSecretKey:           stripeSecretKey,
		teamProductId:             "prod_T5092SkvbjfGDn",
		soloProductId:             "prod_T508epaxTorke8",
		teamLatestPriceId:         "price_1SmEerFT2kvpBfQTjAffzLpn",
		soloLatestPriceId:         "price_1SmEfwFT2kvpBfQTPLbdjmDM",
		portalConfigIsUpdated:     false,
		portalConfigId:            "bpc_1SDWAJFT2kvpBfQTi7EHyIs7",
		sessionSuccessRedirectUrl: serverRootUrl + routes.StripeSuccessPaymentPath,
		cancelSessionUrl:          serverRootUrl + routes.Home,
		portalRedirectUrl:         serverRootUrl + routes.Home,
		endpointSecret:            endpointSecret,
		isTestClient:              false,
	}
	stripe.Key = sc.stripeSecretKey
	return &sc
}

func (c stripeClient_) GetLatestTeamPriceId() PriceId {
	return c.teamLatestPriceId
}
func (c stripeClient_) GetLatestSoloPriceId() PriceId {
	return c.soloLatestPriceId
}

func (c stripeClient_) ResolvePriceId(priceId PriceId) (product Product, isOk bool) {
	// Team
	var teamPriceIds []PriceId
	if c.isTestClient {
		teamPriceIds = allTestTeamPriceIds
	} else {
		teamPriceIds = allTeamPriceIds
	}
	for _, pId := range teamPriceIds {
		if priceId == pId {
			return Product_Subscription_Team, true
		}
	}
	// Solo
	var soloPriceIds []PriceId
	if c.isTestClient {
		soloPriceIds = allTestSoloPriceIds
	} else {
		soloPriceIds = allSoloPriceIds
	}
	for _, pId := range soloPriceIds {
		if priceId == pId {
			return Product_Subscription_Solo, true
		}
	}
	return Product_None, false
}

func (c stripeClient_) GetNewStripeCustomer() (string, error) {
	cusParams := &stripe.CustomerParams{}
	cus, err := customer.New(cusParams)
	return cus.ID, err
}

func (sc stripeClient_) GetNewStripeSession(userID int64, stripeCustomerID string,
	priceId PriceId, quantity int64, forceCurrency string) (string, string, string, error) {

	product, isOk := sc.ResolvePriceId(priceId)
	if !isOk {
		panic("cloud not resolve priceId")
	}
	if product == Product_Subscription_Solo && quantity != 1 {
		panic("invalid quantity for solo plan")
	}

	clientReferenceId := strconv.FormatInt(userID, 10)
	params := sc.getCheckoutSessionParams(
		clientReferenceId, stripeCustomerID, product, priceId, quantity, forceCurrency)

	s, err := session.New(params)
	if err != nil {
		return "", "", "", errors.New("error creating stripe checkout session")
	}
	return clientReferenceId, s.ID, s.URL, nil
}
func (c stripeClient_) ExpireStripeSession(stripeSessionID string) error {
	params := &stripe.CheckoutSessionExpireParams{}
	_, expireSessionErr := session.Expire(stripeSessionID, params)
	// Return early if no error happens
	if expireSessionErr == nil {
		return nil
	}
	// If there's an error, we must check which kind it is. There are two cases:
	// 1 - Some random error (network, etc)
	// 2 - Stripe returned an error because the session was already expired.
	//
	// If 2, err is of type stripe.Error
	_, isStripeErr := expireSessionErr.(*stripe.Error)
	if !isStripeErr {
		// If not a stripe error, it's a random error; so we return it.
		return expireSessionErr
	}
	// If it was a stripe error, lets make sure that what happened was just that
	// the session was already expired. The way to check that is to get the
	// session from stripe.
	s, getSessionErr := session.Get(stripeSessionID, nil)
	if getSessionErr != nil {
		return fmt.Errorf("failed to get session from stripe: %s", getSessionErr)
	}
	if s.Status == stripe.CheckoutSessionStatusExpired ||
		s.Status == stripe.CheckoutSessionStatusComplete {
		// If the session was already expired or completed, that confirms
		// that we only got an error on expireSessionErr bc the session
		// was already expired.In this case, we just return nil from
		// this function.
		return nil
	}
	// If the session is not really expired, we don't know why we got an
	// error when tryinhg to expire the session. So let's just return that
	// errror.
	return expireSessionErr
}

func (c stripeClient_) GetSessionStatus(sessionId string) (SessionPaymentStatus, string, PriceId, int64,
	error) {
	sess, err := session.Get(sessionId, nil)
	if err != nil {
		return -1, "", "", -1, err
	}
	if sess.Status == "expired" {
		return SessionPaymentStatus_Expired, "", "", -1, nil
	}
	switch sess.PaymentStatus {
	case "no_payment_required", "paid":
	case "unpaid":
		return SessionPaymentStatus_Unpaid, "", "", -1, nil
	default:
		// Stripe docs says this should never happen
		panic("unexpected session payment status")
	}
	if sess.Subscription == nil {
		return -1, "", "", -1, fmt.Errorf("%s has no subscription id", sessionId)
	}
	subscriptionId := sess.Subscription.ID
	sessionLineItems, err := c.GetSessionLineItems(sessionId)
	if err != nil {
		return -1, "", "", -1, err
	}
	if len(sessionLineItems.Data) != 1 {
		return -1, "", "", -1, fmt.Errorf("no data for session %s", sessionId)
	}
	if sessionLineItems.Data[0] == nil {
		return -1, "", "", -1, fmt.Errorf("no items in data of session %s", sessionId)
	}
	if sessionLineItems.Data[0].Price == nil {
		return -1, "", "", -1, fmt.Errorf("no price in data of session %s", sessionId)
	}
	if sessionLineItems.Data[0].Quantity <= 0 {
		panic(fmt.Sprintf("got session %s with quantity <= 0", sessionId))
	}
	priceId := PriceId(sessionLineItems.Data[0].Price.ID)
	_, isOk := c.ResolvePriceId(priceId)
	if !isOk {
		panic(fmt.Sprintf("got session %s with invalid priceId=%q", sessionId, priceId))
	}
	quantity := sessionLineItems.Data[0].Quantity
	return SessionPaymentStatus_PaidOrNoPaymentRequired,
		subscriptionId, priceId, quantity, nil
}

func (c *stripeClient_) GetNewCustomerPortalSession(
	serID int64, stripeCustomerID string) (portalSessionUrl string, err error) {
	if !c.portalConfigIsUpdated {
		err = c.updateBillingPortalConfig()
		if err != nil {
			return "", err
		}
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(c.portalRedirectUrl),
		// This config was created in stripe billing
		// Here: https://dashboard.stripe.com/acct_1S8mObFT2kvpBfQT/settings/billing/portal
		Configuration: stripe.String(c.portalConfigId),
	}

	s, err := bpSession.New(params)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (c stripeClient_) getCheckoutSessionParams(stripeClientReferenceID string,
	stripeCustomerID string,
	product Product,
	priceId PriceId,
	quantity int64, forceCurrency string) *stripe.CheckoutSessionParams {
	lineItem := &stripe.CheckoutSessionLineItemParams{
		Price:    stripe.String(priceId),
		Quantity: stripe.Int64(quantity),
	}

	if product == Product_Subscription_Team {
		lineItem.AdjustableQuantity = &stripe.CheckoutSessionLineItemAdjustableQuantityParams{
			Enabled: stripe.Bool(true),
			Maximum: stripe.Int64(999999),
			Minimum: stripe.Int64(1),
		}
	}

	params := &stripe.CheckoutSessionParams{
		Customer:            stripe.String(stripeCustomerID),
		ClientReferenceID:   stripe.String(stripeClientReferenceID),
		Mode:                stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems:           []*stripe.CheckoutSessionLineItemParams{lineItem},
		SuccessURL:          stripe.String(c.sessionSuccessRedirectUrl),
		CancelURL:           stripe.String(c.cancelSessionUrl),
		AllowPromotionCodes: stripe.Bool(true),
	}
	if forceCurrency == "brl" {
		params.Currency = stripe.String("brl")
	}
	if forceCurrency == "usd" {
		params.Currency = stripe.String("usd")
	}
	return params
}
func (c *stripeClient_) updateBillingPortalConfig() error {
	prams := &stripe.BillingPortalConfigurationParams{
		BusinessProfile: &stripe.BillingPortalConfigurationBusinessProfileParams{
			Headline: stripe.String("Twigg manage subscription"), // Portal header
		},
		Features: &stripe.BillingPortalConfigurationFeaturesParams{
			InvoiceHistory: &stripe.BillingPortalConfigurationFeaturesInvoiceHistoryParams{
				Enabled: stripe.Bool(true), // view open & paid invoices
			},
			PaymentMethodUpdate: &stripe.BillingPortalConfigurationFeaturesPaymentMethodUpdateParams{
				Enabled: stripe.Bool(true), // allow payment-method updates
			},
			SubscriptionUpdate: &stripe.BillingPortalConfigurationFeaturesSubscriptionUpdateParams{
				Enabled: stripe.Bool(true),
				DefaultAllowedUpdates: []*string{
					stripe.String("quantity"),
				},
				Products: []*stripe.BillingPortalConfigurationFeaturesSubscriptionUpdateProductParams{
					{
						AdjustableQuantity: &stripe.BillingPortalConfigurationFeaturesSubscriptionUpdateProductAdjustableQuantityParams{
							Enabled: stripe.Bool(true),
							Minimum: stripe.Int64(1),
							Maximum: stripe.Int64(1_000_000),
						},
						Prices:  []*string{(*string)(&c.teamLatestPriceId)},
						Product: stripe.String(c.teamProductId),
					},
				},
				ProrationBehavior: stripe.String("create_prorations"),
			},
			SubscriptionCancel: &stripe.BillingPortalConfigurationFeaturesSubscriptionCancelParams{
				Enabled:           stripe.Bool(true),
				Mode:              stripe.String("immediately"),       // cancel immediately
				ProrationBehavior: stripe.String("create_prorations"), // prorate cancel
				CancellationReason: &stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonParams{
					Enabled: stripe.Bool(true),
					Options: stripe.StringSlice([]string{
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionCustomerService),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionLowQuality),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionMissingFeatures),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionOther),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionSwitchedService),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionTooComplex),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionTooExpensive),
						string(stripe.BillingPortalConfigurationFeaturesSubscriptionCancelCancellationReasonOptionUnused),
					}),
				},
			},
		},
	}
	updatedConfig, err := bpConfig.Update(c.portalConfigId, prams)
	if err != nil {
		return err
	}
	if updatedConfig.ID != c.portalConfigId {
		return fmt.Errorf("got updatedConfig.ID=%v and c.portalConfigId=%v are different", updatedConfig.ID, c.portalConfigId)
	}
	c.portalConfigIsUpdated = true
	return nil
}

func (c *stripeClient_) GetSessionLineItems(sessionId string) (stripe.LineItemList, error) {
	// The Expand parameter tells Stripe's API to return the specified nested
	// field (here "line_items") as part of the session object, so we don't
	// need to make a separate call to /checkout/sessions/{id}/line_items.
	sessionWithItems, err := session.Get(sessionId, &stripe.CheckoutSessionParams{
		Expand: []*string{stripe.String("line_items")},
	})
	if err != nil {
		return stripe.LineItemList{}, err
	}
	return *sessionWithItems.LineItems, nil
}

func (c *stripeClient_) GetWebhookEvent(w http.ResponseWriter, r *http.Request) (event stripe.Event, ok bool) {
	const MaxBodyBytes = int64(65536)
	bodyReader := http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(bodyReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading request body: %v\n", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	signatureHeader := r.Header.Get("Stripe-Signature")
	event, err = webhook.ConstructEvent(payload, signatureHeader, c.endpointSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Webhook signature verification failed. %v\n", err)
		w.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
		return
	}
	ok = true
	return
}

// ========= Mock: Doesn't make a http request to stripe test mode  =========
type mockSession struct {
	userID            int64 // the user ID associated with this session
	stripeCustomerId  string
	clientReferenceId string
	quantity          int64
	sessionUrl        string
	status            SessionPaymentStatus
	subscriptionId    string
	priceId           PriceId
}
type mockStripeClient struct {
	lasUserIdReceived     int64
	lastIdUsed            int
	lastSessionUsed       int
	lasSubscriptionIdUsed int
	// stripeSessionID -> mockSession
	sessions             map[string]mockSession
	sessionIdToLineItems map[string]stripe.LineItemList
	webhookEvents        queue.Queue[stripe.Event]
}

func newMockStripeClient() StripeClientMock {
	return &mockStripeClient{
		sessions:             make(map[string]mockSession),
		sessionIdToLineItems: make(map[string]stripe.LineItemList),
		webhookEvents:        queue.New[stripe.Event](),
	}
}

func (f *mockStripeClient) GetNewStripeCustomer() (string, error) {
	f.lastIdUsed++
	return fmt.Sprintf("stripe_mock_cus_%d", f.lastIdUsed), nil
}
func (f mockStripeClient) GetLastStripeCustomer() (stripeCustomerID string) {
	return fmt.Sprintf("stripe_mock_cus_%d", f.lastIdUsed)
}

func (m *mockStripeClient) GetNewStripeSession(userID int64,
	stripeCustomerID string, priceId PriceId, quantity int64, forceCurrency string) (string, string, string, error) {
	m.lasUserIdReceived = userID
	m.lastSessionUsed++
	m.lastIdUsed++

	product, isOk := m.ResolvePriceId(priceId)
	if !isOk {
		panic("cloud not resolve priceId")
	}
	if product == Product_Subscription_Solo && quantity != 1 {
		panic("invalid quantity for solo plan")
	}
	newSessionId := fmt.Sprintf("cs_mock_%d", m.lastIdUsed)
	mSession := mockSession{
		userID:            userID,
		stripeCustomerId:  stripeCustomerID,
		clientReferenceId: strconv.FormatInt(userID, 10),
		quantity:          quantity,
		sessionUrl:        fmt.Sprintf("/?mock-stripe-session=%d", m.lastSessionUsed),
		status:            SessionPaymentStatus_Unpaid,
	}
	m.sessions[newSessionId] = mSession
	return mSession.clientReferenceId, newSessionId, mSession.sessionUrl, nil
}
func (f mockStripeClient) GetLastStripeSession() (stripeClientReferenceID, stripeSessionID, stripeSessionUrl string) {
	stripeSessionID = fmt.Sprintf("cs_mock_%d", f.lastIdUsed)
	stripeSessionUrl = fmt.Sprintf("/?mock-stripe-session=%d", f.lastSessionUsed)
	stripeClientReferenceID = strconv.FormatInt(f.lasUserIdReceived, 10)
	return
}
func (f *mockStripeClient) ExpireStripeSession(stripeSessionID string) error {
	s, exists := f.sessions[stripeSessionID]
	if !exists {
		return errors.New("session not found")
	}
	if s.status == SessionPaymentStatus_Expired {
		return nil
	}
	if s.status == SessionPaymentStatus_PaidOrNoPaymentRequired {
		return errors.New("session already paid")
	}
	s.status = SessionPaymentStatus_Expired
	f.sessions[stripeSessionID] = s
	return nil
}
func (m *mockStripeClient) GetNewCustomerPortalSession(
	serID int64, stripeCustomerID string) (portalSessionUrl string, err error) {
	m.lastSessionUsed++
	return fmt.Sprintf("/?mock-portal-session=%d", m.lastSessionUsed), nil
}
func (m *mockStripeClient) GetLatestTeamPriceId() PriceId {
	return "mock_price_id_team"
}
func (m *mockStripeClient) GetLatestSoloPriceId() PriceId {
	return "mock_price_id_solo"
}
func (m *mockStripeClient) ResolvePriceId(priceId PriceId) (product Product, isOk bool) {
	if priceId == "mock_price_id_team" {
		return Product_Subscription_Team, true
	}
	if priceId == "mock_price_id_solo" {
		return Product_Subscription_Solo, true
	}
	return Product_None, false
}

func (m *mockStripeClient) GetSessionLineItems(sessionId string) (stripe.LineItemList, error) {
	li, ok := m.sessionIdToLineItems[sessionId]
	if !ok {
		return stripe.LineItemList{}, errors.New("not found")
	}
	return li, nil
}
func (m *mockStripeClient) GetWebhookEvent(w http.ResponseWriter, r *http.Request) (ev stripe.Event, ok bool) {
	if m.webhookEvents.IsEmpty() {
		ok = false
		return
	}
	ev = m.webhookEvents.Pop()
	ok = true
	return
}

func (m mockStripeClient) GetSessionStatus(sessionId string) (st SessionPaymentStatus,
	subscriptionID string, priceId PriceId, quantity int64,
	err error) {
	s, exist := m.sessions[sessionId]
	if !exist {
		err = errors.New("not found")
		return
	}
	st = s.status
	subscriptionID = s.subscriptionId
	priceId = s.priceId
	quantity = s.quantity
	return
}

// ================== Mock methods ==================
func (m *mockStripeClient) CheckSessionWasExpired(sessionId string, t testing.TB) {
	st, _, _, _, err := m.GetSessionStatus(sessionId)
	if err != nil {
		t.Fatalf("session %s not found", sessionId)
	}
	if st != SessionPaymentStatus_Expired {
		t.Fatalf("session %s was not expired", sessionId)
	}
}

func (m *mockStripeClient) CheckSessionIsUnpaid(sessionId string, t testing.TB) {
	st, _, _, _, err := m.GetSessionStatus(sessionId)
	if err != nil {
		t.Fatalf("session %s not found", sessionId)
	}
	if st != SessionPaymentStatus_Unpaid {
		t.Fatalf("session %s is not unpaid", sessionId)
	}
}
func (m *mockStripeClient) CheckSessionIsPaid(sessionId string, t testing.TB) {
	st, _, _, _, err := m.GetSessionStatus(sessionId)
	if err != nil {
		t.Fatalf("session %s not found", sessionId)
	}
	if st != SessionPaymentStatus_PaidOrNoPaymentRequired {
		t.Fatalf("session %s is not paid", sessionId)
	}
}
func (m *mockStripeClient) SessionUserID(sId string) int64 {
	s, exist := m.sessions[sId]
	if !exist {
		panic("session should exist")
	}
	return s.userID
}
func (m *mockStripeClient) SessionQuantity(sId string) int64 {
	s, exist := m.sessions[sId]
	if !exist {
		panic("session should exist")
	}
	return s.quantity
}

func (m *mockStripeClient) MockSessionPaid(
	sessionId string, priceId PriceId, quantity int64) (subscriptionId string) {
	s, exist := m.sessions[sessionId]
	if !exist {
		panic("tried to mock payment of non existing session")
	}
	if s.status != SessionPaymentStatus_Unpaid {
		panic("tried to mock payment of session that is not unpaid")
	}
	// Get a new subscriptionId
	m.lasSubscriptionIdUsed++
	subscriptionId = fmt.Sprintf("subscription-%d", m.lasSubscriptionIdUsed)

	// Update the session
	s.status = SessionPaymentStatus_PaidOrNoPaymentRequired
	s.priceId = priceId
	s.quantity = quantity
	s.subscriptionId = subscriptionId
	m.sessions[sessionId] = s

	// Add a mock event to the queue of webhook events
	mockedCheckoutSession := stripe.CheckoutSession{
		ID:                sessionId,
		ClientReferenceID: s.clientReferenceId,
		Subscription: &stripe.Subscription{
			ID: subscriptionId,
		},
	}
	mockedChechoutSessionBytes, _ := json.Marshal(mockedCheckoutSession)
	m.webhookEvents.Push(stripe.Event{
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: mockedChechoutSessionBytes,
		},
	})
	m.sessionIdToLineItems[sessionId] = stripe.LineItemList{
		Data: []*stripe.LineItem{
			{
				Price: &stripe.Price{
					ID: string(priceId),
				},
				Quantity: quantity,
			},
		},
	}
	return
}

func (m *mockStripeClient) GetLastStripeSubscription() (subscriptionId, stripeCustomerID string) {
	_, sessionId, _ := m.GetLastStripeSession()
	s, ok := m.sessions[sessionId]
	if !ok {
		panic("no subscription used yet")
	}
	subscriptionId = s.subscriptionId
	stripeCustomerID = s.stripeCustomerId
	return
}

func (m *mockStripeClient) MockSubscriptionCanceled(subscriptionId, stripeCustomerID string) {
	var sub stripe.Subscription
	sub.ID = subscriptionId
	sub.Customer = &stripe.Customer{ID: stripeCustomerID}
	mockedChechoutSessionBytes, _ := json.Marshal(sub)
	m.webhookEvents.Push(stripe.Event{
		Type: "customer.subscription.deleted",
		Data: &stripe.EventData{
			Raw: mockedChechoutSessionBytes,
		},
	})
}
func (m *mockStripeClient) MockSubscriptionUpdatedQuantity(
	subscriptionId, stripeCustomerID string, quantity int64) {
	if quantity <= 0 {
		panic("quantity can not be < 0")
	}
	var sub stripe.Subscription
	sub.ID = subscriptionId
	sub.Customer = &stripe.Customer{ID: stripeCustomerID}
	sub.Items = &stripe.SubscriptionItemList{
		Data: []*stripe.SubscriptionItem{
			{
				Quantity: quantity,
			},
		},
	}
	mockedChechoutSessionBytes, _ := json.Marshal(sub)
	m.webhookEvents.Push(stripe.Event{
		Type: "customer.subscription.updated",
		Data: &stripe.EventData{
			Raw: mockedChechoutSessionBytes,
		},
	})
}

var allTeamPriceIds = []PriceId{
	"price_1SmEerFT2kvpBfQTjAffzLpn",
	"price_1S8q9XFT2kvpBfQTVJyVgXYP",
}
var allTestTeamPriceIds = []PriceId{
	"price_1Swo2XFT2kvpBfQT660suFpi",
	"price_1SAKk5FT2kvpBfQTDNzRWXJf",
}

var allSoloPriceIds = []PriceId{
	"price_1SmEfwFT2kvpBfQTPLbdjmDM",
	"price_1S8q86FT2kvpBfQTuKdzaCGo",
}
var allTestSoloPriceIds = []PriceId{
	"price_1SAVyJFT2kvpBfQTVmMyTpDn",
}
