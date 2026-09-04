package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"monorepo/twigg-web/permissions"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stripe/stripe-go/v82"
)

func TestCancelTrialIfNeededAndCreateStripeSession(t *testing.T) {
	const stripeSessionUrl = "https://stripe.example.com/checkout/session"

	testCases := []struct {
		desc                            string
		query                           string
		user                            user.User
		mockGetOrg                      user.User
		mockGetOrgNotFound              bool
		mockGetOrgErr                   bool
		mockHandleSubDeletedErr         bool
		mockGetUserForPaymentErr        bool
		mockHasPermission               bool
		mockHasPermissionErr            bool
		expectedStatus                  int
		expectedShouldCommit            bool
		expectedBodyContains            string
		expectedRedirectLocation        string
		expectShouldDeleteTrial         bool
		expectedDeletedTrialUserId      int64
		expectShouldCreateStripeSession bool
		expectedStripeUserId            int64
		expectedStripePriceId           stripeclient.PriceId
		expectedStripeQuantity          int64
		expectedStripeForceCurrency     string
		expectPermissionChecked         bool
		expectedPermAssetId             string
	}{
		{
			desc: "current user creates solo checkout session",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"1"},
			}.Encode(),
			user:                            user.User{Id: 1},
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            1,
			expectedStripePriceId:           soloLatestPriceId,
			expectedStripeQuantity:          1,
		},
		{
			desc: "current user creates team checkout session with quantity",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(teamLatestPriceId)},
				routes.StripeQuantityParamName: []string{"5"},
			}.Encode(),
			user:                            user.User{Id: 1},
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            1,
			expectedStripePriceId:           teamLatestPriceId,
			expectedStripeQuantity:          5,
		},
		{
			desc: "org creates team checkout session",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
				routes.StripePriceIdParamName:        []string{string(teamLatestPriceId)},
				routes.StripeQuantityParamName:       []string{"3"},
			}.Encode(),
			user:                            user.User{Id: 1},
			mockGetOrg:                      user.User{Id: 999, Username: "acme", IsOrganization: true},
			mockHasPermission:               true,
			expectPermissionChecked:         true,
			expectedPermAssetId:             permissions.OrganizationAssetId(999),
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            999,
			expectedStripePriceId:           teamLatestPriceId,
			expectedStripeQuantity:          3,
		},
		{
			desc: "org user lacks owner permission",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
				routes.StripePriceIdParamName:        []string{string(teamLatestPriceId)},
				routes.StripeQuantityParamName:       []string{"3"},
			}.Encode(),
			user:                    user.User{Id: 1},
			mockGetOrg:              user.User{Id: 999, Username: "acme", IsOrganization: true},
			mockHasPermission:       false,
			expectPermissionChecked: true,
			expectedPermAssetId:     permissions.OrganizationAssetId(999),
			expectedStatus:          http.StatusForbidden,
			expectedBodyContains:    "not allowed",
		},
		{
			desc: "permission check errors",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
				routes.StripePriceIdParamName:        []string{string(teamLatestPriceId)},
				routes.StripeQuantityParamName:       []string{"3"},
			}.Encode(),
			user:                    user.User{Id: 1},
			mockGetOrg:              user.User{Id: 999, Username: "acme", IsOrganization: true},
			mockHasPermissionErr:    true,
			expectPermissionChecked: true,
			expectedPermAssetId:     permissions.OrganizationAssetId(999),
			expectedStatus:          http.StatusInternalServerError,
			expectedBodyContains:    "internal error",
		},
		{
			desc: "user on trial cancels trial then creates session",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"1"},
			}.Encode(),
			user:                            user.User{Id: 1, SelfPaidSubscription: user.Subscription_Trial},
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldDeleteTrial:         true,
			expectedDeletedTrialUserId:      1,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            1,
			expectedStripePriceId:           soloLatestPriceId,
			expectedStripeQuantity:          1,
		},
		{
			desc: "org on trial cancels trial then creates session",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
				routes.StripePriceIdParamName:        []string{string(teamLatestPriceId)},
				routes.StripeQuantityParamName:       []string{"2"},
			}.Encode(),
			user:                            user.User{Id: 1},
			mockGetOrg:                      user.User{Id: 999, Username: "acme", IsOrganization: true, SelfPaidSubscription: user.Subscription_Trial},
			mockHasPermission:               true,
			expectPermissionChecked:         true,
			expectedPermAssetId:             permissions.OrganizationAssetId(999),
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldDeleteTrial:         true,
			expectedDeletedTrialUserId:      999,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            999,
			expectedStripePriceId:           teamLatestPriceId,
			expectedStripeQuantity:          2,
		},
		{
			desc: "force currency usd",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"1"},
				"usd":                          []string{"1"},
			}.Encode(),
			user:                            user.User{Id: 1},
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            1,
			expectedStripePriceId:           soloLatestPriceId,
			expectedStripeQuantity:          1,
			expectedStripeForceCurrency:     "usd",
		},
		{
			desc: "force currency brl",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"1"},
				"brl":                          []string{"1"},
			}.Encode(),
			user:                            user.User{Id: 1},
			expectedStatus:                  http.StatusSeeOther,
			expectedShouldCommit:            true,
			expectedRedirectLocation:        stripeSessionUrl,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            1,
			expectedStripePriceId:           soloLatestPriceId,
			expectedStripeQuantity:          1,
			expectedStripeForceCurrency:     "brl",
		},
		{
			desc: "invalid isChoosingPlanForOrg param",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"boom"},
			}.Encode(),
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "invalid param",
		},
		{
			desc: "org not found",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"missing-org"},
			}.Encode(),
			mockGetOrgNotFound:   true,
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "invalid param",
		},
		{
			desc: "get org error",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
			}.Encode(),
			mockGetOrgErr:        true,
			expectedStatus:       http.StatusInternalServerError,
			expectedBodyContains: "internal error",
		},
		{
			desc:                       "trial deletion error",
			user:                       user.User{Id: 1, SelfPaidSubscription: user.Subscription_Trial},
			mockHandleSubDeletedErr:    true,
			expectShouldDeleteTrial:    true,
			expectedDeletedTrialUserId: 1,
			expectedStatus:             http.StatusInternalServerError,
			expectedBodyContains:       "internal err deleting trial",
		},
		{
			desc:                 "missing price id",
			user:                 user.User{Id: 1},
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "bad request",
		},
		{
			desc: "unresolvable price id",
			query: url.Values{
				routes.StripePriceIdParamName: []string{"unknown_price"},
			}.Encode(),
			user:                 user.User{Id: 1},
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "bad request",
		},
		{
			desc: "invalid quantity",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"not_a_number"},
			}.Encode(),
			user:                 user.User{Id: 1},
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "bad request, invalid quantity",
		},
		{
			desc: "solo with quantity != 1",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"2"},
			}.Encode(),
			user:                 user.User{Id: 1},
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "bad request, invalid quantity for solo",
		},
		{
			desc: "get user for payment with stripe error",
			query: url.Values{
				routes.StripePriceIdParamName:  []string{string(soloLatestPriceId)},
				routes.StripeQuantityParamName: []string{"1"},
			}.Encode(),
			user:                            user.User{Id: 1},
			mockGetUserForPaymentErr:        true,
			expectShouldCreateStripeSession: true,
			expectedStripeUserId:            1,
			expectedStripePriceId:           soloLatestPriceId,
			expectedStripeQuantity:          1,
			expectedStatus:                  http.StatusBadRequest,
			expectedBodyContains:            "bad request",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			deleteTrialCalled := false
			createStripeSessionCalled := false
			permissionCalled := false

			mockedUserSrv := mockUserService{
				getByUsername: func(username string) (user.User, bool, error) {
					if tC.mockGetOrgErr {
						return user.User{}, false, errors.New("boom")
					}
					if tC.mockGetOrgNotFound {
						return user.User{}, true, nil
					}
					return tC.mockGetOrg, false, nil
				},
				handleManualSubscriptionDeleted: func(userId int64) error {
					deleteTrialCalled = true
					if userId != tC.expectedDeletedTrialUserId {
						t.Fatalf("unexpected userId for sub deletion: got %d want %d", userId, tC.expectedDeletedTrialUserId)
					}
					if tC.mockHandleSubDeletedErr {
						return errors.New("boom")
					}
					return nil
				},
				getUserForPaymentWithStripe: func(userId int64, priceId stripeclient.PriceId, qty int64, forceCurrency string) (user.User, bool, error) {
					createStripeSessionCalled = true
					if userId != tC.expectedStripeUserId {
						t.Fatalf("unexpected stripe userId: got %d want %d", userId, tC.expectedStripeUserId)
					}
					if priceId != tC.expectedStripePriceId {
						t.Fatalf("unexpected stripe priceId: got %v want %v", priceId, tC.expectedStripePriceId)
					}
					if qty != tC.expectedStripeQuantity {
						t.Fatalf("unexpected stripe quantity: got %d want %d", qty, tC.expectedStripeQuantity)
					}
					if forceCurrency != tC.expectedStripeForceCurrency {
						t.Fatalf("unexpected forceCurrency: got %q want %q", forceCurrency, tC.expectedStripeForceCurrency)
					}
					if tC.mockGetUserForPaymentErr {
						return user.User{}, false, errors.New("boom")
					}
					return user.User{StripeSessionUrl: stripeSessionUrl}, false, nil
				},
			}
			mockPermSrv := mockPermissionsService{
				hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
					permissionCalled = true
					if userId != tC.user.Id {
						t.Fatalf("unexpected userId in perm check: got %d want %d", userId, tC.user.Id)
					}
					if p != permissions.Permission_OrganizationOwner {
						t.Fatalf("unexpected permission: got %v want %v", p, permissions.Permission_OrganizationOwner)
					}
					if assetId != tC.expectedPermAssetId {
						t.Fatalf("unexpected assetId: got %q want %q", assetId, tC.expectedPermAssetId)
					}
					if tC.mockHasPermissionErr {
						return false, errors.New("boom")
					}
					return tC.mockHasPermission, nil
				},
			}

			h := handler{userS: mockedUserSrv, stripeClient: mockStripeClient{}, db: mockPermSrv}
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/cancel-trial-and-create-stripe-session?"+tC.query, nil)

			shouldCommit := h.cancelTrialIfNeededAndCreateStripeSession(
				rr,
				wrappers.UserMuxRequest{
					Request: req,
					User:    tC.user,
				},
				nil,
			)

			if shouldCommit != tC.expectedShouldCommit {
				t.Fatalf("expected shouldCommit=%v, got %v", tC.expectedShouldCommit, shouldCommit)
			}
			if rr.Code != tC.expectedStatus {
				t.Fatalf("expected status %d, got %d", tC.expectedStatus, rr.Code)
			}
			if tC.expectedBodyContains != "" && !strings.Contains(rr.Body.String(), tC.expectedBodyContains) {
				t.Fatalf("expected body to contain %q, got %q", tC.expectedBodyContains, rr.Body.String())
			}
			if tC.expectedRedirectLocation != "" {
				if location := rr.Header().Get("Location"); location != tC.expectedRedirectLocation {
					t.Fatalf("expected redirect location %q, got %q", tC.expectedRedirectLocation, location)
				}
			}
			if deleteTrialCalled != tC.expectShouldDeleteTrial {
				t.Fatalf("expected deleteTrial called=%v, got %v", tC.expectShouldDeleteTrial, deleteTrialCalled)
			}
			if createStripeSessionCalled != tC.expectShouldCreateStripeSession {
				t.Fatalf("expected createStripeSession called=%v, got %v", tC.expectShouldCreateStripeSession, createStripeSessionCalled)
			}
			if permissionCalled != tC.expectPermissionChecked {
				t.Fatalf("expected permissionChecked=%v, got %v", tC.expectPermissionChecked, permissionCalled)
			}
		})
	}
}

