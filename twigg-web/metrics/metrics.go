package metrics

import (
	"database/sql"
	"errors"
	"expvar"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite" // register driver
)

type service struct {
	startTime  time.Time
	isNotLinux bool

	flushInterval          time.Duration
	cleanupIntervalSeconds int64
	metricRetentionSeconds int64
	lastCleanup            int64
	waitingForFlush        bool
	waitingForCleanupFlush bool

	stopCh chan struct{}
	wg     sync.WaitGroup
	db     *sql.DB

	counters   sync.Map
	meanGauges sync.Map
}

const logPrefix = "[metrics]"

func newService(absDirectoryPath string, flushInterval time.Duration,
	cleanupIntervalSeconds int64, metricRetentionSeconds int64) (Service, func() error, error) {
	if metricRetentionSeconds < cleanupIntervalSeconds {
		panic(fmt.Sprintf("used retention %d <= cleanupInterval %d",
			metricRetentionSeconds, cleanupIntervalSeconds))
	}

	const dbFileName = "metrics.db"
	os.MkdirAll(absDirectoryPath, 0700)
	absPathToDbFile := filepath.Join(absDirectoryPath, dbFileName)
	db, err := sql.Open(
		"sqlite",
		fmt.Sprintf("file:%s?_pragma=journal_mode=WAL", absPathToDbFile))
	if err != nil {
		return Service{nil}, func() error { return nil },
			fmt.Errorf("failed to open db at %s: %s", absPathToDbFile, err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS metrics (
			name TEXT NOT NULL,    
			ts INTEGER NOT NULL,
            value INTEGER NOT NULL,
			PRIMARY KEY (name, ts)
        );
		CREATE INDEX IF NOT EXISTS metrics_by_name
		ON metrics (name, ts ASC);

        CREATE TABLE IF NOT EXISTS float_metrics (
			name TEXT NOT NULL,    
			ts INTEGER NOT NULL,
            value REAL NOT NULL,
			PRIMARY KEY (name, ts)
        );
		CREATE INDEX IF NOT EXISTS float_metrics_by_name
		ON float_metrics (name, ts ASC);
    `)
	if err != nil {
		db.Close()
		return Service{nil}, func() error { return nil },
			fmt.Errorf("failed to setup metrics db : %s", err)
	}

	s := &service{
		startTime:              time.Now(),
		isNotLinux:             runtime.GOOS != "linux",
		db:                     db,
		stopCh:                 make(chan struct{}),
		flushInterval:          flushInterval,
		metricRetentionSeconds: metricRetentionSeconds,
		cleanupIntervalSeconds: cleanupIntervalSeconds,

		counters:   sync.Map{},
		meanGauges: sync.Map{},
	}
	s.startFlushing()
	return Service{s}, s.close, nil
}

func (s *service) startFlushing() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.flushAndLog()
			case <-s.stopCh:
				log.Printf("%s stopped flushing metrics ok", logPrefix)
				return
			}
		}
	}()
}

func (s *service) WaitForFlush(t *testing.T) {
	if s.waitingForFlush {
		panic("tried to wait for flush twice")
	}
	s.waitingForFlush = true
	for s.waitingForFlush {
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *service) WaitForCleanupFlush(t *testing.T) {
	if s.waitingForCleanupFlush {
		panic("tried to wait for cleanup flush twice")
	}
	s.waitingForCleanupFlush = true
	for s.waitingForCleanupFlush {
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *service) flushAndLog() {
	// Take a snapshot of all the counter and gauge metrics to store them
	ts := time.Now().Unix()
	counterNameToSnapshot := make(map[string]uint64)
	s.counters.Range(func(k, v any) bool {
		counterNameToSnapshot[k.(string)] = v.(*atomic.Uint64).Swap(0)
		return true
	})
	gaugeNameToSnapshot := make(map[string]meanAndCount)
	s.meanGauges.Range(func(k, v any) bool {
		meanAndCount := v.(*floatMeanGauge).getSnapshotAndReset()
		gaugeNameToSnapshot[k.(string)] = meanAndCount
		return true
	})

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("%s failed to begin tx to flush metrics: %s", logPrefix, err)
		return
	}
	defer tx.Rollback()
	for counterName, counterCount := range counterNameToSnapshot {
		if counterCount == 0 {
			continue
		}
		_, err = tx.Exec(`INSERT INTO metrics (ts, name, value) VALUES (?, ?, ?)`,
			ts, counterName, counterCount)
		if err != nil {
			log.Printf("%s failed to exec to flush metrics: %s", logPrefix, err)
			return
		}
	}
	for gaugeName, gaugeSnapshot := range gaugeNameToSnapshot {
		// A mean of 0 is valid data; only skip when nothing was observed.
		if gaugeSnapshot.count == 0 {
			continue
		}
		_, err = tx.Exec(`INSERT INTO float_metrics (ts, name, value) VALUES (?, ?, ?)`,
			ts, gaugeName, gaugeSnapshot.mean)
		if err != nil {
			log.Printf("%s failed to exec float to flush metrics: %s", logPrefix, err)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Printf("%s failed to commit to flush metrics: %s", logPrefix, err)
		return
	}

	// Delete old metrics
	if ts-s.lastCleanup >= s.cleanupIntervalSeconds {
		s.cleanup(ts)
		s.waitingForCleanupFlush = false
	}

	s.waitingForFlush = false
}

func (s *service) cleanup(ts int64) {
	s.lastCleanup = ts
	cutoff := ts - s.metricRetentionSeconds
	res, err := s.db.Exec(`DELETE FROM metrics WHERE ts < ?`, cutoff)
	if err != nil {
		log.Printf("%s failed to cleanup old metrics: %s", logPrefix, err)
		return
	}
	nRows, err := res.RowsAffected()
	if err != nil {
		log.Printf("%s failed get num of rows affected by metrics cleanup: %s", logPrefix, err)
		return
	}
	res2, err := s.db.Exec(`DELETE FROM float_metrics WHERE ts < ?;`, cutoff)
	if err != nil {
		log.Printf("%s failed to cleanup old float metrics: %s", logPrefix, err)
		return
	}
	nRows2, err := res2.RowsAffected()
	if err != nil {
		log.Printf("%s failed get num of rows affected by float metrics cleanup: %s", logPrefix, err)
	}
	log.Printf("%s cleanup old metrics ok - %d metrics and %d float_metrics rows affected", logPrefix, nRows, nRows2)
}

func (s *service) close() error {
	close(s.stopCh)
	s.wg.Wait()
	return s.db.Close()
}

func (s *service) Increment(counterName string) {
	val, _ := s.counters.LoadOrStore(counterName, new(atomic.Uint64))
	val.(*atomic.Uint64).Add(1)
}
func (s *service) GetCounter(counterName string, start, end time.Time) ([]TimeSeriesPoint, error) {
	rows, err := s.db.Query(
		`SELECT ts, value FROM metrics 
         WHERE name = ? AND ts >= ? AND ts <= ?
         ORDER BY ts ASC`,
		counterName, start.Unix(), end.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var result []TimeSeriesPoint
	for rows.Next() {
		var ts int64
		var val uint64
		if err := rows.Scan(&ts, &val); err != nil {
			return nil, fmt.Errorf("failed to scan metrics row: %w", err)
		}
		result = append(result, TimeSeriesPoint{
			Timestamp: time.Unix(ts, 0),
			Value:     val,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate on metrics rows: %w", err)
	}

	return result, nil
}

func (s *service) Observe(gaugeMetricName string, value float64) {
	val, _ := s.meanGauges.LoadOrStore(gaugeMetricName, new(floatMeanGauge))
	val.(*floatMeanGauge).observe(value)
}
func (s *service) GetMeanGauge(gaugeMetricName string, start, end time.Time) ([]FloatTimeSeriesPoint, error) {
	rows, err := s.db.Query(
		`SELECT ts, value FROM float_metrics
	     WHERE name = ? AND ts >= ? AND ts <= ?
	     ORDER BY ts ASC`,
		gaugeMetricName, start.Unix(), end.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query float metrics: %w", err)
	}
	defer rows.Close()

	var result []FloatTimeSeriesPoint
	for rows.Next() {
		var ts int64
		var val float64
		if err := rows.Scan(&ts, &val); err != nil {
			return nil, fmt.Errorf("failed to scan float metrics row: %w", err)
		}
		result = append(result, FloatTimeSeriesPoint{
			Timestamp: time.Unix(ts, 0),
			Value:     val,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate on float metrics rows: %w", err)
	}
	return result, nil
}

type floatMeanGauge struct {
	mu    sync.Mutex
	sum   float64
	count uint64
}

func (m *floatMeanGauge) observe(v float64) {
	m.mu.Lock()
	m.sum += v
	m.count++
	m.mu.Unlock()
}

type meanAndCount struct {
	mean  float64
	count uint64
}

func (m *floatMeanGauge) getSnapshotAndReset() meanAndCount {
	m.mu.Lock()
	s := m.sum
	c := m.count
	m.sum = 0
	m.count = 0
	m.mu.Unlock()
	if c == 0 {
		return meanAndCount{0, 0}
	}
	return meanAndCount{mean: s / float64(c), count: c}
}

var (
	allRequestsByUrlPattern = expvar.NewMap("all-requests-by-url-pattern")
)

func (s *service) CountRequest(urlPattern string) {
	allRequestsByUrlPattern.Add(urlPattern, 1)
}
func (s *service) GetRequestCountHandler() http.Handler {
	return expvar.Handler()
}

func (s *service) Uptime() time.Duration {
	return time.Since(s.startTime).Truncate(time.Second)
}
func (s *service) MemMb() (alloc, heapInUse, sys float64, numGcRuns uint32) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	alloc = float64(m.Alloc) / 1024 / 1024
	heapInUse = float64(m.HeapInuse) / 1024 / 1024
	sys = float64(m.Sys) / 1024 / 1024
	numGcRuns = m.NumGC
	return
}
func (s *service) CpuPercent(samplingTime time.Duration) float64 {
	if s.isNotLinux {
		return -1
	}
	t0, t0_nonIdle := getCurrentStats()
	time.Sleep(samplingTime)
	t1, t1_nonIdle := getCurrentStats()
	return 100 * float64(t1_nonIdle-t0_nonIdle) / float64(t1-t0)
}

type cpuUsageSinceBoot struct {
	idle  int
	total int
}

// Interprets the /proc/stat lines that measure
// the cpu usage. They have the following form:
// cpuN 79242 0 74306 842486413 756859 6140 67701 0
// The form may vary depending on the kernel, but
// the 4th number is always the idle time
// https://www.idnt.net/en-US/kb/941772
func newCpuUsageSinceBoot(procStatLine string) (cpuUsageSinceBoot, error) {
	toInt := func(s string) (int, error) {
		n, e := strconv.Atoi(s)
		if e != nil {
			return 0, e
		}
		return n, nil
	}

	vals := strings.Split(procStatLine, " ")
	if len(vals) < 4 {
		return cpuUsageSinceBoot{},
			errors.New("invalid procStatLine:" + procStatLine)
	}

	cpuVals := []int{}
	sum := 0
	for i := range vals {
		val, err := toInt(vals[i])
		if err == nil {
			sum += val
			cpuVals = append(cpuVals, val)
		}
	}

	return cpuUsageSinceBoot{
		total: sum,
		idle:  cpuVals[3],
	}, nil
}
func getCurrentStats() (timeSinceBoot int, nonIdleTimeSinceBoot int) {
	cmd := exec.Command("grep", "cpu", "/proc/stat")
	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(out), "\n")
	timeSinceBoot = 0
	nonIdleTimeSinceBoot = 0
	for _, line := range lines {
		usage, err := newCpuUsageSinceBoot(line)
		if err == nil {
			timeSinceBoot += usage.total
			nonIdleTimeSinceBoot += (usage.total - usage.idle)
		}
	}
	return
}
