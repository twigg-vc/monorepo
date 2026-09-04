package squeue

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type runner struct {
	svc               Storage
	handlers          map[string]func([]byte) error
	decoders          map[string]func(payload []byte) string
	onDeadletter      map[string]func(payload []byte) error
	sleepDuration     time.Duration
	wakeupCh          chan struct{}
	stopCh            chan struct{}
	stopWaitGroup     sync.WaitGroup
	observers         []RunnerObserver
	maxConcurrency    int
	runningJobsById   map[int64]bool
	runningJobsByIdMu sync.Mutex
	wakeupIsDisabled  bool
	isSleeping        bool
}

func newRunner(svc Storage, sleepDuration time.Duration, maxConcurrency int) *runner {
	if maxConcurrency <= 0 {
		panic("got maxConcurrency<=0")
	}
	return &runner{
		svc:               svc,
		handlers:          make(map[string]func([]byte) error),
		decoders:          map[string]func([]byte) string{},
		onDeadletter:      map[string]func(payload []byte) error{},
		sleepDuration:     sleepDuration,
		wakeupCh:          make(chan struct{}, 1),
		stopCh:            make(chan struct{}),
		stopWaitGroup:     sync.WaitGroup{},
		maxConcurrency:    maxConcurrency,
		runningJobsById:   map[int64]bool{},
		runningJobsByIdMu: sync.Mutex{},
	}
}

func (r *runner) Register(payloadType string,
	cb func([]byte) error, dec func(payload []byte) string,
	onDeadLetter func(payload []byte) error) {
	if _, exists := r.handlers[payloadType]; exists {
		panic(fmt.Sprintf("payload type: %q already exists", payloadType))
	}
	r.handlers[payloadType] = cb
	if dec == nil {
		r.decoders[payloadType] = func(payload []byte) string { return string(payload) }
	} else {
		r.decoders[payloadType] = dec
	}
	if onDeadLetter != nil {
		r.onDeadletter[payloadType] = onDeadLetter
	} else {
		r.onDeadletter[payloadType] = func(payload []byte) error { return nil }
	}
}
func (r *runner) Enqueue(payloadType string, payload []byte) error {
	err := r.svc.Enqueue(payloadType, payload)
	if err != nil {
		return err
	}
	if r.wakeupIsDisabled {
		return nil
	}
	// Send wakeup if channel is not full
	select {
	case r.wakeupCh <- struct{}{}:
	default:
		// Drop wakeup because channel is full.
	}
	return nil
}

// Decodes a payload using a decoder from RegisterDisplayStringDecoder.
func (r *runner) GetDisplayString(payloadType string, payload []byte) (string, bool) {
	dec, ok := r.decoders[payloadType]
	if !ok {
		return "", false
	}
	return dec(payload), true
}

func (r *runner) Start() {
	r.stopWaitGroup.Add(1)
	go r.runUntillStopIsCalled()
}
func (r *runner) Stop() {
	log.Print("[queue] stopping queue ...")
	r.runningJobsByIdMu.Lock()
	for id := range r.runningJobsById {
		log.Printf("[queue] jobId=%d is still running", id)
	}
	r.runningJobsByIdMu.Unlock()
	close(r.stopCh)
	r.stopWaitGroup.Wait()
	log.Print("[queue] stopped")
}
func (r *runner) AddObserver(o RunnerObserver) {
	r.observers = append(r.observers, o)
}

// Helper to sleep for r.sleepDuration or untill Stop is called
func (r *runner) sleepForDurationOrUntillStop() {
	r.isSleeping = true
	defer func() {
		r.isSleeping = false
	}()
	for _, obs := range r.observers {
		obs.OnSleep()
	}
	// Instead of time.Sleep (which can't be interrupted), we
	// use a timer inside a select so Stop() can wake us immediately
	timer := time.NewTimer(r.sleepDuration)
	select {
	case <-r.stopCh:
		// Stop() was called while we were waiting.
		// Stop the timer to free resources. If it already fired,
		// drain the timer channel so it doesn't leak.
		if !timer.Stop() {
			<-timer.C
		}
		log.Print("[queue] runner stopping during idle wait")
		return
	case <-timer.C:
		return
	case <-r.wakeupCh:
		return
	}
}

// Main loop that blocks until Stop() is called
func (r *runner) runUntillStopIsCalled() {
	defer r.stopWaitGroup.Done()
	runQueue := make(chan bool, r.maxConcurrency)
	for {
		// Stop if needed
		select {
		case <-r.stopCh:
			return
		default:
		}

		// Fetch entries and sleep if needed
		nEntries := r.maxConcurrency * 2
		const afterId int64 = 0
		entries, err := r.svc.Next(afterId, nEntries)
		if err != nil {
			log.Printf("queue Next() err=%s", err)
			continue
		}
		if len(entries) == 0 {
			r.sleepForDurationOrUntillStop()
			continue
		}

		// Run each entry
		for _, entry := range entries {
			// Skip jobs already running
			r.runningJobsByIdMu.Lock()
			if r.runningJobsById[entry.Id] {
				r.runningJobsByIdMu.Unlock()
				continue
			}
			r.runningJobsById[entry.Id] = true
			r.runningJobsByIdMu.Unlock()

			// Wait until there is room in the runQueue
			select {
			case <-r.stopCh:
				return
			case runQueue <- true:
			}

			r.stopWaitGroup.Add(1)
			go func(e Entry) {
				defer func() {
					// Remove id from map of running jobs
					r.runningJobsByIdMu.Lock()
					delete(r.runningJobsById, e.Id)
					r.runningJobsByIdMu.Unlock()
					// Remove from the stopWaitGroup to allow stop
					r.stopWaitGroup.Done()
					// Remove entry from tue queue
					<-runQueue
				}()
				r.runEntry(e)
			}(entry)
		}
	}
}
func (r *runner) runEntry(entry Entry) {
	// Create the onDeadLetter func that takes no args
	onDeadLetter, ok := r.onDeadletter[entry.PayloadType]
	if !ok {
		onDeadLetter = func(payload []byte) error { return nil }
	}

	handler, ok := r.handlers[entry.PayloadType]
	if !ok {
		log.Printf("[queue] %q: no handler", entry.PayloadType)
		err := r.svc.Fail(entry.Id, onDeadLetter)
		if err != nil {
			log.Printf("[queue] Fail(entryId=%d) err:%s", entry.Id, err)
		}
		return
	}
	displaySting, hasDisplayString := r.GetDisplayString(entry.PayloadType, entry.Payload)
	if !hasDisplayString {
		displaySting = string(entry.Payload)
	}
	log.Printf("[queue] %q: will handle entryId=%d payload=%q ...", entry.PayloadType, entry.Id, displaySting)
	err := handler(entry.Payload)
	for _, obs := range r.observers {
		obs.OnHandle(entry.PayloadType, entry.Payload, err)
	}
	if err != nil {
		log.Printf("[queue] %q: handle entryId=%d payload=%q err: %s", entry.PayloadType, entry.Id, displaySting, err)
		err = r.svc.Fail(entry.Id, onDeadLetter)
		if err != nil {
			log.Printf("[queue] Fail(entryId=%d) err:%s", entry.Id, err)
		}
		return
	}
	err = r.svc.Success(entry.Id)
	if err != nil {
		log.Printf("[queue] %q: Success(entryId=%d) err:%s -> entry will be handled again", entry.PayloadType, entry.Id, err)
		time.Sleep(time.Second * 30)
		return
	}
	log.Printf("[queue] %q: handle entryId=%d payload=%q OK", entry.PayloadType, entry.Id, displaySting)
}
