package memlogger

import (
	"monorepo/twigg-web/metrics"
	"sync"
	"time"
)

type MemLogger struct {
	l *logger
}

// Starts logging memory stats
func (ml MemLogger) Start() {
	ml.l.Start()
}

// Stops logging memory stats
func (ml MemLogger) Stop() {
	ml.l.Stop()
}

func New(interval time.Duration, m metrics.Service) MemLogger {
	return MemLogger{&logger{
		interval: interval,
		stopCh:   make(chan struct{}),
		m:        m,
		wg:       sync.WaitGroup{},
	}}
}