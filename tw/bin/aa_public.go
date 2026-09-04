package bin

import (
	"net/http"
)

type Mux interface {
	HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request))
}

// Register handler under /bin/{filename}
func AddHandlers(mux Mux) {
	addHandler(mux)
}