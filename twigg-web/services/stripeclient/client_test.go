package stripeclient

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stripe/stripe-go/v82"
	bpConfig "github.com/stripe/stripe-go/v82/billingportal/configuration"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
)

const (
	StripeSecretKeyEnvVarName      = "STRIPE_SECRET_KEY"
	StripeEndpointSecretEnvVarName = "STRIPE_ENDPOINT_SECRET"
)

func TestMain(m *testing.M) {
	// Uncomment the following lines and set the appropriate values to actually
	// run the test using stripe test env:
	// os.Setenv(StripeSecretKeyEnvVarName, "sk_test_51 ... ")
	// os.Setenv(StripeEndpointSecretEnvVarName, "whsec_23 ...")

	os.Exit(m.Run())
}

// ================ Test CreateCustomer() ================
// Check in stripe:
// 1. Go to "https://dashboard.stripe.com/acct_YOUR_ACCOUNT/test/customers"
// *acct_YOUR_ACCOUNT = changes from user
// If your are not finding just search on the search bar "Customers"
//
// 2. All Customers create will appear in this page
func TestFakeGetNewStripeCustomer(t *testing.T) {
	testStripeSecretKey := os.Getenv(StripeSecretKeyEnvVarName)
	testStripeEndpointSecret := os.Getenv(StripeEndpointSecretEnvVarName)
	if testStripeSecretKey == "" || testStripeEndpointSecret == "" {
		t.Skip()
		return
	}
	fakeClient := NewTestStripeClient("http://localhost:9001", testStripeSecretKey, testStripeEndpointSecret)
	stripeCustomerID, err := fakeClient.GetNewStripeCustomer()
	if err != nil {
		t.Fatal(err)
	}
	if stripeCustomerID == "" {
		t.Fatal("this should never happen")
	}

	// Verify it exists on Stripe
	cust, err := customer.Get(stripeCustomerID, nil)
	if err != nil {
		t.Fatalf("expected to retrieve customer from Stripe, got %v", err)
	}
	if cust.ID != stripeCustomerID {
		t.Fatalf("expected to get same customer ID back, got %+v", cust)
	}
}