func TestHandleSubscribeTrial(t *testing.T) {
	testCases := []struct {
		desc                     string
		query                    string
		user                     user.User
		mockGetOrg               user.User
		mockGetOrgNotFound       bool
		mockGetOrgErr            bool
		mockHandlePaymentErr     bool
		mockHasPermission        bool
		mockHasPermissionErr     bool
		expectedStatus           int
		expectedShouldCommit     bool
		expectedBodyContains     string
		expectedRedirectLocation string
		expectedSubscribedUserId int64
		expectPermissionChecked  bool
		expectedPermAssetId      string
	}{
		{
			desc:                     "subscribe current user to trial",
			user:                     user.User{Id: 1},
			expectedStatus:           http.StatusSeeOther,
			expectedShouldCommit:     true,
			expectedRedirectLocation: routes.Home,
			expectedSubscribedUserId: 1,
		},
		{
			desc: "subscribe org to trial",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
			}.Encode(),
			user:                     user.User{Id: 1},
			mockGetOrg:               user.User{Id: 999, Username: "acme", IsOrganization: true},
			mockHasPermission:        true,
			expectPermissionChecked:  true,
			expectedPermAssetId:      permissions.OrganizationAssetId(999),
			expectedStatus:           http.StatusSeeOther,
			expectedShouldCommit:     true,
			expectedRedirectLocation: routes.PathToOrganization("acme"),
			expectedSubscribedUserId: 999,
		},
		{
			desc: "user lacks owner permission on org",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
			}.Encode(),
			user:                    user.User{Id: 1},
			mockGetOrg:              user.User{Id: 999, Username: "acme", IsOrganization: true},
			mockHasPermission:       false,
			expectPermissionChecked: true,
			expectedPermAssetId:     permissions.OrganizationAssetId(999),
			expectedStatus:          http.StatusForbidden,
			expectedBodyContains:    "not allowed",
		},
		{
			desc: "permission check errors",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
			}.Encode(),
			user:                    user.User{Id: 1},
			mockGetOrg:              user.User{Id: 999, Username: "acme", IsOrganization: true},
			mockHasPermissionErr:    true,
			expectPermissionChecked: true,
			expectedPermAssetId:     permissions.OrganizationAssetId(999),
			expectedStatus:          http.StatusInternalServerError,
			expectedBodyContains:    "internal error",
		},
		{
			desc: "invalid isChoosingPlanForOrg param",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"boom"},
			}.Encode(),
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "invalid param",
		},
		{
			desc: "org not found",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"missing-org"},
			}.Encode(),
			mockGetOrgNotFound:   true,
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "invalid param",
		},
		{
			desc: "get org error",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: []string{"true"},
				routes.OrganizationNameParamName:     []string{"acme"},
			}.Encode(),
			mockGetOrgErr:        true,
			expectedStatus:       http.StatusInternalServerError,
			expectedBodyContains: "internal error",
		},
		{
			desc:                 "user already subscribed",
			user:                 user.User{Id: 1, SelfPaidSubscription: user.Subscription_Solo},
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "already on sub",
		},
		{
			desc:                     "handle payment error",
			user:                     user.User{Id: 1},
			mockHandlePaymentErr:     true,
			expectedSubscribedUserId: 1,
			expectedStatus:           http.StatusInternalServerError,
			expectedBodyContains:     "server err paying sub",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			mockedUserSrv := mockUserService{
				handleManualPaymentSuccess: func(userId int64, sub user.SubscriptionPlan, qty int64) (user.User, error) {
					if userId != tC.expectedSubscribedUserId {
						t.Fatalf("unexpected userID: got %d want %d", userId, tC.expectedSubscribedUserId)
					}
					if sub != user.Subscription_Trial {
						t.Fatalf("unexpected subscription: got %v want %v", sub, user.Subscription_Trial)
					}
					if qty != 1 {
						t.Fatalf("unexpected qty: got %d want 1", qty)
					}
					if tC.mockHandlePaymentErr {
						return user.User{}, errors.New("boom")
					}
					return user.User{}, nil
				},
				getByUsername: func(username string) (user.User, bool, error) {
					if tC.mockGetOrgErr {
						return user.User{}, false, errors.New("boom")
					}
					if tC.mockGetOrgNotFound {
						return user.User{}, true, nil
					}
					return tC.mockGetOrg, false, nil
				},
			}

			permissionCalled := false
			mockPermSrv := mockPermissionsService{
				hasPermission: func(userId int64, p permissions.Permission, assetId string) (bool, error) {
					permissionCalled = true
					if userId != tC.user.Id {
						t.Fatalf("unexpected userId in perm check: got %d want %d", userId, tC.user.Id)
					}
					if p != permissions.Permission_OrganizationOwner {
						t.Fatalf("unexpected permission: got %v want %v", p, permissions.Permission_OrganizationOwner)
					}
					if assetId != tC.expectedPermAssetId {
						t.Fatalf("unexpected assetId: got %q want %q", assetId, tC.expectedPermAssetId)
					}
					if tC.mockHasPermissionErr {
						return false, errors.New("boom")
					}
					return tC.mockHasPermission, nil
				},
			}

			h := handler{userS: mockedUserSrv, db: mockPermSrv}
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/subscribe-trial?"+tC.query, nil)

			shouldCommit := h.handleSubscribeTrial(
				rr,
				wrappers.UserMuxRequest{
					Request: req,
					User:    tC.user,
				},
				nil,
			)

			if shouldCommit != tC.expectedShouldCommit {
				t.Fatalf("expected shouldCommit=%v, got %v", tC.expectedShouldCommit, shouldCommit)
			}
			if rr.Code != tC.expectedStatus {
				t.Fatalf("expected status %d, got %d", tC.expectedStatus, rr.Code)
			}
			if tC.expectedBodyContains != "" && !strings.Contains(rr.Body.String(), tC.expectedBodyContains) {
				t.Fatalf("expected body to contain %q, got %q", tC.expectedBodyContains, rr.Body.String())
			}
			if tC.expectedRedirectLocation != "" {
				if location := rr.Header().Get("Location"); location != tC.expectedRedirectLocation {
					t.Fatalf("expected redirect location %q, got %q", tC.expectedRedirectLocation, location)
				}
			}
			if permissionCalled != tC.expectPermissionChecked {
				t.Fatalf("expected permissionChecked=%v, got %v", tC.expectPermissionChecked, permissionCalled)
			}
		})
	}
}

