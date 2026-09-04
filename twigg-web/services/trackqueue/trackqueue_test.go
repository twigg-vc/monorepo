package trackqueue

import (
	"context"
	"encoding/json"
	"errors"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/webdb"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestGetAndPutLimits(t *testing.T) {
	q, _, _, db := setupTest(t)
	q.Start()
	defer q.Stop()

	// If not set, the default values are returned
	checkLimits(1, defaultMaxRunningJobs, defaultMaxRunningTimeoutMs, q, db, t)
	checkLimits(2, defaultMaxRunningJobs, defaultMaxRunningTimeoutMs, q, db, t)
	checkLimits(3, defaultMaxRunningJobs, defaultMaxRunningTimeoutMs, q, db, t)

	const maxJobs = 2
	const maxTimeout = 100 * time.Millisecond
	putLimit(1, maxJobs, maxTimeout, q, db, t)
	putLimit(2, maxJobs, maxTimeout, q, db, t)
	// Once set, only the modified values change
	checkLimits(1, maxJobs, maxTimeout.Milliseconds(), q, db, t)
	checkLimits(2, maxJobs, maxTimeout.Milliseconds(), q, db, t)
	checkLimits(3, defaultMaxRunningJobs, defaultMaxRunningTimeoutMs, q, db, t)
}

func TestLimitOfTwoJobs(t *testing.T) {
	q, js, tc, db := setupTest(t)
	q.Start()
	defer q.Stop()

	const maxJobs = 2
	const maxTimeout = 100 * time.Millisecond

	putLimit(1, maxJobs, maxTimeout, q, db, t)
	putLimit(2, maxJobs, maxTimeout, q, db, t)

	// Only 2 jobs per user can run in parallel
	putJob(1, "user1job1", runnerlib.JobPayload{Name: "user1job1"}, q, db, t)
	putJob(1, "user1job2", runnerlib.JobPayload{Name: "user1job2"}, q, db, t)
	putJob(1, "user1job3", runnerlib.JobPayload{Name: "user1job3"}, q, db, t)
	putJob(2, "user2job1", runnerlib.JobPayload{Name: "user2job1"}, q, db, t)
	putJob(2, "user2job2", runnerlib.JobPayload{Name: "user2job2"}, q, db, t)

	waitAndCheckJobsPublishedEquals(q, tc, []string{
		"user1job1", "user1job2", "user2job1", "user2job2"})
	js.checkJobIsPosted("user1job1")
	js.checkJobIsPosted("user1job2")
	js.checkJobIsPosted("user2job1")
	js.checkJobIsPosted("user2job2")

	callPutJobFinished("user1job2", q, db, t)
	waitAndCheckJobsPublishedEquals(q, tc, []string{
		"user1job1", "user1job2", "user1job3", "user2job1", "user2job2"})

	js.checkJobIsPosted("user1job3")
}

func TestLimitOf100msTimeout(t *testing.T) {
	q, js, tc, db := setupTest(t)
	q.Start()
	defer q.Stop()
	const maxJobs = 2
	const maxTimeout = 100 * time.Millisecond
	putLimit(1, maxJobs, maxTimeout, q, db, t)
	putLimit(2, maxJobs, maxTimeout, q, db, t)

	// Only 100ms per user can be scheduled in parallel
	putJob(1, "user1job1", runnerlib.JobPayload{Name: "user1job1", TimeoutMilliSeconds: 10}, q, db, t)
	putJob(1, "user1job2", runnerlib.JobPayload{Name: "user1job2", TimeoutMilliSeconds: 200}, q, db, t)
	putJob(1, "user1job3", runnerlib.JobPayload{Name: "user1job3", TimeoutMilliSeconds: 100}, q, db, t)
	putJob(2, "user2job1", runnerlib.JobPayload{Name: "user2job1", TimeoutMilliSeconds: 200}, q, db, t)
	putJob(2, "user2job2", runnerlib.JobPayload{Name: "user2job2", TimeoutMilliSeconds: 100}, q, db, t)

	// user1: 10ms + 200ms = 210ms
	// user2: 200ms
	waitAndCheckJobsPublishedEquals(q, tc, []string{
		"user1job1", "user1job2", "user2job1"})
	js.checkJobIsPosted("user1job1")
	js.checkJobIsPosted("user1job2")
	js.checkJobIsPosted("user2job1")

	// finish user1job1 -> no ectra job enqueued
	callPutJobFinished("user1job1", q, db, t)
	// user1: 200ms = 200ms
	// user2: 200ms
	waitAndCheckJobsPublishedEquals(q, tc, []string{
		"user1job1", "user1job2", "user2job1"})

	// finish other jobs
	// user1: 0ms
	// user2: 0ms
	callPutJobFinished("user1job2", q, db, t)
	callPutJobFinished("user2job1", q, db, t)
	waitAndCheckJobsPublishedEquals(q, tc, []string{
		"user1job1", "user1job2", "user1job3", "user2job1", "user2job2"})
	js.checkJobIsPosted("user1job3")
}

