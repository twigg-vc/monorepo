package testpage

import (
	"embed"
	"monorepo/twigg-track/wrappers"
	"net/http"
)

// Registers a handler that returns an html page so that the browser acts like
// a client of the track server. The browser can put jobs and view their outputs
func AddHandlers(serverUrl string, authMux wrappers.AuthMux) {
	authMux.HandleFunc("GET /test", func(w http.ResponseWriter, r wrappers.AuthMuxRequest) {
		http.ServeFileFS(w, r.Request, testPageFS, "test.html")
	})
}

//go:embed test.html
var testPageFS embed.FS
