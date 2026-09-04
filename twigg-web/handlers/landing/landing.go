package landing

import (
	"embed"
	"io/fs"
	"monorepo/twigg-web/cacheheaders"
	"net/http"
)

//go:embed files
var files embed.FS

func newHandler() http.Handler {
	// Make "files/favicon.ico" addressable as "/favicon.ico"
	sub, err := fs.Sub(files, "files")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			cacheheaders.LongCache(w)
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		cacheheaders.VeryLongCache(w)
		fileServer.ServeHTTP(w, r)
	})
}