func TestJanitor(t *testing.T) {
	q, js, tc, db := setupTest(t)
	q.Start()
	defer q.Stop()

	const maxJobs = 2
	const maxTimeout = 100 * time.Millisecond
	putLimit(1, maxJobs, maxTimeout, q, db, t)

	// Only 100ms per user can be scheduled in parallel,
	// so only one will run at a time
	putJob(1, "user1job1", runnerlib.JobPayload{Name: "user1job1", TimeoutMilliSeconds: 200}, q, db, t)
	putJob(1, "user1job2", runnerlib.JobPayload{Name: "user1job2", TimeoutMilliSeconds: 200}, q, db, t)
	waitAndCheckJobsPublishedEquals(q, tc, []string{"user1job1"})
	js.checkJobIsPosted("user1job1")

	// Mock that the job succeeded, but don't call PutJobFinished.
	// The janitor should eventually call the track client and call PutJobFinished
	tc.mockJobSuccess("user1job1")
	waitAndCheckJobsPublishedEquals(q, tc, []string{"user1job1", "user1job2"})
	js.checkJobIsPosted("user1job1")
	js.checkJobIsPosted("user1job2")
}

type fakeTrackClient struct {
	t             *testing.T
	payloadsById  map[string]runnerlib.JobPayload
	trackJobsById map[string]trackclient.TrackJob
}

func (tc *fakeTrackClient) Get(jobId string) (tj trackclient.TrackJob, pl runnerlib.JobPayload, isNotFoundErr bool, err error) {
	tj, ok := tc.trackJobsById[jobId]
	if !ok {
		isNotFoundErr = true
		err = errors.New("not found")
		return
	}
	pl = tc.payloadsById[jobId]
	return
}
func (tc *fakeTrackClient) Put(jobId string, jobPayload runnerlib.JobPayload) error {
	_, ok := tc.payloadsById[jobId]
	if ok {
		return nil
	}
	jobPayloadBytes, _ := json.Marshal(jobPayload)
	tc.payloadsById[jobId] = jobPayload
	tc.trackJobsById[jobId] = trackclient.TrackJob{
		Id:              jobId,
		Payload:         jobPayloadBytes,
		Status:          trackclient.TrackJobStatusQueued,
		CreatedAtMillis: time.Now().UnixMilli(),
	}
	return nil
}
func (tc *fakeTrackClient) mockJobSuccess(jobId string) {
	j := tc.trackJobsById[jobId]
	if j.Status != trackclient.TrackJobStatusQueued {
		panic("invalid call of mockJobSuccess")
	}
	j.Status = trackclient.TrackJobStatusSuccess
	j.FinalDurationMillis = time.Now().UnixMilli()
	tc.trackJobsById[jobId] = j
}

type fakeJobsStorage struct {
	t            *testing.T
	postedJobIds map[string]bool
}

func (js *fakeJobsStorage) SetToPosted(wl context.Context, id string) error {
	js.postedJobIds[id] = true
	return nil
}
func (js *fakeJobsStorage) checkJobIsPosted(id string) {
	_, ok := js.postedJobIds[id]
	if !ok {
		js.t.Fatalf("jobId %s is not posted", id)
	}
}
func (js *fakeJobsStorage) checkJobIsNotPosted(id string) {
	_, ok := js.postedJobIds[id]
	if ok {
		js.t.Fatalf("jobId %s is posted", id)
	}
}

