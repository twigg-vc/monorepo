package db_test

import (
	"errors"
	"monorepo/twigg-track/db"
	"monorepo/twigg-track/trackclient"
	"reflect"
	"testing"
	"time"
)

func newInMemoryDb(t *testing.T) db.Sqlite {
	s, err := db.NewSqlite(db.InMemoryPathToDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInitInTmpDir(t *testing.T) {
	s, err := db.NewSqlite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Init(); err != nil {
		t.Fatal(err)
	}
}

func TestCantPassReadCtxAgain(t *testing.T) {
	s := newInMemoryDb(t)
	readCtx, closeTx, err := s.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	if _, _, err = s.BeginReadWithCtx(readCtx); err == nil {
		t.Fatal("expected error passing a context that already has a read tx")
	}
	if _, _, _, err = s.BeginWriteWithCtx(readCtx); err == nil {
		t.Fatal("expected error passing a read ctx to BeginWrite")
	}
}

func TestCantPassWriteCtxAgain(t *testing.T) {
	s := newInMemoryDb(t)
	writeCtx, closeTx, _, err := s.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	if _, _, err = s.BeginReadWithCtx(writeCtx); err == nil {
		t.Fatal("expected error passing a write ctx to BeginRead")
	}
	if _, _, _, err = s.BeginWriteWithCtx(writeCtx); err == nil {
		t.Fatal("expected error passing a context that already has a write tx")
	}
}

func TestCreateSetStatusAndGet(t *testing.T) {
	db := newInMemoryDb(t)
	tx, closeTx, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	jobID := "job-123"
	payload := []byte("hello world")
	const skipWebhooks = false

	// -- Create --
	job, err := db.Create(tx, jobID, payload, skipWebhooks)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if job.Id != jobID {
		t.Fatalf("id mismatch: %q", job.Id)
	}
	if job.Status != trackclient.TrackJobStatusQueued {
		t.Fatalf("status mismatch: %q", job.Status)
	}
	if string(job.Payload) != string(payload) {
		t.Fatalf("payload mismatch")
	}
	if job.CreatedAtMillis == 0 {
		t.Fatalf("createdAt not set")
	}
	if job.SkipWebhooks != skipWebhooks {
		t.Fatalf("skipWebhooks mismatch")
	}

	// ---- Get ----
	got, isNotFoundErr, err := db.Get(tx, jobID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if isNotFoundErr {
		t.Fatalf("Get returned isNotFoundErr")
	}
	if !reflect.DeepEqual(got, job) {
		t.Fatalf("Get returned different job:\n%+v\n%+v", got, job)
	}
	_, isNotFoundErr, err = db.Get(tx, "non existing")
	if !isNotFoundErr || err == nil {
		t.Fatalf("Got non isNotFoundErr: %v %s", isNotFoundErr, err)
	}
	exists, err := db.Exists(tx, jobID)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatalf("got not exists")
	}

	// ---- SetStatus ----
	execTime := int64(300)
	err = db.SetStatus(tx, jobID, trackclient.TrackJobStatusSuccess, execTime)
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	updated, _, err := db.Get(tx, jobID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.Status != trackclient.TrackJobStatusSuccess {
		t.Fatalf("status not updated: %q", updated.Status)
	}
	if updated.CreatedAtMillis != job.CreatedAtMillis {
		t.Fatalf("createdAt should not change")
	}
	if updated.FinalDurationMillis != execTime {
		t.Fatalf("createdAt should not change")
	}

	// ---- Verify persisted ----
	got2, _, err := db.Get(tx, jobID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got2.Status != trackclient.TrackJobStatusSuccess {
		t.Fatalf("status not persisted")
	}
}

func TestCreateSetStatusAndGet_skipWebhooks(t *testing.T) {
	db := newInMemoryDb(t)
	tx, closeTx, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	jobID := "job-123"
	const skipWebhooks = true

	job, err := db.Create(tx, jobID, []byte("payload"), skipWebhooks)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !job.SkipWebhooks {
		t.Fatalf("got SkipWebhooks=false")
	}

	got, _, err := db.Get(tx, jobID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(got, job) {
		t.Fatalf("Get returned different job:\n%+v\n%+v", got, job)
	}
}

func TestIsNotFoundErr(t *testing.T) {
	db_ := newInMemoryDb(t)
	tx, closeTx, _, err := db_.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	exists, err := db_.Exists(tx, "non existing")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Fatalf("got exists")
	}

	_, isNotFoundErr, err := db_.Get(tx, "non existing")
	if !isNotFoundErr || !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("Got non isNotFoundErr: %v %s", isNotFoundErr, err)
	}
}

func TestTooLargePayload(t *testing.T) {
	db_ := newInMemoryDb(t)
	tx, closeTx, _, err := db_.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	// Access the internal max payload just for this test
	db_.SetMaxPayloadLen(2)

	okPayload := make([]byte, 2)
	tooLargePayload := make([]byte, 3)
	const skipWebhooks = false
	_, err = db_.Create(tx, "1", okPayload, skipWebhooks)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db_.Create(tx, "2", tooLargePayload, skipWebhooks)
	if !errors.Is(err, db.ErrPayloadTooBig) {
		t.Fatalf("no error creating too large payload")
	}
}

func TestBestEffortCancels(t *testing.T) {
	db := newInMemoryDb(t)
	tx, closeTx, _, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer closeTx()

	ok, err := db.BestEffortCancelationWasRequested(tx, "non-existing-job")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("cancel not requested for non-existing-job")
	}

	err = db.RequestBestEffortCancelation(tx, "job1")
	if err != nil {
		t.Fatal(err)
	}
	err = db.RequestBestEffortCancelation(tx, "job2")
	if err != nil {
		t.Fatal(err)
	}
	err = db.CleanupBestEffortCancelation(tx, 1*time.Hour, t) // shouldnt change anything
	if err != nil {
		t.Fatal(err)
	}
	ok, err = db.BestEffortCancelationWasRequested(tx, "job1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("cancel not requested for job 1")
	}
	ok, err = db.BestEffortCancelationWasRequested(tx, "job2")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("cancel not requested for job2")
	}

	// Test cleanup
	time.Sleep(2 * time.Millisecond)
	err = db.CleanupBestEffortCancelation(tx, time.Millisecond, t)
	if err != nil {
		t.Fatal(err)
	}
	ok, err = db.BestEffortCancelationWasRequested(tx, "job1")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("job1 not cleaned up")
	}
}
