package wrappers

import (
	"crypto/subtle"
	"net/http"
)

type Mux interface {
	HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request))
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// Rate limitted mux that also logs requests
type RateLimittedMux struct {
	rl rlMux
}

func NewRateLimittedMux(maxQps float64, maxQpsBurst int, mux Mux) RateLimittedMux {
	return RateLimittedMux{rl: newRateLimitted(maxQps, maxQpsBurst, mux)}
}
func (m RateLimittedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.rl.ServeHTTP(w, r)
}
func (m RateLimittedMux) HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	m.rl.HandleFunc(pattern, handler)
}

type AuthMux struct {
	trackKey string
	m        Mux
}

func NewAuthMux(trackKey string, m Mux) AuthMux {
	return AuthMux{trackKey, m}
}

type AuthMuxRequest struct {
	*http.Request
}

const TrackKeyHeaderName = "TrackKey"

func (a AuthMux) HandleFunc(pattern string, handler func(w http.ResponseWriter, r AuthMuxRequest)) {
	a.m.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		headerMatches := subtle.ConstantTimeCompare(
			[]byte(r.Header.Get(TrackKeyHeaderName)), []byte(a.trackKey)) == 1
		queryMatches := subtle.ConstantTimeCompare(
			[]byte(r.URL.Query().Get(TrackKeyHeaderName)), []byte(a.trackKey)) == 1
		if !headerMatches && !queryMatches {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		handler(w, AuthMuxRequest{r})
	})
}