// ================ Test GetNewStripeSession() ================
// Check in stripe:
// 1. Go to "https://dashboard.stripe.com/acct_YOUR_ACCOUNT/test/workbench/logs"
// *acct_YOUR_ACCOUNT = changes from user
// If your are not finding just search on the search bar "logs"
//
// 2. All logs will appear in this page
//
// 3. You can see creating customer, starting its session and expiring session
func TestFakeNewAndExpireStripeSession(t *testing.T) {
	testStripeSecretKey := os.Getenv(StripeSecretKeyEnvVarName)
	testStripeEndpointSecret := os.Getenv(StripeEndpointSecretEnvVarName)
	if testStripeSecretKey == "" || testStripeEndpointSecret == "" {
		t.Skip()
		return
	}
	fakeClient := newTestStripeClient("http://localhost:9001", testStripeSecretKey, testStripeEndpointSecret)
	stripeCustomerID, err := fakeClient.GetNewStripeCustomer()
	if err != nil {
		t.Fatal(err)
	}
	clientRefId, stripeSessionID, stripeSessionUrl, err :=
		fakeClient.GetNewStripeSession(99, stripeCustomerID,
			fakeClient.GetLatestTeamPriceId(),
			/*quantity*/ 5,
			/*forceCurrency*/ "")
	if err != nil {
		t.Fatal(err)
	}
	if clientRefId == "" || stripeSessionID == "" || stripeSessionUrl == "" {
		t.Fatalf("failed to create session: id=%q url=%q err=%q",
			stripeSessionID, stripeSessionUrl, err)
	}
	// Verify exists on Stripe
	s, err := session.Get(stripeSessionID, nil)
	if err != nil {
		t.Fatalf("session not found on stripe: %v", err)
	}
	if clientRefId != "99" {
		t.Fatal("invalid clientRefId")
	}
	userId, err := strconv.ParseInt(s.ClientReferenceID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if userId != 99 {
		t.Fatal("wrong user id")
	}

	// Delete
	err = fakeClient.ExpireStripeSession(stripeSessionID)
	if err != nil {
		t.Fatal(err)
	}
	// Verify it was inactive
	s, err = session.Get(stripeSessionID, nil)
	if err != nil {
		t.Fatalf("got error %q when getting deleted session", err)
	}
	if s.Status != stripe.CheckoutSessionStatusExpired {
		t.Fatalf("expected session to be expired")
	}
}

// ================ Test GetNewCustomerPortalSession() ================
// Check in stripe:
// 1. Go to "https://dashboard.stripe.com/acct_YOUR_ACCOUNT/test/workbench/logs"
// *acct_YOUR_ACCOUNT = changes from user
// If your are not finding just search on the search bar "logs"
//
// 2. All logs will appear in this page
//
// 3. You can see creating portal session for. Just check customer id
// (stripe id) and thats it
func TestFakeGetNewCustomerPortalSession(t *testing.T) {
	testStripeSecretKey := os.Getenv(StripeSecretKeyEnvVarName)
	testStripeEndpointSecret := os.Getenv(StripeEndpointSecretEnvVarName)
	if testStripeSecretKey == "" || testStripeEndpointSecret == "" {
		t.Skip()
		return
	}
	fakeClient := newTestStripeClient("http://localhost:9001", testStripeSecretKey, testStripeEndpointSecret)
	stripeCustomerID, err := fakeClient.GetNewStripeCustomer()
	if err != nil {
		t.Fatal(err)
	}
	portalSessionUrl, err :=
		fakeClient.GetNewCustomerPortalSession(0, stripeCustomerID)
	if err != nil {
		t.Fatal(err)
	}
	if portalSessionUrl == "" {
		t.Fatal("this should never happen")
	}
	if !fakeClient.portalConfigIsUpdated {
		t.Fatalf("expected to be updated")
	}

	// Quick sanity check: Stripe Portal URLs usually start with
	// https://billing.stripe.com
	if !strings.HasPrefix(portalSessionUrl, "https://billing.stripe.com/") {
		t.Fatalf("unexpected portal session URL: %s", portalSessionUrl)
	}

	result, err := bpConfig.Get(fakeClient.portalConfigId, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Subscription Cancel
	if !result.Features.SubscriptionCancel.Enabled ||
		result.Features.SubscriptionCancel.Mode != "immediately" ||
		result.Features.SubscriptionCancel.ProrationBehavior != "create_prorations" {
		t.Fatalf("expected subscription cancellation to be allowed, immediately and with prorations")
	}
	// Subscription Update
	if !result.Features.SubscriptionUpdate.Enabled {
		t.Fatalf("gotresult.Features.SubscriptionUpdate.Enabled=%v",
			result.Features.SubscriptionUpdate.Enabled)
	}
}

// =============================== Test Mock ===============================
// This test don't make http request so we can let them uncommented
func TestMock(t *testing.T) {
	mock := NewMockStripeClient()
	fakeStripeCustomerID, err := mock.GetNewStripeCustomer()
	if err != nil {
		t.Fatal(err)
	}
	if fakeStripeCustomerID == "" {
		t.Fatal("this should never happen")
	}

	fakeSecondStripeCustomerID, err := mock.GetNewStripeCustomer()
	if err != nil {
		t.Fatal(err)
	}
	if fakeStripeCustomerID == fakeSecondStripeCustomerID {
		t.Fatal("they should be different")
	}
	lastCustomerId := mock.GetLastStripeCustomer()
	if lastCustomerId != fakeSecondStripeCustomerID {
		t.Fatal("unexpeced GetLastStripeCustomer")
	}

	clientRefId, sId, sUrl, err := mock.GetNewStripeSession(
		66, fakeStripeCustomerID, mock.GetLatestTeamPriceId() /*quantity*/, 10,
		/*forceCurrency*/ "")
	if err != nil {
		t.Fatal(err)
	}
	if clientRefId != "66" {
		t.Fatalf("invalid clientRefId: %s", clientRefId)
	}
	if sUrl == "" {
		t.Fatal("invalid session url")
	}
	mock.CheckSessionIsUnpaid(sId, t)
	if mock.SessionUserID(sId) != 66 {
		t.Fatal("session user id should be 66")
	}
	if mock.SessionQuantity(sId) != 10 {
		t.Fatal("session Quantity be 10")
	}
	lastClientRefId, lastSId, lastSUrl := mock.GetLastStripeSession()
	if lastClientRefId != clientRefId || lastSId != sId || lastSUrl != sUrl {
		t.Fatal("unexpeced GetLastStripeSession")
	}

	clientRefId, sIdSecond, sUrlSecond, err := mock.GetNewStripeSession(
		420, fakeSecondStripeCustomerID, mock.GetLatestSoloPriceId() /*quantity*/, 1,
		/*forceCurrency*/ "")
	if err != nil {
		t.Fatal(err)
	}
	if clientRefId != "420" {
		t.Fatal("wrong client ref id")
	}
	if sUrl == sUrlSecond {
		t.Fatal("urls should be different")
	}
	if sId == sIdSecond {
		t.Fatal("ids should be different")
	}
	mock.CheckSessionIsUnpaid(sIdSecond, t)
	if mock.SessionUserID(sIdSecond) != 420 {
		t.Fatal("session user id should be 420")
	}
	if mock.SessionQuantity(sIdSecond) != 1 {
		t.Fatal("session Quantity be 1")
	}

	// Deleting many times should be ok
	err = mock.ExpireStripeSession(sId)
	if err != nil {
		t.Fatal(err)
	}
	err = mock.ExpireStripeSession(sId)
	if err != nil {
		t.Fatal(err)
	}
	mock.CheckSessionWasExpired(sId, t)

	// Deting non-existing should err
	err = mock.ExpireStripeSession("non existing")
	if err == nil {
		t.Fatal("got nil err deleting non exsiting session")
	}

	// Portal session
	pSessionUrl, err := mock.GetNewCustomerPortalSession(0, sId)
	if err != nil {
		t.Fatal(err)
	}
	if pSessionUrl == "" {
		t.Fatal("invalid portal session")
	}
	newestPortalSession, err := mock.GetNewCustomerPortalSession(0, sId)
	if err != nil {
		t.Fatal(err)
	}
	if pSessionUrl == newestPortalSession {
		t.Fatal("they should be different")
	}
}

func TestMockSessionPaid(t *testing.T) {
	mock := NewMockStripeClient()

	// Starts out empty
	_, ok := mock.GetWebhookEvent(nil, nil)
	if ok {
		t.Fatal("expected no event")
	}

	// Create a customer
	customerId, _ := mock.GetNewStripeCustomer()

	// Create a session for that customer to pay for solo
	userId := int64(9)
	soloPriceId := mock.GetLatestSoloPriceId()
	clientReferenceId, sessionId, _, _ := mock.GetNewStripeSession(
		userId, customerId, soloPriceId, 1 /*forceCurrency*/, "")

	// Verify session starts as non paid
	mock.CheckSessionIsUnpaid(sessionId, t)

	// Mock that the user paid for the team plan with quantity=5
	teamPriceId := mock.GetLatestTeamPriceId()
	mockSubscriptionId := mock.MockSessionPaid(sessionId, teamPriceId, 5)

	// Check session status
	mock.CheckSessionIsPaid(sessionId, t)

	// Calling the webhook method should return the evenf of the session paid
	expectedSession := stripe.CheckoutSession{
		ID:                sessionId,
		ClientReferenceID: clientReferenceId,
		Subscription: &stripe.Subscription{
			ID: mockSubscriptionId,
		},
	}
	rawExpectedSessiom, _ := json.Marshal(expectedSession)
	expectedEvent := stripe.Event{
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: rawExpectedSessiom,
		},
	}
	event, ok := mock.GetWebhookEvent(nil, nil)
	if !ok {
		t.Fatal("expected event")
	}
	if !reflect.DeepEqual(event, expectedEvent) {
		t.Fatalf("expected event %v got %v", expectedEvent, event)
	}

	expectedLineItems := stripe.LineItemList{
		Data: []*stripe.LineItem{
			{
				Quantity: 5,
				Price: &stripe.Price{
					ID: string(teamPriceId),
				},
			},
		},
	}
	lineItems, err := mock.GetSessionLineItems(sessionId)
	if err != nil {
		t.Fatal(err)
	}
	if len(expectedLineItems.Data) != len(lineItems.Data) {
		t.Fatal("unexpected data len")
	}
	if expectedLineItems.Data[0].Quantity != lineItems.Data[0].Quantity {
		t.Fatal("unexpected qty")
	}
	if expectedLineItems.Data[0].Price.ID != lineItems.Data[0].Price.ID {
		t.Fatal("unexpected price")
	}

	// Next call should return err bc there are no more webhook events
	_, ok = mock.GetWebhookEvent(nil, nil)
	if ok {
		t.Fatal("expected no event")
	}

}

func TestMockSessionCacneled(t *testing.T) {
	mock := NewMockStripeClient()

	// Starts out empty
	_, ok := mock.GetWebhookEvent(nil, nil)
	if ok {
		t.Fatal("expected no event")
	}

	mock.MockSubscriptionCanceled("sub123", "stripeUserId123")
	ev, ok := mock.GetWebhookEvent(nil, nil)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.Type != "customer.subscription.deleted" {
		t.Fatalf("got type %s", ev.Type)
	}
	var sub stripe.Subscription
	err := json.Unmarshal(ev.Data.Raw, &sub)
	if err != nil {
		t.Fatal(err)
	}
	if sub.ID != "sub123" {
		t.Fatalf("unexpected subId: %s", sub.ID)
	}
	if sub.Customer.ID != "stripeUserId123" {
		t.Fatalf("unexpected customer id: %s", sub.Customer.ID)
	}

	// Next call should return err bc there are no more webhook events
	_, ok = mock.GetWebhookEvent(nil, nil)
	if ok {
		t.Fatal("expected no event")
	}

}

func TestMockSubscriptionUpdatedQuantity(t *testing.T) {
	mock := NewMockStripeClient()

	// Starts out empty
	_, ok := mock.GetWebhookEvent(nil, nil)
	if ok {
		t.Fatal("expected no event")
	}

	mock.MockSubscriptionUpdatedQuantity("sub123", "stripeUserId123", 100)
	ev, ok := mock.GetWebhookEvent(nil, nil)
	if !ok {
		t.Fatal("expected event")
	}
	if ev.Type != "customer.subscription.updated" {
		t.Fatalf("got type %s", ev.Type)
	}
	var sub stripe.Subscription
	err := json.Unmarshal(ev.Data.Raw, &sub)
	if err != nil {
		t.Fatal(err)
	}
	if sub.ID != "sub123" {
		t.Fatalf("unexpected subId: %s", sub.ID)
	}
	if sub.Customer.ID != "stripeUserId123" {
		t.Fatalf("unexpected customer id: %s", sub.Customer.ID)
	}
	if sub.Items.Data[0].Quantity != 100 {
		t.Fatalf("unexpected quantity: %v", sub.Items.Data[0].Quantity)
	}

	// Next call should return err bc there are no more webhook events
	_, ok = mock.GetWebhookEvent(nil, nil)
	if ok {
		t.Fatal("expected no event")
	}

}
