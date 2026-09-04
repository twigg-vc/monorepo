package wrappers

import (
	"log"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type rlMux struct {
	mux Mux
	rl  *rate.Limiter
}

func newRateLimitted(maxQps float64, maxQpsBurst int, mux Mux) rlMux {
	return rlMux{
		mux: mux,
		rl:  rate.NewLimiter(rate.Limit(maxQps), maxQpsBurst),
	}
}

const errMsg = "Server busy, try again later"
const logSuccessfullAdminDashRequests = false

func (m rlMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	if m.rl.Allow() {
		m.mux.ServeHTTP(recorder, r)
	} else {
		http.Error(recorder, errMsg, http.StatusTooManyRequests)
	}
	duration := time.Since(startTime)
	isSuccessfullAdminDashReq := recorder.statusCode == http.StatusOK && (r.URL.Path == "/logs" || r.URL.Path == "/queued" || r.URL.Path == "/deadletter")
	shouldLog := !isSuccessfullAdminDashReq || isSuccessfullAdminDashReq && logSuccessfullAdminDashRequests
	if shouldLog {
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.statusCode, duration)
	}
}
func (m rlMux) HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}