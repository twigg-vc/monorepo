package plans

import (
	"context"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/stripeclient"
	"monorepo/twigg-web/user"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(
	mux wrappers.UserMux,
	stripeClient StripeClient,
	userService UserService,
) {
	h := handler{stripeClient, userService}
	mux.HandleFuncR("GET "+routes.PlansPage, h.handleGet)
}

type StripeClient interface {
	GetLatestTeamPriceId() stripeclient.PriceId
	GetLatestSoloPriceId() stripeclient.PriceId
}

type UserService interface {
	GetByUsername(r context.Context, username string) (u user.User, isNotFoundErr bool, err error)
}
