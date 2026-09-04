package metrics

import (
	"net/http"
	"testing"
	"time"
)

const (
	TotalRequestsCounterName             = "requests"
	MeanRequestsMillisecLatencyGaugeName = "requests-millis"
)

// MUST BE CONSTRUCTED WITH `New`.
// Service has methods for storing and reading metrics - both inmemory and
// on disk using a sqlite database for persistence.
type Service struct {
	s *service
}

// The metrics will be saved in a sqlite db in the provided path.
// Once created, it'll automatically start flushing the metrics periodically.
// We run a "cleanup" every `cleanupIntervalSeconds` seconds and delete data
// older than `metricRetentionSeconds` seconds
func New(absDirectoryPath string,
	flushInterval time.Duration,
	cleanupIntervalSeconds int64, metricRetentionSeconds int64) (s Service, closeS func() error, err error) {
	return newService(absDirectoryPath, flushInterval,
		cleanupIntervalSeconds, metricRetentionSeconds)
}

// Returns the duration since construction
func (s Service) Uptime() time.Duration { return s.s.Uptime() }

// Returns memory stats in Mb
func (s Service) MemMb() (alloc, heapInUse, sys float64, numGcRuns uint32) { return s.s.MemMb() }

// Samples CPU to find usage. Only works on linux (returns -1 otherwise).
func (s Service) CpuPercent(samplingTime time.Duration) float64 { return s.s.CpuPercent(samplingTime) }

// TimeSeriesPoint represents a point in a discrete-value timeseries
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     uint64
}

// Increment the requests counter
func (s Service) Increment(counterName string) { s.s.Increment(counterName) }

// Get the timeseries of the provided counter
func (s Service) GetCounter(counterName string, start, end time.Time) ([]TimeSeriesPoint, error) {
	return s.s.GetCounter(counterName, start, end)
}

// Makes the gauge metric observe the value provided
func (s Service) Observe(gaugeMetricName string, value float64) { s.s.Observe(gaugeMetricName, value) }

// FloatTimeSeriesPoint represents a point in a continuous-value timeseries
type FloatTimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}

// Returns the timeseries of the mean value of the gauge metric
func (s Service) GetMeanGauge(gaugeMetricName string, start, end time.Time) ([]FloatTimeSeriesPoint, error) {
	return s.s.GetMeanGauge(gaugeMetricName, start, end)
}

// Used to wait for flushes during testing
func (s Service) WaitForFlush(t *testing.T) { s.s.WaitForFlush(t) }

// Used to wait for flushes that cause a cleanup during testing
func (s Service) WaitForCleanupFlush(t *testing.T) { s.s.WaitForCleanupFlush(t) }

// Counts a request to the provided url pattern so that it can later be read with
// the GetRequestCountHandler.
// Use PATTERN and not the URL itself to avoid cardinality explosion.
func (s Service) CountRequest(urlPattern string) { s.s.CountRequest(urlPattern) }

// Returns a handler that serves a json with the results of CountRequest usage
func (s Service) GetRequestCountHandler() http.Handler { return s.s.GetRequestCountHandler() }
