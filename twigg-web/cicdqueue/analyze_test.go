package cicdqueue

import (
	"context"
	"errors"
	"monorepo/squeue"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/webdb"
	"slices"
	"testing"
	"time"
)

func TestAnalyzeHappyPath(t *testing.T) {
	// Initialize all the dependencies and the service
	queueStorage := squeue.NewtTestStorage(nil, t)
	queueRunner := squeue.NewRunner(queueStorage, 10*time.Millisecond, 2)
	obs := runnerObserver{}
	queueRunner.AddObserver(&obs)
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	js := newMockJobStorage()
	ci := mockCiCdJobsPublisher{}
	srv, err := New(js, &ci, db, queueRunner)
	if err != nil {
		t.Fatal(err)
	}

	// Start the queue runner
	queueRunner.Start()
	defer queueRunner.Stop()

	// Enqueue a run
	w, closeW, commit, err := db.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	const (
		repoId        = 0
		commitId      = 1
		commitVersion = 1
	)
	runNumber, err := srv.EnqueueCiCdRun(repoId, commitId, commitVersion, runnerlib.OnPush, w)
	if err != nil {
		t.Fatal(err)
	}
	if runNumber != 0 {
		t.Fatalf("first runNumber: %d", runNumber)
	}
	// The queue will actually run the cicd only after commiting and closing
	err = commit()
	if err != nil {
		t.Fatal(err)
	}
	closeW()
	start := time.Now()
	for obs.success != 1 {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 10*time.Second {
			t.Fatal("waited too long")
		}
	}
	ci.checkCiCdRunWasPut(repoId, commitId, commitVersion, runNumber, t)

	r, closeR, err := db.BeginRead()
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	st, err := srv.GetCiCdLatestRunStatus(repoId, commitId, commitVersion, r)
	if err != nil {
		t.Fatal(err)
	}
	if st != CiCdStatusStarted {
		t.Fatalf("unexpected status: %s", st)
	}
}

func TestPrepareButDontCommitAnalysis(t *testing.T) {
	// Initialize all the dependencies and the service
	queueStorage := squeue.NewtTestStorage(nil, t)
	queueRunner := squeue.NewRunner(queueStorage, 10*time.Millisecond, 2)
	obs := runnerObserver{}
	queueRunner.AddObserver(&obs)
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	ci := mockCiCdJobsPublisher{}
	js := newMockJobStorage()
	srv, err := New(js, &ci, db, queueRunner)
	if err != nil {
		t.Fatal(err)
	}

	// Start the queue runner
	queueRunner.Start()
	defer queueRunner.Stop()

	// Prepare an analysis but fail to commit it
	w, closeW, _, err := db.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	const (
		repoId        = 0
		commitId      = 1
		commitVersion = 1
	)
	_, err = srv.EnqueueCiCdRun(repoId, commitId, commitVersion, runnerlib.OnPush, w)
	if err != nil {
		t.Fatal(err)
	}
	if obs.err != 0 {
		t.Fatal("expected zero errors")
	}
	// Dont commit the lock. The Queue will only error
	closeW()

	if obs.success != 0 {
		t.Fatal("expected no successes")
	}
	// Expect error count to increase
	originalErrCount := obs.err
	start := time.Now()
	for obs.err == originalErrCount {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 10*time.Second {
			t.Fatal("waited too long")
		}
	}
}

func TestRunNumberIncreases(t *testing.T) {
	queueStorage := squeue.NewtTestStorage(nil, t)
	queueRunner := squeue.NewRunner(queueStorage, 10*time.Millisecond, 2)
	obs := runnerObserver{}
	queueRunner.AddObserver(&obs)
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	js := newMockJobStorage()
	ci := mockCiCdJobsPublisher{}
	srv, err := New(js, &ci, db, queueRunner)
	if err != nil {
		t.Fatal(err)
	}

	// Start the queue runner
	queueRunner.Start()
	defer queueRunner.Stop()

	// Run two CI/CD and vheck that the runnumber increases
	w, closeW, commit, err := db.BeginWrite()
	defer closeW()
	if err != nil {
		t.Fatal(err)
	}
	const (
		repoId        = 0
		commitId      = 1
		commitVersion = 1
	)
	runNumber, err := srv.EnqueueCiCdRun(repoId, commitId, commitVersion, runnerlib.OnPush, w)
	if err != nil {
		t.Fatal(err)
	}
	if runNumber != 0 {
		t.Fatalf("expected run number 0 got %d", runNumber)
	}
	runNumber, err = srv.EnqueueCiCdRun(repoId, commitId, commitVersion, runnerlib.OnPush, w)
	if err != nil {
		t.Fatal(err)
	}
	if runNumber != 1 {
		t.Fatalf("expected run number 1 got %d", runNumber)
	}
	err = commit()
	if err != nil {
		t.Fatal(err)
	}
	closeW()
	start := time.Now()
	for obs.success != 2 {
		time.Sleep(50 * time.Millisecond)
		if time.Since(start) > 10*time.Second {
			t.Fatal("waited too long")
		}
	}
	ci.checkCiCdRunWasPut(repoId, commitId, commitVersion /*runNumber*/, 0, t)
	ci.checkCiCdRunWasPut(repoId, commitId, commitVersion /*runNumber*/, 1, t)
}

