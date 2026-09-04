package squeue

import (
	"errors"
	"monorepo/base/iterator"
	"reflect"
	"testing"
	"time"
)

func TestQueueLifecycle(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)

	// Enqueue a payload
	payloadType := "test-type"
	payload := []byte("hello world")
	if err := q.Enqueue(payloadType, payload); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// next should return the same payload
	entries, err := q.Next(0, 10)
	if err != nil {
		t.Fatalf("failed to get next: %v", err)
	}
	if len(entries) != 1 {
		t.Fatal("got no entry")
	}
	id := entries[0].Id
	gotType := entries[0].PayloadType
	gotPayload := entries[0].Payload
	if gotType != payloadType {
		t.Errorf("expected payloadType %q, got %q", payloadType, gotType)
	}
	if string(gotPayload) != string(payload) {
		t.Errorf("expected payload %q, got %q", payload, gotPayload)
	}
	if id != 1 {
		t.Errorf("expected id to be 1")
	}

	// Success should remove the job
	if err := q.Success(id); err != nil {
		t.Fatalf("failed to mark success: %v", err)
	}

	// next should now return no rows
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("expected no next err, queue is empty")
	}
	if len(entries) != 0 {
		t.Fatalf("got non empty entries")
	}
}

func TestNextReturnsMany(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)

	if err := q.Enqueue("1", []byte("111")); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	if err := q.Enqueue("2", []byte("222")); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	if err := q.Enqueue("3", []byte("333")); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	if err := q.Enqueue("4", []byte("444")); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Get 3 entries after id=0
	entries, err := q.Next(0, 3)
	if err != nil {
		t.Fatalf("failed to get next: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries", len(entries))
	}
	expected := []Entry{
		{Id: 1, PayloadType: "1", Payload: []byte("111")},
		{Id: 2, PayloadType: "2", Payload: []byte("222")},
		{Id: 3, PayloadType: "3", Payload: []byte("333")},
	}
	if !reflect.DeepEqual(entries, expected) {
		t.Fatalf("unexpected entries: %v", entries)
	}
	// Get 2 entries after id=3
	entries, err = q.Next(3, 2)
	if err != nil {
		t.Fatalf("failed to get next: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries", len(entries))
	}
	expected = []Entry{
		{Id: 4, PayloadType: "4", Payload: []byte("444")},
	}
	if !reflect.DeepEqual(entries, expected) {
		t.Fatalf("unexpected entries: %v", entries)
	}
}

func TestQueueFailAndRetry(t *testing.T) {
	startTime := time.UnixMilli(0)
	now := newMockNowGetter(startTime)
	q := NewtTestStorage(now, t)

	// Retry delays: 1s, 2s, 2s, ...
	q.SetBaseRetryDelay(1 * time.Second)
	q.SetMaxRetryDelay(2 * time.Second)
	q.SetDelayMultiplier(2)

	// Enqueue one item
	payloadType := "test"
	payload := []byte("retry me")
	if err := q.Enqueue(payloadType, payload); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	entries, err := q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: %d", len(entries))
	}
	entryId := entries[0].Id
	// Fail once (should set available_at into the future)
	if err := q.Fail(entryId, nil); err != nil {
		t.Fatalf("fail failed: %v", err)
	}

	// Should not appear in next() immediately (retry delay)
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries: %d", len(entries))
	}

	// Wait one baseDelay
	now.Set(startTime.Add(time.Second))
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("expected job after delay, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: %d", len(entries))
	}

	// Fail again. Should only be available after two base delays now
	if err := q.Fail(entryId, nil); err != nil {
		t.Fatalf("fail failed: %v", err)
	}
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries: %d", len(entries))
	}
	// Wait 3 base delays since the start
	now.Set(startTime.Add(3 * time.Second))
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("expected job after delay, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: %d", len(entries))
	}

	// Fail again. Should "cap" delay at 2s
	if err := q.Fail(entryId, nil); err != nil {
		t.Fatalf("fail failed: %v", err)
	}
	// 1s, 2s, 2s = 5s delay since the start
	now.Set(startTime.Add(5 * time.Second))
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("expected job after delay, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: %d", len(entries))
	}
}

