package wrappers

import (
	"log"
	"monorepo/twigg-web/metrics"
	"monorepo/twigg-web/services/session"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

type rlMux struct {
	metrics        metrics.Service
	sessionService session.Service
	mux            Mux
	rl             *rate.Limiter
}

func newRateLimitted(maxQps float64, maxQpsBurst int, metrics metrics.Service,
	sessionService session.Service, mux Mux) RlMux {
	return rlMux{
		metrics:        metrics,
		sessionService: sessionService,
		mux:            mux,
		rl:             rate.NewLimiter(rate.Limit(maxQps), maxQpsBurst),
	}
}

const errMsg = "Server busy, try again later"

func (m rlMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	if m.rl.Allow() {
		m.mux.ServeHTTP(recorder, r)
	} else {
		http.Error(recorder, errMsg, http.StatusTooManyRequests)
	}
	duration := time.Since(startTime)
	suffixes := []string{".svg", ".css", ".js", ".png", ".ico"}
	for _, s := range suffixes {
		if strings.HasSuffix(r.URL.Path, s) {
			return
		}
	}
	milliSec := float64(duration) / float64(time.Millisecond)
	m.metrics.Observe(metrics.MeanRequestsMillisecLatencyGaugeName, milliSec)
	userId, username, _, ok := m.sessionService.ReadSessionCookie(r)
	if ok {
		if username != "" {
			log.Printf("%s usr=%s %s %d %s", r.Method, username,
				r.URL.Path, recorder.statusCode, duration)
		} else {
			log.Printf("%s usrId=%d %s %d %s", r.Method, userId,
				r.URL.Path, recorder.statusCode, duration)
		}
	} else {
		log.Printf("%s %s %d %s", r.Method,
			r.URL.Path, recorder.statusCode, duration)
	}
}
func (m rlMux) HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		m.metrics.Increment(metrics.TotalRequestsCounterName)
		m.metrics.CountRequest(pattern)
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