// helper to verify that exactly n jobs will be published.
// note that this waits until the queue actually does an extra loop
func waitAndCheckJobsPublishedEquals(q TrackQueue, tc *fakeTrackClient, names []string) {
	// Wait for paylods to be put
	start := time.Now()
	for len(tc.payloadsById) < len(names) {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			tc.t.Fatal("waited too long for job to be put")
		}
	}
	// Wait for one extra loop to make sure it wont increase
	obs := loopCounter{}
	q.AddObserver(&obs)
	for obs.loops < 1 {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			tc.t.Fatal("waited too long for loop")
		}
	}
	if len(tc.payloadsById) != len(names) {
		tc.t.Fatalf("expected %d payloads got %d", len(names), len(tc.payloadsById))
	}
	gotNames := []string{}
	for _, pl := range tc.payloadsById {
		gotNames = append(gotNames, pl.Name)
	}
	slices.Sort(names)
	slices.Sort(gotNames)
	if !reflect.DeepEqual(names, gotNames) {
		tc.t.Fatalf("expected job names %v got %v", names, gotNames)
	}
}

// helper to call PutJobFinished
func callPutJobFinished(jobId string, q TrackQueue, db webdb.WebDb, t *testing.T) {
	w, close, commit, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	// Call twice just to check idempotency
	err = q.PutJobFinished(jobId, w)
	if err != nil {
		t.Fatal(err)
	}
	err = q.PutJobFinished(jobId, w)
	if err != nil {
		t.Fatal(err)
	}
	err = commit()
	if err != nil {
		t.Fatal(err)
	}
}

func checkLimits(ownerId int64, expectedMaxJobs int, expectedMaxJobsTimeoutMs int64, q TrackQueue, db webdb.WebDb, t *testing.T) {
	r, closeR, err := db.BeginRead()
	if err != nil {
		t.Fatal(err)
	}
	defer closeR()

	maxJobs, maxTimeout, err := q.GetLimits(ownerId, r)
	if err != nil {
		t.Fatal(err)
	}
	if maxJobs != expectedMaxJobs {
		t.Fatalf("expected ownerId=%d maxJobs=%d got=%d", ownerId, expectedMaxJobs, maxJobs)
	}
	if maxTimeout.Milliseconds() != expectedMaxJobsTimeoutMs {
		t.Fatalf("expected ownerId=%d maxTimeoutMs=%d got=%d", ownerId, expectedMaxJobsTimeoutMs, maxTimeout.Milliseconds())
	}
}

func putLimit(ownerId int64, maxJobs int, maxJobsTimeout time.Duration, q TrackQueue, db webdb.WebDb, t *testing.T) {
	w, close, commit, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	err = q.PutLimits(ownerId, maxJobs, maxJobsTimeout, w)
	if err != nil {
		t.Fatal(err)
	}
	err = commit()
	if err != nil {
		t.Fatal(err)
	}
}

// helper to put a job to the trackqueue
func putJob(ownerId int64, jobId string, pl runnerlib.JobPayload, q TrackQueue, db webdb.WebDb, t *testing.T) {
	w, close, commit, err := db.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	err = q.Put(ownerId, jobId, pl, w)
	if err != nil {
		t.Fatal(err)
	}
	err = commit()
	if err != nil {
		t.Fatal(err)
	}
}

// Initializes and sets up cleanups for most important entities
func setupTest(t *testing.T) (TrackQueue, *fakeJobsStorage, *fakeTrackClient, webdb.WebDb) {
	t.Helper()
	lDb, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	tc := &fakeTrackClient{
		t:             t,
		payloadsById:  map[string]runnerlib.JobPayload{},
		trackJobsById: map[string]trackclient.TrackJob{},
	}
	js := &fakeJobsStorage{
		t:            t,
		postedJobIds: map[string]bool{},
	}
	w, close, commit, err := lDb.BeginWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	q, err := New(js, tc, w, lDb,
		WithPoolInternal(10*time.Millisecond),        // Polls every 10 ms
		WithJanitorPoolInterval(20*time.Millisecond), // Polls track every 20ms
	)
	if err != nil {
		t.Fatal(err)
	}
	err = commit()
	if err != nil {
		t.Fatal(err)
	}
	return q, js, tc, lDb
}

type loopCounter struct {
	loops        int
	janitorLoops int
}

func (l *loopCounter) OnLoop() {
	l.loops += 1
}
func (l *loopCounter) OnJanitorLoop() {
	l.janitorLoops += 1
}