func TestHandleSubscriptionDeleted(t *testing.T) {
	const (
		custId = "cus_456"
		subId  = "sub_123"
	)
	validEvent := makeStripeEvent(`{"id":"` + subId + `","customer":"` + custId + `"}`)

	testCases := []struct {
		desc                         string
		event                        stripe.Event
		deletedUser                  user.User
		subDeletedErr                bool
		enforceErr                   bool
		expectedStatus               int
		expectedBodyContains         string
		expectSubDeletedCalled       bool
		expectEnforceSeatLimitCalled bool
	}{
		{
			desc:                 "invalid json",
			event:                makeStripeEvent("invalid"),
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "failed to parse",
		},
		{
			desc:                   "HandlesSubscriptionDeleted error",
			event:                  validEvent,
			subDeletedErr:          true,
			expectSubDeletedCalled: true,
			expectedStatus:         http.StatusInternalServerError,
			expectedBodyContains:   "internal error",
		},
		{
			desc:                   "regular user subscription deleted",
			event:                  validEvent,
			deletedUser:            user.User{Id: 1},
			expectSubDeletedCalled: true,
			expectedStatus:         http.StatusOK,
		},
		{
			desc:                         "org subscription deleted enforces seat limit with 1 seat",
			event:                        validEvent,
			deletedUser:                  user.User{Id: 99, IsOrganization: true},
			expectSubDeletedCalled:       true,
			expectEnforceSeatLimitCalled: true,
			expectedStatus:               http.StatusOK,
		},
		{
			desc:                         "org EnforceOrgSeatLimit error",
			event:                        validEvent,
			deletedUser:                  user.User{Id: 99, IsOrganization: true},
			enforceErr:                   true,
			expectSubDeletedCalled:       true,
			expectEnforceSeatLimitCalled: true,
			expectedStatus:               http.StatusInternalServerError,
			expectedBodyContains:         "internal error enforcing seats",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var subDeletedCalled = false
			var enforceCalled = false
			var enforceCalledOrgAssetId string
			var enforceCalledSeats int64

			db := mockDb{}

			mockedUserSrv := mockUserService{
				handlesSubscriptionDeleted: func(stripeId, subscriptionId string) (user.User, error) {
					subDeletedCalled = true
					if stripeId != custId {
						t.Errorf("unexpected customer id: got %q want %q", stripeId, custId)
					}
					if subscriptionId != subId {
						t.Errorf("unexpected subscription id: got %q want %q", subscriptionId, subId)
					}
					if tC.subDeletedErr {
						return user.User{}, errors.New("boom")
					}
					return tC.deletedUser, nil
				},
			}
			mockedOrgHelper := mockOrgHelper{
				enforceOrgSeatLimit: func(orgAssetId string, seats int64) error {
					enforceCalled = true
					enforceCalledOrgAssetId = orgAssetId
					enforceCalledSeats = seats
					if tC.enforceErr {
						return errors.New("boom")
					}
					return nil
				},
			}

			h := handler{db: db, userS: mockedUserSrv, orgHelper: mockedOrgHelper}
			rr := httptest.NewRecorder()
			h.handleSubscriptionDeleted(rr, tC.event)

			if rr.Code != tC.expectedStatus {
				t.Fatalf("expected status %d, got %d", tC.expectedStatus, rr.Code)
			}
			if tC.expectedBodyContains != "" && !strings.Contains(rr.Body.String(), tC.expectedBodyContains) {
				t.Fatalf("expected body to contain %q, got %q", tC.expectedBodyContains, rr.Body.String())
			}
			if subDeletedCalled != tC.expectSubDeletedCalled {
				t.Fatalf("HandlesSubscriptionDeleted called=%v, expected %v", subDeletedCalled, tC.expectSubDeletedCalled)
			}
			if enforceCalled != tC.expectEnforceSeatLimitCalled {
				t.Fatalf("EnforceOrgSeatLimit called=%v, expected %v", enforceCalled, tC.expectEnforceSeatLimitCalled)
			}
			if tC.expectEnforceSeatLimitCalled {
				wantAssetId := permissions.OrganizationAssetId(tC.deletedUser.Id)
				if enforceCalledOrgAssetId != wantAssetId {
					t.Fatalf("EnforceOrgSeatLimit orgAssetId=%q, want %q", enforceCalledOrgAssetId, wantAssetId)
				}
				if enforceCalledSeats != 1 {
					t.Fatalf("EnforceOrgSeatLimit numberOfSeats=%d, want 1", enforceCalledSeats)
				}
			}
		})
	}
}

