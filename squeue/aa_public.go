// This package contains implementation of a simple queue service that uses
// a sql database under the hood.
package squeue

import (
	"monorepo/base/iterator"
	"sync"
	"testing"
	"time"
)

// MUST BE INITIALIZED WITH `NewRunner`
// Runs a queue and routes the payloads to handlers.
type Runner struct {
	r *runner
}

func NewRunner(db Storage, sleepDuration time.Duration, maxConcurrency int) Runner {
	return Runner{newRunner(db, sleepDuration, maxConcurrency)}
}
func (r Runner) Start() {
	r.r.Start()
}
func (r Runner) Stop() {
	r.r.Stop()
}
func (r Runner) IsSleeping() bool {
	return r.r.isSleeping
}

// withDecoder is optional and is used to render human readable payloads
// in logs etc. withOnMoveToDeadLetter is also optional and is called when an
// entry is moved to the deadleter. This function is called only a couple times
// if it errors and can be called >1 (i.e. there's no execution guarantee).
func (r Runner) Register(payloadType string,
	handler func(payload []byte) error,
	withDecoder func(payload []byte) string,
	withOnMoveToDeadLetter func(payload []byte) error) {
	r.r.Register(payloadType, handler, withDecoder, withOnMoveToDeadLetter)
}
func (r Runner) Enqueue(payloadType string, payload []byte) error {
	return r.r.Enqueue(payloadType, payload)
}
func (r Runner) AddObserver(o RunnerObserver) {
	r.r.AddObserver(o)
}

// MUST BE INITIALIZED WITH `NewPayloadTypeObserver`
// Default helper implementation to observe executions of a specific payload
type PayloadTypeObserver struct {
	PayloadType string
	OkCount     int
	ErrCount    int
	mu          *sync.Mutex
}

func NewPayloadTypeObserver(PayloadType string) *PayloadTypeObserver {
	return &PayloadTypeObserver{
		PayloadType: PayloadType,
		OkCount:     0,
		ErrCount:    0,
		mu:          &sync.Mutex{},
	}
}
func (p *PayloadTypeObserver) OnSleep() {}
func (p *PayloadTypeObserver) OnHandle(payloadType string, payload []byte, result error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if payloadType != p.PayloadType {
		return
	}
	if result == nil {
		p.OkCount += 1
	} else {
		p.ErrCount += 1
	}
}

func (p *PayloadTypeObserver) WaitForOkCount(n int, t *testing.T) {
	start := time.Now()
	for p.OkCount < n {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 30*time.Second {
			t.Fatalf("spent too long waiting for successfull execution of payload type %s",
				p.PayloadType)
		}
	}
}

func (p *PayloadTypeObserver) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.OkCount = 0
	p.ErrCount = 0
}

// Disable auto-wakeup on enqueue (only used for testing)
func (r Runner) DisableWakeup(t *testing.T) {
	r.r.wakeupIsDisabled = true
}

// Decodes a payload using a decoder from RegisterDisplayStringDecoder.
// Returns `"", false` if not found
func (r Runner) GetDisplayString(payloadType string, payload []byte) (s string, ok bool) {
	return r.r.GetDisplayString(payloadType, payload)
}

// Used to observe when handlers are called
type RunnerObserver interface {
	OnSleep()
	OnHandle(payloadType string, payload []byte, result error)
}

type Entry struct {
	Id          int64
	PayloadType string
	Payload     []byte
}

// Used to store the state of each message before the runner processes them
type Storage interface {
	Enqueue(payloadType string, payload []byte) error
	Next(afterId int64, n int) ([]Entry, error)
	Fail(entryId int64, onDeadLetter func(payload []byte) error) error
	Success(entryId int64) error
}

// MUST BE INITIALIZED WITH `NewSqliteStorage`
// Stores the queue data in a sqlite db
type SqliteStorage struct {
	s *sqliteStorage
}

