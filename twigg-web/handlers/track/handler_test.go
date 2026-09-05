package track

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"monorepo/twigg-web/webdb"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"monorepo/base/iterator"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/services/sign"
	"monorepo/twigg-web/services/twiggtoken"
	"monorepo/twigg-web/wrappers"
)

var testSigner = sign.NewSigner([]byte("track-handler-test-key"))

// TODO: Add handleTrackWebhook test.

type fakeSecrets struct {
	secrets map[string]string
}

func (fs fakeSecrets) GetRepoIdSecret(rl context.Context, repoId uint64, secretName string) (string, bool, error) {
	if fs.secrets == nil {
		return "", true, errors.New("secret not found")
	}
	s, contains := fs.secrets[secretName]
	if contains {
		return s, false, nil
	}
	return "", true, errors.New("secret not found")
}

const testApiKey = "fake-api-key"

func TestHandleGetSecrets_OK(t *testing.T) {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	secrets := fakeSecrets{
		secrets: map[string]string{
			"secret-a": "secret-a-value",
			"secret-b": "secret-b-value",
		},
	}
	h := handler{
		db:      db,
		secrets: secrets,
	}

	q := url.Values{}
	q.Add(routes.RepoSecretNameParamName, "secret-a")
	q.Add(routes.RepoSecretNameParamName, "secret-b")
	req := httptest.NewRequest("GET", "/?"+q.Encode(), nil)

	rawTwToken, err := twiggtoken.NewTwiggToken(
		/*epoId, commitServerId, commitVersion*/ 1, 1, 1,
		[]twiggtoken.TokenAction{
			twiggtoken.TokenActionGetSecret,
			twiggtoken.TokenActionGetSecret,
		},
		[]string{"secret-a", "secret-b"},
		time.Hour,
		testSigner,
	)
	if err != nil {
		t.Fatal(err)
	}
	twToken, _, err := twiggtoken.ParseToken(rawTwToken, testSigner)
	if err != nil {
		t.Fatal(err)
	}

	r := wrappers.ServerKeyAndTokenAuthTrackMuxRequest{
		Request:    req,
		TwiggToken: twToken,
	}

	w := httptest.NewRecorder()

	h.handleGetSecrets(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", w.Code)
	}

	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		t.Fatal(err)
	}

	if resp["secret-a"] != "secret-a-value" {
		t.Fatalf("unexpected value for secret-a: %v", resp["secret-a"])
	}

	if resp["secret-b"] != "secret-b-value" {
		t.Fatalf("unexpected value for secret-b: %v", resp["secret-b"])
	}
}

func TestHandleGetSecrets_GetForUnsupportedSecret(t *testing.T) {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)
	secrets := fakeSecrets{
		secrets: map[string]string{
			"secret-a": "secret-a-value",
			"secret-b": "secret-b-value",
		},
	}
	h := handler{
		db:      db,
		secrets: secrets,
	}

	q := url.Values{}
	q.Add(routes.RepoSecretNameParamName, "secret-a") // Supports
	q.Add(routes.RepoSecretNameParamName, "secret-b") // Unsupported
	req := httptest.NewRequest("GET", "/?"+q.Encode(), nil)

	rawTwToken, err := twiggtoken.NewTwiggToken(
		/*epoId, commitServerId, commitVersion*/ 1, 1, 1,
		[]twiggtoken.TokenAction{
			twiggtoken.TokenActionGetSecret,
		},
		[]string{"secret-a"},
		time.Hour,
		testSigner,
	)
	if err != nil {
		t.Fatal(err)
	}
	twToken, _, err := twiggtoken.ParseToken(rawTwToken, testSigner)
	if err != nil {
		t.Fatal(err)
	}

	r := wrappers.ServerKeyAndTokenAuthTrackMuxRequest{
		Request:    req,
		TwiggToken: twToken,
	}

	w := httptest.NewRecorder()

	h.handleGetSecrets(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 got %d", w.Code)
	}

	// Check if the response is empty
	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	if err == nil {
		t.Fatal(err)
	}
}