func TestHandleCheckoutCompleted(t *testing.T) {
	const (
		sessionId     = "cs_test_123"
		clientRef     = "99"
		checkoutSubId = "sub_456"
		lineItemQty   = int64(3)
	)

	validEvent := makeCheckoutSessionEvent(sessionId, clientRef, checkoutSubId)

	testCases := []struct {
		desc                         string
		event                        stripe.Event
		checkoutUser                 user.User
		checkoutErr                  bool
		lineItemsErr                 bool
		enforceErr                   bool
		expectedStatus               int
		expectedBodyContains         string
		expectCheckoutSuccessCalled  bool
		expectEnforceSeatLimitCalled bool
	}{
		{
			desc:                 "invalid json",
			event:                makeStripeEvent("invalid"),
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "failed to parse",
		},
		{
			desc:                 "GetSessionLineItems error",
			event:                validEvent,
			lineItemsErr:         true,
			expectedStatus:       http.StatusInternalServerError,
			expectedBodyContains: "unable to fetch checkout session details",
		},
		{
			desc:                        "HandleStripeCheckoutSessionSuccess error",
			event:                       validEvent,
			checkoutErr:                 true,
			expectCheckoutSuccessCalled: true,
			expectedStatus:              http.StatusInternalServerError,
			expectedBodyContains:        "internal error",
		},
		{
			desc:                        "regular user checkout success",
			event:                       validEvent,
			checkoutUser:                user.User{Id: 99},
			expectCheckoutSuccessCalled: true,
			expectedStatus:              http.StatusOK,
		},
		{
			desc:                         "org checkout success enforces seat limit with checkout quantity",
			event:                        validEvent,
			checkoutUser:                 user.User{Id: 99, IsOrganization: true},
			expectCheckoutSuccessCalled:  true,
			expectEnforceSeatLimitCalled: true,
			expectedStatus:               http.StatusOK,
		},
		{
			desc:                         "org EnforceOrgSeatLimit error",
			event:                        validEvent,
			checkoutUser:                 user.User{Id: 99, IsOrganization: true},
			enforceErr:                   true,
			expectCheckoutSuccessCalled:  true,
			expectEnforceSeatLimitCalled: true,
			expectedStatus:               http.StatusInternalServerError,
			expectedBodyContains:         "internal error enforcing seats",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var checkoutSuccessCalled = false
			var enforceCalled = false
			var enforceCalledSeats int64

			db := mockDb{}

			mockedStripeClient := mockStripeClient{
				getSessionLineItems: func(id string) (stripe.LineItemList, error) {
					if tC.lineItemsErr {
						return stripe.LineItemList{}, errors.New("boom")
					}
					return stripe.LineItemList{
						Data: []*stripe.LineItem{{
							Price:    &stripe.Price{ID: string(teamLatestPriceId)},
							Quantity: lineItemQty,
						}},
					}, nil
				},
			}
			mockedUserSrv := mockUserService{
				handleStripeCheckoutSessionSuccess: func(userId int64, subId, sid string, pid stripeclient.PriceId, qty int64) (user.User, error) {
					checkoutSuccessCalled = true
					if tC.checkoutErr {
						return user.User{}, errors.New("boom")
					}
					return tC.checkoutUser, nil
				},
			}
			mockedOrgHelper := mockOrgHelper{
				enforceOrgSeatLimit: func(orgAssetId string, seats int64) error {
					enforceCalled = true
					enforceCalledSeats = seats
					if tC.enforceErr {
						return errors.New("boom")
					}
					return nil
				},
			}

			h := handler{db: db, userS: mockedUserSrv, stripeClient: mockedStripeClient, orgHelper: mockedOrgHelper}
			rr := httptest.NewRecorder()
			h.handleCheckoutCompleted(rr, tC.event)

			if rr.Code != tC.expectedStatus {
				t.Fatalf("expected status %d, got %d", tC.expectedStatus, rr.Code)
			}
			if tC.expectedBodyContains != "" && !strings.Contains(rr.Body.String(), tC.expectedBodyContains) {
				t.Fatalf("expected body to contain %q, got %q", tC.expectedBodyContains, rr.Body.String())
			}
			if checkoutSuccessCalled != tC.expectCheckoutSuccessCalled {
				t.Fatalf("HandleStripeCheckoutSessionSuccess called=%v, expected %v", checkoutSuccessCalled, tC.expectCheckoutSuccessCalled)
			}
			if enforceCalled != tC.expectEnforceSeatLimitCalled {
				t.Fatalf("EnforceOrgSeatLimit called=%v, expected %v", enforceCalled, tC.expectEnforceSeatLimitCalled)
			}
			if tC.expectEnforceSeatLimitCalled {
				if enforceCalledSeats != lineItemQty {
					t.Fatalf("EnforceOrgSeatLimit numberOfSeats=%d, want %d", enforceCalledSeats, lineItemQty)
				}
			}
		})
	}
}

