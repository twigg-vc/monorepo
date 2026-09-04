package docusaurus

import (
	"embed"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strings"
)

//go:embed build
var buildFS embed.FS

const indexFilePath = "build/index.html"

func AddDocsHandler(configName string, mux wrappers.RlMux) {
	h := handler{}
	mux.HandleFunc("GET "+routes.DocumentationPage2, h.handleGetIndex)
	mux.HandleFunc("GET "+routes.DocumentationPage2+"/", h.handleGetFile)
	mux.HandleFunc("HEAD "+routes.DocumentationPage2+"/", h.handleGetFile)

	// Redirect /docs
	mux.HandleFunc("GET "+routes.DocumentationPage, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, routes.DocumentationPage2, http.StatusSeeOther)
	})
	// Redirect /blog
	mux.HandleFunc("GET "+routes.BlogPage, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, routes.DocumentationPage2+"/blog", http.StatusSeeOther)
	})
}

type handler struct {
}

func (h handler) handleGetFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len(routes.DocumentationPage2):]

	filePath := indexFilePath
	if path != "" && path != "/" {
		if strings.HasSuffix(path, "/") {
			filePath = "build" + path + "index.html"
		} else {
			filePath = "build" + path
		}
	}

	setCacheHeaders(w, filePath)
	http.ServeFileFS(w, r, buildFS, filePath)
}
func (h handler) handleGetIndex(w http.ResponseWriter, r *http.Request) {
	setCacheHeaders(w, indexFilePath)
	http.ServeFileFS(w, r, buildFS, indexFilePath)
}
