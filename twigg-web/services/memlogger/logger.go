package memlogger

import (
	"log"
	"monorepo/twigg-web/metrics"
	"sync"
	"time"
)

type logger struct {
	interval time.Duration
	stopCh   chan struct{}
	m        metrics.Service
	wg       sync.WaitGroup
}

func (l *logger) Start() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()
		for {
			select {
			case <-l.stopCh:
				log.Print("[mem] logger stopped")
				return
			case <-ticker.C:
				alloc, heapInUse, sys, numGc := l.m.MemMb()
				log.Printf("[mem] activellyUsed=%.1fMB allocated=%.1fMB gotFromOs=%.1fMB gc=%v",
					alloc,
					heapInUse,
					sys,
					numGc,
				)
			}
		}
	}()
}
func (l *logger) Stop() {
	close(l.stopCh)
	l.wg.Wait()
}
func (l *logger) private() {

}