func TestHandleSubscriptionUpdated(t *testing.T) {
	const (
		subId  = "sub_123"
		custId = "cus_456"
		newQty = int64(5)
	)

	validEvent := makeSubscriptionUpdatedEvent(subId, custId, newQty)

	testCases := []struct {
		desc                         string
		event                        stripe.Event
		existingUserQty              int64
		updatedUser                  user.User
		updateQuantityErr            bool
		enforceErr                   bool
		expectedStatus               int
		expectedBodyContains         string
		expectUpdateQuantityCalled   bool
		expectEnforceSeatLimitCalled bool
	}{
		{
			desc:                 "invalid json",
			event:                makeStripeEvent("invalid"),
			expectedStatus:       http.StatusBadRequest,
			expectedBodyContains: "failed to parse",
		},
		{
			desc:            "quantity unchanged - no update",
			event:           validEvent,
			existingUserQty: newQty,
			expectedStatus:  http.StatusOK,
		},
		{
			desc:                       "HandleSubscriptionQuantityUpdated error",
			event:                      validEvent,
			existingUserQty:            newQty - 1,
			updateQuantityErr:          true,
			expectUpdateQuantityCalled: true,
			expectedStatus:             http.StatusInternalServerError,
			expectedBodyContains:       "internal error",
		},
		{
			desc:                       "regular user subscription quantity updated",
			event:                      validEvent,
			existingUserQty:            newQty - 1,
			updatedUser:                user.User{Id: 1},
			expectUpdateQuantityCalled: true,
			expectedStatus:             http.StatusOK,
		},
		{
			desc:                         "org subscription quantity updated enforces seat limit with new quantity",
			event:                        validEvent,
			existingUserQty:              newQty - 1,
			updatedUser:                  user.User{Id: 99, IsOrganization: true},
			expectUpdateQuantityCalled:   true,
			expectEnforceSeatLimitCalled: true,
			expectedStatus:               http.StatusOK,
		},
		{
			desc:                         "org EnforceOrgSeatLimit error",
			event:                        validEvent,
			existingUserQty:              newQty - 1,
			updatedUser:                  user.User{Id: 99, IsOrganization: true},
			enforceErr:                   true,
			expectUpdateQuantityCalled:   true,
			expectEnforceSeatLimitCalled: true,
			expectedStatus:               http.StatusInternalServerError,
			expectedBodyContains:         "internal error enforcing seats",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var updateQuantityCalled = false
			var enforceCalled = false
			var enforceCalledOrgAssetId string
			var enforceCalledSeats int64

			db := mockDb{}

			mockedUserSrv := mockUserService{
				getByStripeId: func(stripeId string) (user.User, bool, error) {
					return user.User{SelfPaidSubscriptionQuantity: tC.existingUserQty}, false, nil
				},
				handleSubscriptionQuantityUpdated: func(stripeId, sId string, qty int64) (user.User, error) {
					updateQuantityCalled = true
					if stripeId != custId {
						t.Errorf("unexpected customer id: got %q want %q", stripeId, custId)
					}
					if sId != subId {
						t.Errorf("unexpected subscription id: got %q want %q", sId, subId)
					}
					if qty != newQty {
						t.Errorf("unexpected quantity: got %d want %d", qty, newQty)
					}
					if tC.updateQuantityErr {
						return user.User{}, errors.New("boom")
					}
					return tC.updatedUser, nil
				},
			}
			mockedOrgHelper := mockOrgHelper{
				enforceOrgSeatLimit: func(orgAssetId string, seats int64) error {
					enforceCalled = true
					enforceCalledOrgAssetId = orgAssetId
					enforceCalledSeats = seats
					if tC.enforceErr {
						return errors.New("boom")
					}
					return nil
				},
			}

			h := handler{db: db, userS: mockedUserSrv, orgHelper: mockedOrgHelper}
			rr := httptest.NewRecorder()
			h.handleSubscriptionUpdated(rr, tC.event)

			if rr.Code != tC.expectedStatus {
				t.Fatalf("expected status %d, got %d", tC.expectedStatus, rr.Code)
			}
			if tC.expectedBodyContains != "" && !strings.Contains(rr.Body.String(), tC.expectedBodyContains) {
				t.Fatalf("expected body to contain %q, got %q", tC.expectedBodyContains, rr.Body.String())
			}
			if updateQuantityCalled != tC.expectUpdateQuantityCalled {
				t.Fatalf("HandleSubscriptionQuantityUpdated called=%v, expected %v", updateQuantityCalled, tC.expectUpdateQuantityCalled)
			}
			if enforceCalled != tC.expectEnforceSeatLimitCalled {
				t.Fatalf("EnforceOrgSeatLimit called=%v, expected %v", enforceCalled, tC.expectEnforceSeatLimitCalled)
			}
			if tC.expectEnforceSeatLimitCalled {
				wantAssetId := permissions.OrganizationAssetId(tC.updatedUser.Id)
				if enforceCalledOrgAssetId != wantAssetId {
					t.Fatalf("EnforceOrgSeatLimit orgAssetId=%q, want %q", enforceCalledOrgAssetId, wantAssetId)
				}
				if enforceCalledSeats != newQty {
					t.Fatalf("EnforceOrgSeatLimit numberOfSeats=%d, want %d", enforceCalledSeats, newQty)
				}
			}
		})
	}
}