func TestQueueDeadLetter(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)
	q.SetDelayMultiplier(0)
	q.SetMaxNumberOfRetries(2)  // after 2 fails goes to dead-letter
	q.SetOnDeadLetterRetries(2) // tries running the onDeadLetter 2 times
	q.SetOnDeadLetterRetryDelay(5 * time.Millisecond)

	// Mock onDeadLetterHandler that fails once and succeeds on second try
	mockOnDeadLetterCalls := 0
	mockOnDeadLetter := func(p []byte) error {
		mockOnDeadLetterCalls += 1
		if mockOnDeadLetterCalls == 1 {
			return errors.New("BOOM")
		}
		return nil
	}

	if err := q.Enqueue("t", []byte("x")); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	entries, err := q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: err=%v", err)
	}
	e1 := entries[0]

	// Fail twice: should move to dead letter after the second fail
	if err := q.Fail(e1.Id, mockOnDeadLetter); err != nil {
		t.Fatalf("fail1: %v", err)
	}
	if err := q.Fail(e1.Id, mockOnDeadLetter); err != nil {
		t.Fatalf("fail2: %v", err)
	}
	if mockOnDeadLetterCalls != 2 {
		t.Fatalf("mockOnDeadLetterCalls=%d, expected 2", mockOnDeadLetterCalls)
	}
	// Now Next() must notthing
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: err=%v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries: %d", len(entries))
	}

	// Now Next() must not return the same id again
	// Easiest black-box check: try to enqueue + check that old id never returns.
	if err := q.Enqueue("smoke", []byte("x")); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	e2 := entries[0]
	if e2.Id == e1.Id {
		t.Fatalf("expected old id %d to be gone (dead-letter), but Next() returned it", e1.Id)
	}
}

func TestOnDeadLetterDoesntRetryForever(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)
	q.SetDelayMultiplier(0)
	q.SetMaxNumberOfRetries(2)  // after 2 fails goes to dead-letter
	q.SetOnDeadLetterRetries(2) // onDeadLetter is only called twice
	q.SetOnDeadLetterRetryDelay(2 * time.Millisecond)

	// Mock onDeadLetterHandler that always fails
	mockOnDeadLetter := func(p []byte) error {
		return errors.New("BOOM")
	}

	if err := q.Enqueue("t", []byte("x")); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	entries, err := q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: err=%v", err)
	}
	e1 := entries[0]

	// Fail twice: should move to dead letter after the second fail.
	// The "onDeadLetter" will always fail; but we stop retrying after a while.
	if err := q.Fail(e1.Id, mockOnDeadLetter); err != nil {
		t.Fatalf("fail1: %v", err)
	}
	if err := q.Fail(e1.Id, mockOnDeadLetter); err != nil {
		t.Fatalf("fail2: %v", err)
	}

	itemsIter, closeItems, err := q.ts.GetAllDeadLetter()
	if err != nil {
		t.Fatal(err)
	}
	defer closeItems()
	items, err := iterator.GetFirstN(100, itemsIter)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("%d deadLetter items, expected 1", len(items))
	}
}

func TestGetAllQueued(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)
	// enqueue multiple items
	if err := q.Enqueue("a", []byte("payload-a")); err != nil {
		t.Fatalf("enqueue a failed: %v", err)
	}
	if err := q.Enqueue("b", []byte("payload-b")); err != nil {
		t.Fatalf("enqueue b failed: %v", err)
	}
	it, unlock, err := q.ts.GetAllQueued()
	if err != nil {
		t.Fatalf("GetAllQueued failed: %v", err)
	}
	defer unlock()
	var items []QueueItem
	for it.Next() {
		item, err := it.Get()
		if err != nil {
			t.Fatalf("iterator.Get failed: %v", err)
		}

		items = append(items, item)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterator.Err failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	unlock()

	// First item
	if items[0].Id != 1 {
		t.Errorf("expected first Id to be 1, got %d", items[0].Id)
	}
	if items[0].PayloadType != "a" {
		t.Errorf("expected first PayloadType 'a', got %q", items[0].PayloadType)
	}
	if string(items[0].Payload) != "payload-a" {
		t.Errorf("expected first Payload 'payload-a', got %q", string(items[0].Payload))
	}
	// Second item
	if items[1].Id != 2 {
		t.Errorf("expected second Id to be 2, got %d", items[1].Id)
	}
	if items[1].PayloadType != "b" {
		t.Errorf("expected second PayloadType 'b', got %q", items[1].PayloadType)
	}
	if string(items[1].Payload) != "payload-b" {
		t.Errorf("expected second Payload 'payload-b', got %q", string(items[1].Payload))
	}
}

func TestGetAllQueued_EmptyQueue(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)

	it, unlock, err := q.ts.GetAllQueued()
	if err != nil {
		t.Fatalf("GetAllQueued failed: %v", err)
	}
	defer unlock()
	if it.Next() {
		t.Fatalf("expected no items, but iterator.Next() returned true")
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterator.Err failed: %v", err)
	}
}

