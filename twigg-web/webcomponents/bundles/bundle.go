package bundles

import (
	"embed"
	"fmt"
	"monorepo/buildmeta"
	"monorepo/twigg-web/cacheheaders"
	"monorepo/twigg-web/wrappers"
	"net/http"

	"monorepo/maragudk/gomponents"
	"monorepo/maragudk/gomponents/html"
)

func header() gomponents.Node {
	return gomponents.Group{
		html.Script(
			html.Src(fmt.Sprintf("/wc-bundle-%s.js", buildmeta.Version)),
			html.Defer(),
		),
		html.Link(
			html.Rel("stylesheet"),
			html.Href(fmt.Sprintf("/wc-bundle-%s.css", buildmeta.Version)),
		),
	}
}

func addHandler(mux wrappers.RlMux) {
	mux.HandleFunc(fmt.Sprintf("GET /wc-bundle-%s.js", buildmeta.Version), handleGet)
	mux.HandleFunc(fmt.Sprintf("GET /wc-bundle-%s.css", buildmeta.Version), handleGetCss)
}

//go:embed files
var folder embed.FS

func handleGet(w http.ResponseWriter, r *http.Request) {
	cacheheaders.LongCache(w)
	http.ServeFileFS(
		w,
		r,
		folder,
		"files/bundle.js")
}

func handleGetCss(w http.ResponseWriter, r *http.Request) {
	cacheheaders.LongCache(w)
	http.ServeFileFS(
		w,
		r,
		folder,
		"files/index.css")
}