type mockUserService struct {
	handleManualPaymentSuccess         func(userID int64, sub user.SubscriptionPlan, qty int64) (user.User, error)
	getByUsername                      func(username string) (u user.User, isNotFoundErr bool, err error)
	handleManualSubscriptionDeleted    func(userID int64) error
	getUserForPaymentWithStripe        func(userID int64, priceId stripeclient.PriceId, qty int64, forceCurrency string) (user.User, bool, error)
	handlesSubscriptionDeleted         func(stripeId, subscriptionId string) (user.User, error)
	handleStripeCheckoutSessionSuccess func(userId int64, subId, sessionId string, priceId stripeclient.PriceId, qty int64) (user.User, error)
	getByStripeId                      func(stripeId string) (user.User, bool, error)
	handleSubscriptionQuantityUpdated  func(stripeId, subId string, qty int64) (user.User, error)
}

func (m mockUserService) HandleManualPaymentSuccess(_ context.Context, userID int64, sub user.SubscriptionPlan, qty int64) (user.User, error) {
	return m.handleManualPaymentSuccess(userID, sub, qty)
}

func (m mockUserService) GetByUsername(_ context.Context, username string) (user.User, bool, error) {
	return m.getByUsername(username)
}

func (m mockUserService) HandleManualSubscriptionDeleted(_ context.Context, userID int64) error {
	return m.handleManualSubscriptionDeleted(userID)
}

