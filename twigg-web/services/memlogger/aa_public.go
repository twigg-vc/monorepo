package memlogger

import (
	"monorepo/twigg-web/metrics"
	"sync"
	"time"
)

type MemLogger interface {
	// Starts logging memory stats
	Start()
	// Stops logging memory stats
	Stop()

	private()
}

func New(interval time.Duration, m metrics.Service) MemLogger {
	return &logger{
		interval: interval,
		stopCh:   make(chan struct{}),
		m:        m,
		wg:       sync.WaitGroup{},
	}
}
