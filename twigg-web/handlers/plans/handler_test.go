package plans

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
)

func Test_handleGet(t *testing.T) {
	tests := []struct {
		name              string
		query             url.Values
		mockGetByUsername func(username string) (user.User, bool, error)
		wantStatus        int
	}{
		{
			name:       "default user flow",
			query:      url.Values{},
			wantStatus: http.StatusOK,
		},
		{
			name: "org flow success",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: {"true"},
				routes.OrganizationNameParamName:     {"my-org"},
			},
			mockGetByUsername: func(username string) (user.User, bool, error) {
				if username != "my-org" {
					t.Fatal("called Get with invalid username")
				}
				return user.User{Username: username}, false, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid bool param",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: {"invalid"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid org name param",
			query: url.Values{
				routes.IsChoosingPlanForOrgParamName: {"true"},
				routes.OrganizationNameParamName:     {""},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.query.Encode(), nil)
			rr := httptest.NewRecorder()

			h := handler{
				stripeClient: mockStripeClient{},
				userService: mockUserService{
					getByUsername: func(username string) (user.User, bool, error) {
						if tt.mockGetByUsername != nil {
							return tt.mockGetByUsername(username)
						}
						return user.User{}, false, nil
					},
				},
			}
			wrappedReq := wrappers.UserMuxRequest{
				Request: req,
				User: user.User{
					Id:       1,
					Username: "test-user",
				},
			}

			h.handleGet(rr, wrappedReq, nil)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

type mockStripeClient struct{}

func (m mockStripeClient) GetLatestSoloPriceId() stripeclient.PriceId { return "solo_price" }
func (m mockStripeClient) GetLatestTeamPriceId() stripeclient.PriceId { return "team_price" }

type mockUserService struct {
	getByUsername func(username string) (u user.User, isNotFoundErr bool, err error)
}

func (m mockUserService) GetByUsername(_ context.Context, username string) (u user.User, isNotFoundErr bool, err error) {
	return m.getByUsername(username)
}