func (m mockUserService) GetUserForPaymentWithStripe(_ context.Context, userID int64, priceId stripeclient.PriceId, qty int64, forceCurrency string) (user.User, bool, error) {
	return m.getUserForPaymentWithStripe(userID, priceId, qty, forceCurrency)
}

func (m mockUserService) HandleStripeCheckoutSessionSuccess(_ context.Context, userId int64, subId, sessionId string, priceId stripeclient.PriceId, qty int64) (user.User, error) {
	if m.handleStripeCheckoutSessionSuccess == nil {
		panic("not implemented")
	}
	return m.handleStripeCheckoutSessionSuccess(userId, subId, sessionId, priceId, qty)
}

func (m mockUserService) HandlesSubscriptionDeleted(_ context.Context, stripeId, subscriptionId string) (user.User, error) {
	return m.handlesSubscriptionDeleted(stripeId, subscriptionId)
}

func (m mockUserService) HandleSubscriptionQuantityUpdated(_ context.Context, stripeId, subId string, qty int64) (user.User, error) {
	return m.handleSubscriptionQuantityUpdated(stripeId, subId, qty)
}

func (m mockUserService) GetByStripeId(_ context.Context, stripeId string) (user.User, bool, error) {
	return m.getByStripeId(stripeId)
}

type mockStripeClient struct {
	getSessionLineItems func(id string) (stripe.LineItemList, error)
}

