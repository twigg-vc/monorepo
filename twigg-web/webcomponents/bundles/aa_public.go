package bundles

import (
	"monorepo/twigg-web/wrappers"

	"monorepo/maragudk/gomponents"
)

// Returns the header that must be included in the HTML to import wc-bundle.js
func Header() gomponents.Node {
	return header()
}

// Adds a handler that handles GET requests to return wc-bundle.js
func AddHandler(mux wrappers.RlMux) {
	addHandler(mux)
}