func TestHandleGetSecrets_NoParams(t *testing.T) {
	h := handler{}

	req := httptest.NewRequest("GET", "/", nil)

	rawTwToken, err := twiggtoken.NewTwiggToken(
		/*epoId, commitServerId, commitVersion*/ 1, 1, 1,
		[]twiggtoken.TokenAction{
			twiggtoken.TokenActionGetSecret,
		},
		[]string{"secret-a"},
		time.Hour,
		testSigner,
	)
	if err != nil {
		t.Fatal(err)
	}
	twToken, _, err := twiggtoken.ParseToken(rawTwToken, testSigner)
	if err != nil {
		t.Fatal(err)
	}
	r := wrappers.ServerKeyAndTokenAuthTrackMuxRequest{
		Request:    req,
		TwiggToken: twToken,
	}

	w := httptest.NewRecorder()

	h.handleGetSecrets(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
	// Check if the response is empty
	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	if err == nil {
		t.Fatal(err)
	}
}

func TestHandleGetSecrets_SecretNotFound(t *testing.T) {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDb)

	secrets := fakeSecrets{
		secrets: map[string]string{
			"secret-a": "secret-a-value",
		},
	}

	h := handler{
		db:      db,
		secrets: secrets,
	}

	q := url.Values{}
	q.Add(routes.RepoSecretNameParamName, "secret-a")
	q.Add(routes.RepoSecretNameParamName, "secret-missing")
	req := httptest.NewRequest("GET", "/?"+q.Encode(), nil)

	rawTwToken, err := twiggtoken.NewTwiggToken(
		/*repoId, commitServerId, commitVersio*/ 1, 1, 1,
		[]twiggtoken.TokenAction{
			twiggtoken.TokenActionGetSecret,
			twiggtoken.TokenActionGetSecret,
		},
		[]string{"secret-a", "secret-missing"},
		time.Hour,
		testSigner,
	)
	if err != nil {
		t.Fatal(err)
	}

	twToken, _, err := twiggtoken.ParseToken(rawTwToken, testSigner)
	if err != nil {
		t.Fatal(err)
	}

	r := wrappers.ServerKeyAndTokenAuthTrackMuxRequest{
		Request:    req,
		TwiggToken: twToken,
	}

	w := httptest.NewRecorder()

	h.handleGetSecrets(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}

func TestHandlePipelinesSuccessOfTwoStages(t *testing.T) {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	mockWebhookParser := &mockWebhookParser{} // Used to mock webhook parsing
	jobsStorage := newFakeJobStorage()        // Used to mock job status
	trackQueue := newMockTrackQueue()         // Used to track number of jobs running per user
	cdQueue := &mockCdQueue{}                 // Used to run next stages of pipelines
	h := &handler{
		db:       db,
		js:       jobsStorage,
		whParser: mockWebhookParser,
		cdQueue:  cdQueue,
		tq:       trackQueue,
	}
	// Create a pipeline with two stages; the first stage is running,
	// the second is waiting
	const (
		testRepoId        uint64 = 1001
		testCommit        uint64 = 55022
		testCommitVersion uint64 = 1
		testPath          string = "path/to/file/CD.json"
		testJobName       string = "build-and-push"
		testRunNumber     int64  = 1
	)
	pipelineId := jobs.PipelineId(testRepoId, testCommit, testCommitVersion,
		testPath, testJobName, testRunNumber)
	jobsStorage.pipelinesById[pipelineId] = jobs.Pipeline{
		Status:         jobs.PipelineStatusRunning,
		NumberOfStages: 2,
	}
	jobsStorage.pipelineStagesByPipelineId[pipelineId] = []jobs.PipelineStage{
		{PipelineId: pipelineId, Stage: 0, Status: jobs.JobStatusRunning},
		{PipelineId: pipelineId, Stage: 1, Status: jobs.JobStatusWaiting},
	}
	stage0Id := jobs.PipelineStageId(pipelineId, 0)
	stage1Id := jobs.PipelineStageId(pipelineId, 1)

	// We get a webhook saying the stage 0 finished
	// Stage 0 stage should update to "success" and Stage 1 should still be
	// marked as queued. The next stage should be enqueued at the queue and
	// The stage0 should be marked as done in the trackqueue
	testGotStageZeroSucces := func() {
		checkOkWebhook(stage0Id, trackclient.TrackJobStatusSuccess, mockWebhookParser, h, t)
		jobsStorage.checkPipelineStatus(pipelineId, jobs.PipelineStatusRunning, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 0, jobs.JobStatusSuccess, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 1, jobs.JobStatusWaiting, t)
		trackQueue.checkJobFinished(stage0Id, t)
		cdQueue.checkIsEnqueued(pipelineId, 1, t)
	}
	// Run a couple times to ensure idempotency works
	testGotStageZeroSucces()
	testGotStageZeroSucces()

	// Try sending a webhook that says stage1 failed.
	// Stage1 isn't yet marked as "queued" nor "running", so this should return
	// an error
	checkBadWebhook(stage1Id, trackclient.TrackJobStatusFail, mockWebhookParser, h, t)
	// Let's mock as it we had published the stage1 to be "posted"
	jobsStorage.pipelineStagesByPipelineId[pipelineId] = []jobs.PipelineStage{
		{PipelineId: pipelineId, Stage: 0, Status: jobs.JobStatusSuccess},
		{PipelineId: pipelineId, Stage: 1, Status: jobs.JobStatusPosted},
	}

	// Send a webhook communicating stage 1 started running
	testStageOneStartedToRun := func() {
		checkOkWebhook(stage1Id, trackclient.TrackJobStatusRunning, mockWebhookParser, h, t)
		jobsStorage.checkPipelineStatus(pipelineId, jobs.PipelineStatusRunning, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 0, jobs.JobStatusSuccess, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 1, jobs.JobStatusRunning, t)
	}
	testStageOneStartedToRun()
	testStageOneStartedToRun()

	// Now the whole pipeline will be marked as success.
	// No other job should be enqueued
	sizeBeforeStageOneSuccess := len(cdQueue.queue)
	testGotStageOneSuccess := func() {
		checkOkWebhook(stage1Id, trackclient.TrackJobStatusSuccess, mockWebhookParser, h, t)
		jobsStorage.checkPipelineStatus(pipelineId, jobs.PipelineStatusSuccess, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 0, jobs.JobStatusSuccess, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 1, jobs.JobStatusSuccess, t)
		trackQueue.checkJobFinished(stage0Id, t)
		trackQueue.checkJobFinished(stage1Id, t)
		cdQueue.checkQueueSize(sizeBeforeStageOneSuccess, t)
	}
	testGotStageOneSuccess()
	testGotStageOneSuccess()

	// Test some other unexpected hooks
	checkBadWebhook(stage0Id, trackclient.TrackJobStatusQueued, mockWebhookParser, h, t)
	checkBadWebhook(stage0Id, trackclient.TrackJobStatusFail, mockWebhookParser, h, t)
	checkBadWebhook(stage0Id, trackclient.TrackJobStatusTimeout, mockWebhookParser, h, t)
	checkBadWebhook(stage1Id, trackclient.TrackJobStatusQueued, mockWebhookParser, h, t)
	checkBadWebhook(stage1Id, trackclient.TrackJobStatusFail, mockWebhookParser, h, t)
	checkBadWebhook(stage1Id, trackclient.TrackJobStatusTimeout, mockWebhookParser, h, t)

}

func TestHandlePipelinesCancelation(t *testing.T) {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	mockWebhookParser := &mockWebhookParser{}
	jobsStorage := newFakeJobStorage()
	trackQueue := newMockTrackQueue()
	cdQueue := &mockCdQueue{}
	h := &handler{
		db:       db,
		js:       jobsStorage,
		whParser: mockWebhookParser,
		cdQueue:  cdQueue,
		tq:       trackQueue,
	}
	// Create a pipeline with three stages:
	// Stage0: success
	// Stage1: running
	// Stage2: waiting
	pipelineId := jobs.PipelineId( /*repoId*/ 100,
		/*commit*/ 101 /*commitV*/, 102,
		"path/to/cd", "jobname" /*runNumber*/, 1)
	jobsStorage.pipelinesById[pipelineId] = jobs.Pipeline{
		Status:         jobs.PipelineStatusRunning,
		NumberOfStages: 2,
	}
	jobsStorage.pipelineStagesByPipelineId[pipelineId] = []jobs.PipelineStage{
		{PipelineId: pipelineId, Stage: 0, Status: jobs.JobStatusSuccess},
		{PipelineId: pipelineId, Stage: 1, Status: jobs.JobStatusRunning},
		{PipelineId: pipelineId, Stage: 2, Status: jobs.JobStatusWaiting},
	}
	stage1Id := jobs.PipelineStageId(pipelineId, 1)

	// Post a webhook that says Stage1 was canceled
	testStage1WasCanceled := func() {
		checkOkWebhook(stage1Id, trackclient.TrackJobStatusCancel, mockWebhookParser, h, t)
		jobsStorage.checkPipelineStatus(pipelineId, jobs.PipelineStatusCancel, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 0, jobs.JobStatusSuccess, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 1, jobs.JobStatusCanceled, t)
		jobsStorage.checkPipelineStageStatus(pipelineId, 2, jobs.JobStatusWaiting, t)
		trackQueue.checkJobFinished(stage1Id, t)
		cdQueue.checkIsNotEnqueued(pipelineId, 1, t)
	}
	// Run a couple times to ensure idempotency works
	testStage1WasCanceled()
	testStage1WasCanceled()
}

func TestHandleJobCancelation(t *testing.T) {
	db, closeDb, err := webdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	defer closeDb()
	mockWebhookParser := &mockWebhookParser{}
	jobsStorage := newFakeJobStorage()
	trackQueue := newMockTrackQueue()
	cdQueue := &mockCdQueue{}
	h := &handler{
		db:       db,
		js:       jobsStorage,
		whParser: mockWebhookParser,
		cdQueue:  cdQueue,
		tq:       trackQueue,
	}
	mockJob := jobs.Job{
		InternalId:    1,
		RepoId:        2,
		Commit:        3,
		CommitVersion: 4,
		Path:          "path/to/file",
		Name:          "jobname",
		RunNumber:     5,
		Status:        jobs.JobStatusRunning,
	}
	mockJobId := mockJob.Id()
	jobsStorage.jobsById[mockJobId] = mockJob

	// Helper to test cancelation of the mockJobId
	testJobWasCanceled := func() {
		checkOkWebhook(mockJobId, trackclient.TrackJobStatusCancel, mockWebhookParser, h, t)
		jobsStorage.checkJobStatus(mockJobId, jobs.JobStatusCanceled, t)
		trackQueue.checkJobFinished(mockJobId, t)
	}
	// Run twice to check idempotency
	testJobWasCanceled()
	testJobWasCanceled()
}

// Helper to post a webhook of a status and check that an OK response is returned
func checkOkWebhook(stageId string, status trackclient.TrackJobStatus,
	parser *mockWebhookParser, h *handler, t *testing.T) {
	parser.trackJob = trackclient.TrackJob{
		Id:     stageId,
		Status: status,
	}
	resp := httptest.NewRecorder()
	h.handleTrackWebhook(resp, wrappers.ServerKeyAuthTrackMuxRequest{
		Request: httptest.NewRequest("GET", "/", nil),
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("got non-ok response for stageId=%q status=%q", stageId, status)
	}
}

// Helper to post a webhook of a status and check that a non-OK response is returned
func checkBadWebhook(stageId string, status trackclient.TrackJobStatus,
	parser *mockWebhookParser, h *handler, t *testing.T) {
	parser.trackJob = trackclient.TrackJob{
		Id:     stageId,
		Status: status,
	}
	resp := httptest.NewRecorder()
	h.handleTrackWebhook(resp, wrappers.ServerKeyAuthTrackMuxRequest{
		Request: httptest.NewRequest("GET", "/", nil),
	})
	if resp.Code == http.StatusOK {
		t.Fatalf("got ok response for stageId=%q status=%q", stageId, status)
	}
}

type fakeJobsStorage struct {
	jobsById                   map[string]jobs.Job
	pipelinesById              map[string]jobs.Pipeline
	pipelineStagesByPipelineId map[string][]jobs.PipelineStage
}

func newFakeJobStorage() *fakeJobsStorage {
	return &fakeJobsStorage{
		jobsById:                   map[string]jobs.Job{},
		pipelinesById:              map[string]jobs.Pipeline{},
		pipelineStagesByPipelineId: map[string][]jobs.PipelineStage{},
	}
}

func (js fakeJobsStorage) GetJobById(rl context.Context, id string) (jobs.Job, error) {
	j, ok := js.jobsById[id]
	if !ok {
		return j, errors.New("not found")
	}
	return j, nil
}
func (js fakeJobsStorage) SetJobStatus(wl context.Context, id string, status jobs.JobStatus) error {
	jobCopy, ok := js.jobsById[id]
	if !ok {
		return errors.New("not found")
	}
	jobCopy.Status = status
	js.jobsById[id] = jobCopy
	return nil
}
func (js fakeJobsStorage) GetPipelineStagesById(tx context.Context, id string) (iterator.I[jobs.PipelineStage], error) {
	stages, ok := js.pipelineStagesByPipelineId[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return iterator.NewIterFromSlice(stages), nil
}
func (js fakeJobsStorage) SetStatusOfPipelineStage(tx context.Context, pipelineId string, stage int32, status jobs.JobStatus) error {
	stages, ok := js.pipelineStagesByPipelineId[pipelineId]
	if !ok {
		return errors.New("pipeline not found")
	}
	if stage >= int32(len(stages)) {
		return errors.New("stage not found")
	}
	stages[stage].Status = status
	isLastStage := stage == int32(len(stages)-1)
	if isLastStage || slices.Contains([]jobs.JobStatus{
		jobs.JobStatusFail,
		jobs.JobStatusTimeout,
		jobs.JobStatusCanceled,
	}, status) {
		pipelineCopy := js.pipelinesById[pipelineId]
		switch status {
		case jobs.JobStatusFail:
			pipelineCopy.Status = jobs.PipelineStatusFail
		case jobs.JobStatusTimeout:
			pipelineCopy.Status = jobs.PipelineStatusFail
		case jobs.JobStatusCanceled:
			pipelineCopy.Status = jobs.PipelineStatusCancel
		case jobs.JobStatusRunning:
			pipelineCopy.Status = jobs.PipelineStatusRunning
		default:
			pipelineCopy.Status = jobs.PipelineStatusSuccess
		}
		js.pipelinesById[pipelineId] = pipelineCopy
	}

	js.pipelineStagesByPipelineId[pipelineId] = stages
	return nil
}
func (js fakeJobsStorage) checkPipelineStatus(pipelineId string, expected jobs.PipelineStatus, t *testing.T) {
	t.Helper()
	p := js.pipelinesById[pipelineId]
	if p.Status != expected {
		t.Fatalf("expected status %q got %q", expected, p.Status)
	}
}
func (js fakeJobsStorage) checkPipelineStageStatus(pipelineId string, stage int32, expected jobs.JobStatus, t *testing.T) {
	t.Helper()
	stages := js.pipelineStagesByPipelineId[pipelineId]
	if stage >= int32(len(stages)) {
		t.Fatalf("pipeline only has %d stages", len(stages))
	}
	if stages[stage].Status != expected {
		t.Fatalf("expected status %q got %q", expected, stages[stage].Status)
	}
}

func (js fakeJobsStorage) checkJobStatus(jobId string, expected jobs.JobStatus, t *testing.T) {
	t.Helper()
	gotJob, ok := js.jobsById[jobId]
	if !ok {
		t.Fatalf("jobId=%q not found", jobId)
	}
	if gotJob.Status != expected {
		t.Fatalf("expected status %q got %q", expected, gotJob.Status)
	}
}

type mockWebhookParser struct {
	trackJob   trackclient.TrackJob
	jobPayload runnerlib.JobPayload
}

func (wh mockWebhookParser) ParseWebhook(r io.Reader) (trackclient.TrackJob, runnerlib.JobPayload, error) {
	return wh.trackJob, wh.jobPayload, nil
}

type mockTrackQueue struct {
	finishedJobs map[string]bool
}

func newMockTrackQueue() *mockTrackQueue {
	return &mockTrackQueue{
		finishedJobs: map[string]bool{},
	}
}
func (tq mockTrackQueue) PutJobFinished(jobId string, tx context.Context) error {
	tq.finishedJobs[jobId] = true
	return nil
}
func (tq mockTrackQueue) checkJobFinished(jobId string, t *testing.T) {
	_, ok := tq.finishedJobs[jobId]
	if !ok {
		t.Fatalf("jobId=%q not marked as finished", jobId)
	}
}

type mockCdQueue struct {
	queue []mockCdQueueEntry
}
type mockCdQueueEntry struct {
	PipelineId string
	Stage      int32
}

func newMockCdQueue() *mockCdQueue {
	return &mockCdQueue{}
}

func (q *mockCdQueue) ResumeCdToStage(pipelineId string, Stage int32) error {
	q.queue = append(q.queue, mockCdQueueEntry{pipelineId, Stage})
	return nil
}

func (q *mockCdQueue) checkIsEnqueued(pipelineId string, Stage int32, t *testing.T) {
	for i := range q.queue {
		if q.queue[i].PipelineId == pipelineId && q.queue[i].Stage == Stage {
			return
		}
	}
	t.Fatalf("%s stage %d is not enqueued", pipelineId, Stage)
}
func (q *mockCdQueue) checkIsNotEnqueued(pipelineId string, Stage int32, t *testing.T) {
	for i := range q.queue {
		if q.queue[i].PipelineId == pipelineId && q.queue[i].Stage == Stage {
			t.Fatalf("%s stage %d is enqueued", pipelineId, Stage)
		}
	}
}
func (q *mockCdQueue) checkQueueSize(size int, t *testing.T) {
	if len(q.queue) != size {
		t.Fatalf("expected size %d got %d", size, len(q.queue))
	}
}