const teamLatestPriceId = stripeclient.PriceId("price_team_latest")
const soloLatestPriceId = stripeclient.PriceId("price_solo_latest")

func (m mockStripeClient) ResolvePriceId(priceId stripeclient.PriceId) (product stripeclient.Product, isOk bool) {
	if priceId == teamLatestPriceId {
		return stripeclient.Product_Subscription_Team, true
	}
	if priceId == soloLatestPriceId {
		return stripeclient.Product_Subscription_Solo, true
	}
	return stripeclient.Product_None, false
}

func (m mockStripeClient) GetLatestTeamPriceId() stripeclient.PriceId {
	return teamLatestPriceId
}

func (m mockStripeClient) GetLatestSoloPriceId() stripeclient.PriceId {
	return soloLatestPriceId
}

func (m mockStripeClient) GetNewCustomerPortalSession(int64, string) (string, error) {
	panic("not implemented")
}

func (m mockStripeClient) GetSessionLineItems(id string) (stripe.LineItemList, error) {
	return m.getSessionLineItems(id)
}

func (m mockStripeClient) GetWebhookEvent(http.ResponseWriter, *http.Request) (stripe.Event, bool) {
	panic("not implemented")
}

type mockPermissionsService struct {
	hasPermission func(userId int64, p permissions.Permission, assetId string) (bool, error)
}

func (m mockPermissionsService) HasPermission(rl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	return m.hasPermission(userId, p, assetId)
}

func (m mockPermissionsService) BeginWrite() (context.Context, func(), func() error, error) {
	panic("unexpected call to BeginWrite")
}

type mockOrgHelper struct {
	enforceOrgSeatLimit func(orgAssetId string, numberOfSeats int64) error
}

func (m mockOrgHelper) EnforceOrgSeatLimit(wl context.Context, orgAssetId string, numberOfSeats int64) error {
	return m.enforceOrgSeatLimit(orgAssetId, numberOfSeats)
}

type mockDb struct{}

func (m mockDb) BeginWrite() (context.Context, func(), func() error, error) {
	return context.Background(), func() {}, func() error { return nil }, nil
}

func (m mockDb) HasPermission(rl context.Context, userId int64, p permissions.Permission, assetId string) (bool, error) {
	panic("unexpected call to HasPermission")
}

func makeStripeEvent(raw string) stripe.Event {
	return stripe.Event{Data: &stripe.EventData{Raw: json.RawMessage(raw)}}
}

func makeCheckoutSessionEvent(sessionId, clientRefId, subId string) stripe.Event {
	raw := fmt.Sprintf(`{"id":%q,"client_reference_id":%q,"subscription":{"id":%q}}`, sessionId, clientRefId, subId)
	return makeStripeEvent(raw)
}

func makeSubscriptionUpdatedEvent(subId, custId string, quantity int64) stripe.Event {
	raw := fmt.Sprintf(`{"id":%q,"customer":%q,"items":{"data":[{"quantity":%d}]}}`, subId, custId, quantity)
	return makeStripeEvent(raw)
}