func TestGetAllDeadLetter(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)
	q.SetDelayMultiplier(0)
	q.SetMaxNumberOfRetries(2)

	// enqueue 2 and force them to dead-letter
	// 1
	if err := q.Enqueue("dead-1", []byte("letter-1")); err != nil {
		t.Fatalf("enqueue 1 failed: %v", err)
	}
	entries, err := q.Next(0, 10)
	if err != nil {
		t.Fatalf("next 1 failed: %v", err)
	}
	e1 := entries[0]
	if err := q.Fail(e1.Id, nil); err != nil {
		t.Fatalf("fail 1.1 failed: %v", err)
	}
	if err := q.Fail(e1.Id, nil); err != nil {
		t.Fatalf("fail 1.2 failed: %v", err)
	}
	// 2
	if err := q.Enqueue("dead-2", []byte("letter-2")); err != nil {
		t.Fatalf("enqueue 2 failed: %v", err)
	}
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("next 2 failed: %v", err)
	}
	e2 := entries[0]
	if err := q.Fail(e2.Id, nil); err != nil {
		t.Fatalf("fail 2.1 failed: %v", err)
	}
	if err := q.Fail(e2.Id, nil); err != nil {
		t.Fatalf("fail 2.2 failed: %v", err)
	}
	// Get them in expected order
	it, unlock, err := q.ts.GetAllDeadLetter()
	if err != nil {
		t.Fatalf("GetAllDeadLetter failed: %v", err)
	}
	defer unlock()
	items, err := iterator.GetFirstN(100, it)
	if err != nil {
		t.Fatalf("iterator.Err failed: %v", err)
	}
	unlock()
	if len(items) != 2 {
		t.Fatalf("expected 2 dead-letter items, got %d", len(items))
	}
	// Expects 2 to come first since it is the newest to enter DeadLetter.
	// 2
	if items[0].PayloadType != "dead-2" {
		t.Errorf("expected payloadType 'dead-2', got %q", items[0].PayloadType)
	}
	if string(items[0].Payload) != "letter-2" {
		t.Errorf("unexpected payload: %q", items[0].Payload)
	}
	// 1
	if items[1].PayloadType != "dead-1" {
		t.Errorf("expected payloadType 'dead-1', got %q", items[1].PayloadType)
	}
	if string(items[1].Payload) != "letter-1" {
		t.Errorf("unexpected payload: %q", items[1].Payload)
	}
}

func TestGetAllDeadLetter_Empty(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)

	it, unlock, err := q.ts.GetAllDeadLetter()
	if err != nil {
		t.Fatalf("GetAllDeadLetter failed: %v", err)
	}
	defer unlock()
	if it.Next() {
		t.Fatalf("expected no dead-letter items")
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterator.Err failed: %v", err)
	}
}

func TestRequeueDeadLetter(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)
	q.SetDelayMultiplier(0)
	q.SetMaxNumberOfRetries(1)

	// enqueue -> fail -> dead-letter
	if err := q.Enqueue("dead", []byte("letter")); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	entries, err := q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	e := entries[0]
	if err := q.Fail(e.Id, nil); err != nil {
		t.Fatalf("fail failed: %v", err)
	}

	// requeue
	if err := q.ts.RequeueDeadLetter(1); err != nil {
		t.Fatalf("RequeueDeadLetter failed: %v", err)
	}

	// dead-letter must now be empty
	it2, unlock, err := q.ts.GetAllDeadLetter()
	if err != nil {
		t.Fatalf("GetAllDeadLetter failed: %v", err)
	}
	defer unlock()
	if it2.Next() {
		t.Fatalf("dead-letter should be empty after requeue")
	}
	unlock()

	// payload must be back in queue
	entries, err = q.Next(0, 10)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	newE := entries[0]
	if newE.PayloadType != "dead" {
		t.Fatalf("unexpected payloadType: %q", newE.PayloadType)
	}
	if string(newE.Payload) != "letter" {
		t.Fatalf("unexpected payload: %q", newE.Payload)
	}
	if newE.Id == 1 {
		t.Fatalf("expected new queue id, got same id %d", newE.Id)
	}
}

func TestRequeueDeadLetter_NotFound(t *testing.T) {
	q := NewtTestStorage( /*NowGetter=*/ nil, t)

	err := q.ts.RequeueDeadLetter(999)
	if err == nil {
		t.Fatalf("expected error for non-existing dead-letter id")
	}
}

type mockNowGetter struct {
	now time.Time
}

func newMockNowGetter(startTime time.Time) *mockNowGetter {
	return &mockNowGetter{
		now: startTime,
	}
}
func (n mockNowGetter) Now() time.Time {
	return n.now
}
func (n *mockNowGetter) Set(t time.Time) {
	n.now = t
}