func (s SqliteStorage) Enqueue(payloadType string, payload []byte) error {
	return s.s.Enqueue(payloadType, payload)
}
func (s SqliteStorage) Next(afterId int64, n int) ([]Entry, error) {
	return s.s.Next(afterId, n)
}
func (s SqliteStorage) Fail(entryId int64, onDeadLetter func(payload []byte) error) error {
	return s.s.Fail(entryId, onDeadLetter)
}
func (s SqliteStorage) Success(entryId int64) error {
	return s.s.Success(entryId)
}
func (s SqliteStorage) GetAllQueued() (it iterator.I[QueueItem], unlock func(), err error) {
	return s.s.GetAllQueued()
}
func (s SqliteStorage) GetAllDeadLetter() (it iterator.I[DeadLetterItem], unlock func(), err error) {
	return s.s.GetAllDeadLetter()
}
func (s SqliteStorage) RequeueDeadLetter(id int64) error {
	return s.s.RequeueDeadLetter(id)
}
func (s SqliteStorage) SetBaseRetryDelay(d time.Duration) {
	s.s.baseRetryDelay = d
}
func (s SqliteStorage) SetMaxNumberOfRetries(n int64) {
	s.s.maxNumberOfRetries = n
}

type QueueItem struct {
	Id          int64
	PayloadType string
	Payload     []byte
	CreatedAt   string
	AvailableAt string
	RetryCount  int64
}
type DeadLetterItem struct {
	Id                int64
	PayloadType       string
	Payload           []byte
	OriginalCreatedAt string
	FailedAt          string
	RetryCount        int64
}

// Returns a default instance that uses sqlite under the hood
func NewSqliteStorage(absDirectoryPath string) (SqliteStorage, func() error, error) {
	db, closeDb, err := newSqliteStorage(absDirectoryPath)
	return SqliteStorage{db}, closeDb, err
}

// MUST BE INITIALIZED WITH `NewtTestStorage`
// Helper storage for use during testing.
type TestStorage struct {
	ts *testStorage
}

// If n=nil, time.Now() is used.
func NewtTestStorage(n NowGetter, t *testing.T) TestStorage {
	return newtTestStorage(n, t)
}

// The first failure is retried after baseRetryDelay
func (ts TestStorage) SetBaseRetryDelay(d time.Duration) {
	ts.ts.baseRetryDelay = d
}

// After each failure, the retry delay is multiplied by delayMultiplier
func (ts TestStorage) SetDelayMultiplier(delayMultiplier int) {
	ts.ts.SetDelayMultiplier(int64(delayMultiplier))
}

// Limits the maximum retry delay
func (ts TestStorage) SetMaxRetryDelay(d time.Duration) {
	ts.ts.maxRetryDelay = d
}

// If retry_count >= maxNumberOfRetries. queue goes to dead_letter.
func (ts TestStorage) SetMaxNumberOfRetries(maxNumberOfRetries int64) {
	ts.ts.SetMaxNumberOfRetries(maxNumberOfRetries)
}

// Sets the delay bewteen each call of the onDeadLetter
func (q TestStorage) SetOnDeadLetterRetryDelay(d time.Duration) {
	q.ts.SetOnDeadLetterRetryDelay(d)
}

// Sets max retries of onDeadLetter
func (q TestStorage) SetOnDeadLetterRetries(n int) {
	q.ts.SetOnDeadLetterRetries(n)
}

// Returns number of times Fail was called
func (ts TestStorage) FailCount() int {
	return ts.ts.FailCount()
}

// Returns number of times Success was called
func (ts TestStorage) SuccessCount() int {
	return ts.ts.SuccessCount()
}

func (ts TestStorage) Enqueue(payloadType string, payload []byte) error {
	return ts.ts.Enqueue(payloadType, payload)
}
func (ts TestStorage) Next(afterId int64, n int) ([]Entry, error) {
	return ts.ts.Next(afterId, n)
}
func (ts TestStorage) Fail(entryId int64, onDeadLetter func([]byte) error) error {
	return ts.ts.Fail(entryId, onDeadLetter)
}
func (ts TestStorage) Success(entryId int64) error {
	return ts.ts.Success(entryId)
}

type NowGetter interface {
	Now() time.Time
}
