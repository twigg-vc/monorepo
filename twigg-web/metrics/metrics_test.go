package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const oneHourInSeconds = 60 * 60
const counterName = "my-counter"
const meanGaugeName = "my-mean-gauge"

func TestEmpty(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, "test-dir")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	m, closeM, err := New(dir, 5*time.Second,
		/*cleanupInternalSeconds*/ oneHourInSeconds,
		/*retention*/ 2*oneHourInSeconds)
	if err != nil {
		t.Fatal(err)
	}
	defer closeM()
	const minutesAgo = 2
	ts, err := m.GetCounter(counterName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 0 {
		t.Fatalf("got %v", ts)
	}
}

func TestTwoIncrements(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, "test-dir")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	m, closeM, err := New(dir, 100*time.Millisecond,
		/*cleanupInternalSeconds*/ oneHourInSeconds,
		/*retention*/ 2*oneHourInSeconds)
	if err != nil {
		t.Fatal(err)
	}
	defer closeM()

	// Wait for a first flush so that we write on the same "bucket"
	m.WaitForFlush(t)
	// Write two increments. This should be MUCH faster than the time to flush,
	// so we should always succeed to write before the next flush.
	m.Increment(counterName)
	m.Increment(counterName)
	m.WaitForFlush(t)

	// Expect one bucket with data
	const minutesAgo = 2
	ts, err := m.GetCounter(counterName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("got len %v", len(ts))
	}
	if ts[0].Value != 2 {
		t.Fatalf("got value %d", ts[0].Value)
	}
}

func TestEviction(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, "test-dir")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	const cleanupInSeconds = 1
	const retentionInSeconds = 1
	m, closeM, err := New(dir, 200*time.Millisecond,
		cleanupInSeconds, retentionInSeconds)
	if err != nil {
		t.Fatal(err)
	}
	defer closeM()
	m.Increment(counterName)
	m.Increment(counterName)
	m.Increment(counterName)
	m.Increment(counterName)
	m.Observe(meanGaugeName, 7.0)
	m.Observe(meanGaugeName, 7.0)
	m.Observe(meanGaugeName, 7.0)
	// Wait for a first flush and check the metrics
	m.WaitForFlush(t)
	const minutesAgo = 1
	ts, err := m.GetCounter(counterName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	totalValue := uint64(0)
	for _, t := range ts {
		totalValue += t.Value
	}
	if totalValue != 4 {
		t.Fatal("expected 4")
	}
	gTs, err := m.GetMeanGauge(meanGaugeName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(gTs) == 0 {
		t.Fatalf("got no mean gauge vals")
	}
	for _, v := range gTs {
		if v.Value != 7.0 {
			t.Fatalf("got mean val %f", v.Value)
		}
	}

	// Now wait for a second and for a next flush to ensure the eviction will
	// happen
	time.Sleep(1 * time.Second)
	m.WaitForCleanupFlush(t)
	ts, err = m.GetCounter(counterName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 0 {
		t.Fatalf("got ts len = %d", len(ts))
	}
	gTs, err = m.GetMeanGauge(meanGaugeName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(gTs) != 0 {
		t.Fatalf("got gs len = %d", len(gTs))
	}
}

func TestMeanGauge(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, "test-dir")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	m, closeM, err := New(dir, 200*time.Millisecond,
		/*cleanupInternalSeconds*/ oneHourInSeconds,
		/*retention*/ 2*oneHourInSeconds)
	if err != nil {
		t.Fatal(err)
	}
	defer closeM()

	// Wait for a flush to ensure both writes fall in the same bucket
	m.Observe(meanGaugeName, 2.0)
	m.Observe(meanGaugeName, 3.0)
	m.WaitForFlush(t)
	const minutesAgo = 10
	ts, err := m.GetMeanGauge(meanGaugeName,
		time.Now().Add(-time.Duration(minutesAgo)*time.Minute),
		time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 {
		t.Fatalf("got len = %d", len(ts))
	}
	if ts[0].Value != (2.0+3.0)/2 {
		t.Fatalf("got mean %f", ts[0].Value)
	}
}

func TestMeanGaugeWithZeroMeanStillCreatesPoint(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, "test-dir")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	const flushInterval = 10 * time.Millisecond
	m, closeM, err := New(dir, flushInterval,
		/*cleanupInternalSeconds*/ oneHourInSeconds,
		/*retention*/ 2*oneHourInSeconds)
	if err != nil {
		t.Fatal(err)
	}
	defer closeM()

	// Observations happened, but their mean is 0. A point must still be
	// flushed, otherwise the gauge timeseries silently loses buckets.
	m.Observe(meanGaugeName, 0.0)
	m.Observe(meanGaugeName, 0.0)
	m.WaitForFlush(t)
	const minutesAgo = 10
	start := time.Now().Add(-time.Duration(minutesAgo) * time.Minute)
	end := time.Now()
	ts, err := m.GetMeanGauge(meanGaugeName,
		start, end)
	if err != nil {
		t.Fatal(err)
	}
	// We might get 1 or 2 buckets - bc maybe the metrics were flushed between
	// the first and the second Observe (unlikelly but anyway)
	if len(ts) < 1 {
		t.Fatalf("got %d points", len(ts))
	}
	for _, val := range ts {
		if val.Value != 0.0 {
			t.Fatalf("got mean %f", ts[0].Value)
		}
	}
}