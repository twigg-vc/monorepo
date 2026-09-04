package wrappers

import (
	"crypto/subtle"
	"log"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/twiggtoken"
	"net/http"
)

type serverKeyAuthTrackMux struct {
	twiggServerKey string
	mux            RlMux
}

func (m serverKeyAuthTrackMux) HandleFunc(
	pattern string, handler func(w http.ResponseWriter, r ServerKeyAuthTrackMuxRequest)) {
	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare(
			[]byte(r.Header.Get("TwiggServerKey")), []byte(m.twiggServerKey)) != 1 {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusForbidden)
			return
		}
		handler(w, ServerKeyAuthTrackMuxRequest{Request: r})
	})
}

type serverKeyAndTokenAuthTrackMux struct {
	twiggServerKey string
	signer         twiggtoken.TokenSigner
	mux            RlMux
}

func (m serverKeyAndTokenAuthTrackMux) HandleFunc(
	pattern string, handler func(w http.ResponseWriter, r ServerKeyAndTokenAuthTrackMuxRequest)) {
	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare(
			[]byte(r.Header.Get("TwiggServerKey")), []byte(m.twiggServerKey)) != 1 {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusForbidden)
			return
		}
		rawToken := twiggtoken.GetTwiggTokenInHeader(r)
		twToken, isExpiredErr, err := twiggtoken.ParseToken(rawToken, m.signer)
		if isExpiredErr {
			log.Printf("[serverKeyAndTokenAuthTrackMux] twigg token expired: %#v", twToken)
			http.Error(w, "token expired", http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, routes.SuperPoliteResponseToBadActors, http.StatusForbidden)
			return
		}
		handler(w, ServerKeyAndTokenAuthTrackMuxRequest{r, twToken})
	})
}
