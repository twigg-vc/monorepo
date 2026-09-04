package bin

import (
	"embed"
	"net/http"
)

//go:embed tw_*
var binFiles embed.FS

func addHandler(mux Mux) {
	mux.HandleFunc("GET /bin/{filename}", handleGetLinux)
}

func handleGetLinux(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, binFiles, r.PathValue("filename"))
}
