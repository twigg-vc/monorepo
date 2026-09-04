package cacheheaders

import (
	"fmt"
	"net/http"
)

// Cache for 1h
func ShortCache(w http.ResponseWriter) {
	cacheForHours(w, 1)
}

// Cache for 6h
func MediumCache(w http.ResponseWriter) {
	cacheForHours(w, 6)
}

// Cache for 1d
func LongCache(w http.ResponseWriter) {
	cacheForHours(w, 24)
}

// Cache for 1month
func VeryLongCache(w http.ResponseWriter) {
	cacheForHours(w, 30*24)
}

// Don't cache at all
func NoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache")
}

func cacheForHours(w http.ResponseWriter, nHours int) {
	w.Header().Set("Cache-Control",
		fmt.Sprintf("public, max-age=%d", nHours*60*60))
}
