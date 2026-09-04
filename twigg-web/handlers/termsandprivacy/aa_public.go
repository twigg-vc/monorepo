package termsandprivacy

import (
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
)

func AddHandlers(mux wrappers.RlMux) {

	mux.HandleFunc("GET "+routes.TermsPage, handleGetTerms)
	mux.HandleFunc("GET "+routes.PrivacyPage, handleGetPrivacy)
}
