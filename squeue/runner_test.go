package squeue

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerRegister(t *testing.T) {
	srv := NewtTestStorage( /*NowGetter=*/ nil, t)
	r := NewRunner(srv, 10*time.Millisecond, 2)
	obs := handleObserver{}
	r.AddObserver(&obs)

	r.Register("life",
		func(b []byte) error {
			return nil
		},
		func(payload []byte) string {
			return "payload: " + string(payload)
		},
		/*onDeadLetter*/ nil,
	)
	r.Start()
	r.Enqueue("life", []byte("memento mori"))
	for obs.count == 0 {
		time.Sleep(time.Millisecond)
	}
	r.Stop()
	if len(obs.payloads) != 1 {
		t.Fatal("expected 1 handle call")
	}
	if string(obs.payloads[0]) != "memento mori" {
		t.Fatal("invalid payload")
	}
	if srv.SuccessCount() != 1 {
		t.Fatalf("Success() should have been called one not %v", srv.SuccessCount())
	}

	s, ok := r.GetDisplayString("life", []byte("abc"))
	if !ok {
		t.Fatal("got non ok for known display string decoder")
	}
	if s != "payload: abc" {
		t.Fatalf("unexpected display string: %s", s)
	}
	_, ok = r.GetDisplayString("unknown-payload", []byte(""))
	if ok {
		t.Fatal("got ok for unknown display string decoder")
	}
}

func TestRunnerHandlerFails(t *testing.T) {
	srv := NewtTestStorage( /*NowGetter=*/ nil, t)
	r := NewRunner(srv, 10*time.Millisecond, 2)
	obs := handleObserver{}
	r.AddObserver(&obs)
	r.Register("email", func(b []byte) error {
		return errors.New("boom")
	}, nil, nil)

	r.Start()
	r.Enqueue("email", []byte("marcus@Aurelius.com"))
	for obs.count == 0 {
		time.Sleep(time.Millisecond)
	}
	r.Stop()
	if srv.FailCount() != 1 {
		t.Fatalf("Fail() should have been called ones not %v", srv.FailCount())
	}
}

func TestHandlerNeverCalled(t *testing.T) {
	srv := NewtTestStorage( /*NowGetter=*/ nil, t)
	r := NewRunner(srv, 10*time.Millisecond, 2)

	called := false
	r.Register("email", func(b []byte) error {
		called = true
		return nil
	}, nil, nil)
	r.Register("notEmail", func(b []byte) error {
		return nil
	}, nil, nil)

	r.Start()
	r.Enqueue("notEmail", []byte("no email"))
	time.Sleep(20 * time.Millisecond)
	r.Stop()

	if called {
		t.Fatal("handler should not be called")
	}
}

func TestRunnerStopInterruptsSleep(t *testing.T) {
	srv := NewtTestStorage( /*NowGetter=*/ nil, t)

	// Use a long sleep duration so we can detect whether Stop() waits
	hugeSleepDuration := 100_000 * time.Second
	r := NewRunner(srv, hugeSleepDuration, 2)
	obs := handleObserver{}
	r.AddObserver(&obs)
	r.Start()

	// Wait for it to start sleeping
	for obs.sleepCount < 1 {
		time.Sleep(50 * time.Millisecond)
	}

	start := time.Now()
	r.Stop()
	elapsed := time.Since(start)
	// Stop must return *far* quicker than sleep duration,
	// meaning the idle wait was interrupted properly.
	// In production we expect this to only take a few ms but the test uses
	// a relativelly huge number for the expected `elapsed` just to avoid flakyness
	// This is ok because what we're really testing is that this takes much
	// less than the `hugeSleepDuration`
	if elapsed > 5*time.Second {
		t.Fatalf("Stop() took too long (%v). Expected it to interrupt sleep immediately.", elapsed)
	}
}

func TestWakeup(t *testing.T) {
	srv := NewtTestStorage( /*NowGetter=*/ nil, t)
	// use a huge sleep time
	r := NewRunner(srv, 10*time.Hour, 2)
	obs := handleObserver{}
	r.AddObserver(&obs)

	count := 0
	r.Register("count", func(b []byte) error {
		count += 1
		return nil
	}, nil, nil)
	r.Start()
	defer r.Stop()

	start := time.Now()
	for obs.sleepCount == 0 {
		time.Sleep(time.Millisecond)
		if time.Since(start) > time.Second {
			t.Fatalf("spent too long waiting for sleep")
		}
	}
	// At this point, the queue is sleeping. It should quickly wakeup on enqueue
	err := r.Enqueue("count", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	for count == 0 {
		time.Sleep(time.Millisecond)
		if time.Since(start) > time.Second {
			t.Fatalf("spent too long waiting for wakeup")
		}
	}
}

func TestConcurrency(t *testing.T) {
	srv := NewtTestStorage( /*NowGetter=*/ nil, t)
	// Instantiate a runner with a large concurrency
	const concurrency = 20
	r := NewRunner(srv, 10*time.Millisecond, concurrency)
	obs := handleObserver{}
	r.AddObserver(&obs)
	// Register a handler that just sleeps for 200 ms
	var totalMsSpentSleeping atomic.Int64
	r.Register("sleep", func(b []byte) error {
		time.Sleep(200 * time.Millisecond)
		totalMsSpentSleeping.Add(200)
		return nil
	}, nil, nil)

	// Enqueue 20 tasks.
	// Each will sleep for 200ms -> In total we'll spend 4000ms sleeping.
	// Those "sleeps" will run in parallel, so it should take much
	// less than 4000ms to run all. It should take ~ the amount it takes to run
	// one tasks (we just don't use a too tight expectation to avoid
	// test flakyness)
	expectAllTasksToRunInLessThan := 600 * time.Millisecond
	r.Start()
	start := time.Now()
	for i := 0; i < 20; i++ {
		r.Enqueue("sleep", []byte(""))
	}
	// Wait until they all run
	for obs.count < 20 {
		time.Sleep(10 * time.Millisecond)
	}
	r.Stop()
	if totalMsSpentSleeping.Load() != 4000 {
		panic("something is wrong with this test")
	}
	allTasksRanIn := time.Since(start)
	if allTasksRanIn >= expectAllTasksToRunInLessThan {
		t.Fatalf("time to run all tasks: %v", allTasksRanIn)
	}
}

type handleObserver struct {
	mu           sync.Mutex
	payloadTypes []string
	payloads     [][]byte
	results      []error
	count        int
	sleepCount   int
}

func (h *handleObserver) OnHandle(
	payloadType string, payload []byte, result error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.payloadTypes = append(h.payloadTypes, payloadType)
	h.payloads = append(h.payloads, payload)
	h.results = append(h.results, result)
	h.count++
}
func (h *handleObserver) OnSleep() {
	h.sleepCount += 1
}