func TestEnqueueCdStage(t *testing.T) {
	queueStorage := squeue.NewtTestStorage(nil, t)
	queueRunner := squeue.NewRunner(queueStorage, 10*time.Millisecond, 2)
	obs := runnerObserver{}
	queueRunner.AddObserver(&obs)
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	ci := mockCiCdJobsPublisher{}
	js := newMockJobStorage()
	srv, err := New(js, &ci, db, queueRunner)
	if err != nil {
		t.Fatal(err)
	}

	// Mock that a stage 0 of a mock pipeline is done
	pipelineId := "mock-pipeline-id"
	js.setStageToDone(pipelineId, 0)
	// Enqueue the next stage
	err = srv.ResumeCdToStage(pipelineId, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Start the queue and wait
	queueRunner.Start()
	defer queueRunner.Stop()
	for obs.err+obs.success == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	// Pipeline should have resumed
	ci.checkPipelineResumed(pipelineId, t)

	// Enqueue to run stage of a pipeline but don't mark its stage 0 as complete
	originalEventsCount := obs.err + obs.success
	err = srv.ResumeCdToStage("mock-pipeline-with-stage-0-not-done", 1)
	if err != nil {
		t.Fatal(err)
	}
	// Wait for a queue run
	for obs.err+obs.success == originalEventsCount {
		time.Sleep(50 * time.Millisecond)
	}
	ci.checkPipelineDidntResume("mock-pipeline-with-stage-0-not-done", t)
}

type mockJobStorage struct {
	pipelineStagesDone map[string][]int32
}

func newMockJobStorage() *mockJobStorage {
	return &mockJobStorage{
		pipelineStagesDone: map[string][]int32{},
	}
}

func (js *mockJobStorage) CanPutResumePipelineToStage(tx context.Context, pipelineId string, stage int32) (bool, error) {
	if js.pipelineStagesDone == nil {
		js.pipelineStagesDone = map[string][]int32{}
	}
	stagesDone, ok := js.pipelineStagesDone[pipelineId]
	if !ok {
		return false, errors.New("not found")
	}
	return slices.Contains(stagesDone, stage-1), nil
}
func (js *mockJobStorage) setStageToDone(pipelineId string, stage int32) {
	stagesDone := js.pipelineStagesDone[pipelineId]
	stagesDone = append(stagesDone, stage)
	js.pipelineStagesDone[pipelineId] = stagesDone
}

type mockCiCdJobsPublisher struct {
	ciCdRuns         []ciCdRun
	resumedPipelines []string
}

func (ci *mockCiCdJobsPublisher) PutAutoCiCdRun(repoId, commitId, commitVersion uint64,
	runNumber int64, trigger runnerlib.JobTrigger, w context.Context) error {
	ci.ciCdRuns = append(ci.ciCdRuns, ciCdRun{repoId, commitId, commitVersion, runNumber})
	return nil
}
func (ci *mockCiCdJobsPublisher) PutResumePipelineWaitingStage(pipelineId string, atStage int32, w context.Context) error {
	ci.resumedPipelines = append(ci.resumedPipelines, pipelineId)
	return nil
}
func (ci mockCiCdJobsPublisher) checkCiCdRunWasPut(repoId, commitId, commitVersion uint64,
	runNumber int64, t *testing.T) {
	if !slices.Contains(ci.ciCdRuns, ciCdRun{
		repoId: repoId, commitId: commitId,
		commitVersion: commitVersion, runNumber: runNumber}) {
		t.Fatalf("ciCdRun was not put")
	}
}
func (ci mockCiCdJobsPublisher) checkPipelineResumed(pipelineId string, t *testing.T) {
	if !slices.Contains(ci.resumedPipelines, pipelineId) {
		t.Fatalf("pipelineId=%q did not resume", pipelineId)
	}
}
func (ci mockCiCdJobsPublisher) checkPipelineDidntResume(pipelineId string, t *testing.T) {
	if slices.Contains(ci.resumedPipelines, pipelineId) {
		t.Fatalf("pipelineId=%q resumed", pipelineId)
	}
}

type ciCdRun struct {
	repoId        uint64
	commitId      uint64
	commitVersion uint64
	runNumber     int64
}

type runnerObserver struct {
	success int
	err     int
}

func (o *runnerObserver) OnHandle(payloadType string, payload []byte, result error) {
	if result != nil {
		o.err += 1
	} else {
		o.success += 1
	}
}
func (o *runnerObserver) OnSleep() {}