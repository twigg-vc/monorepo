package plans

import (
	"context"
	"fmt"
	"log"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/user"
	twiggwc "monorepo/twigg-web/webcomponents"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strconv"
)

type handler struct {
	stripeClient StripeClient
	userService  UserService
}

func (h handler) handleGet(w http.ResponseWriter, r wrappers.UserMuxRequest, dbRead context.Context) {
	isChoosingPlanForOrg, org, ok := h.parseIsChoosingPlanForOrg(w, r, dbRead)
	if !ok {
		return
	}

	twiggwc.Page( /*hideNavBar*/ false,
		r.Flags,
		twiggwc.PlansCards(
			r.User,
			string(h.stripeClient.GetLatestSoloPriceId()),
			string(h.stripeClient.GetLatestTeamPriceId()),
			isChoosingPlanForOrg,
			org,
		),
	).Render(w)
}

// if !ok error already written in ResponseWriter
func (h handler) parseIsChoosingPlanForOrg(
	w http.ResponseWriter,
	r wrappers.UserMuxRequest,
	dbRead context.Context,
) (isChoosingPlanForOrg bool, org user.User, ok bool) {

	isChoosingPlanForOrgStr := r.URL.Query().Get(routes.IsChoosingPlanForOrgParamName)
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
		organizationName := r.URL.Query().Get(routes.OrganizationNameParamName)
		if organizationName == "" {
			ok = false
			http.Error(w, fmt.Sprintf("invalid param:%q, got:%q", routes.OrganizationNameParamName, organizationName), http.StatusBadRequest)
			return
		}
		var err error
		org, _, err = h.userService.GetByUsername(dbRead, organizationName)
		if err != nil {
			ok = false
			log.Printf("failed to get org user: %s", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	ok = true
	return
}
