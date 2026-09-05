package jobshandler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"monorepo/base/iterator"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"testing"
)

func TestHandleGetPipelineRefs(t *testing.T) {
	js := newFakeJobService()
	js.repoIdToRefs[76] = []jobs.PipelineRef{
		{RepoId: 76, Path: "path/to/file1", Name: "job1"},
		{RepoId: 76, Path: "path/to/file1", Name: "job2"},
		{RepoId: 76, Path: "path/to/file2", Name: "job1"},
		{RepoId: 76, Path: "path/to/file2", Name: "job2"},
	}
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)
	w := httptest.NewRecorder()
	q := url.Values{}
	q.Set(routes.AfterRefPathQueryParamName, "path/to/file1")
	q.Set(routes.AfterRefNameQueryParamName, "job2")
	pathValues := map[string]string{}
	req := newUserRepoMuxReq(76, pathValues, q)
	h.HandleGetPipelineRefs(w, req, context.Background())

	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	expectedRefs := []jobs.PipelineRef{
		{RepoId: 76, Path: "path/to/file2", Name: "job1"},
		{RepoId: 76, Path: "path/to/file2", Name: "job2"},
	}
	checkRefsResponse(w, expectedRefs, t)
}

func TestHandleGetRefPipelines(t *testing.T) {
	js := newFakeJobService()
	ref0 := jobs.PipelineRef{RepoId: 54, Path: "ref0/path", Name: "ref0/name"}
	ref1 := jobs.PipelineRef{RepoId: 54, Path: "ref1/path", Name: "ref1/name"}
	js.refToPipelines[ref0] = []jobs.Pipeline{
		{InternalId: 1, Name: "mock running pipeline 1", IsCreatedByUser: true, CreatedByUserId: 1},
		{InternalId: 2, Name: "mock running pipeline 2", IsCreatedByUser: true, CreatedByUserId: 2},
	}
	js.refToPipelines[ref1] = []jobs.Pipeline{
		{InternalId: 3, Name: "mock running pipeline 3"},
		{InternalId: 4, Name: "mock running pipeline 4"},
		{InternalId: 5, Name: "mock running pipeline 5"},
		{InternalId: 6, Name: "mock running pipeline 6"},
	}
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	ug.getUsername = func(id int64) (string, error) {
		switch id {
		case 1:
			return fmt.Sprintf("user-%d", 1), nil
		case 2:
			return fmt.Sprintf("user-%d", 2), nil
		case 3:
			return fmt.Sprintf("user-%d", 3), nil
		case 4:
			return fmt.Sprintf("user-%d", 4), nil
		default:
			return "", errors.New("not found")
		}
	}
	h := NewHandler(ug, js, tc, p)

	// Get pipelines for ref0
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{
		routes.PipelineRefPathPathParamName: "ref0/path",
		routes.PipelineRefNamePathParamName: "ref0/name",
	}
	req := newUserRepoMuxReq(54, pathValues, q)
	h.HandleGetRefPipelines(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	expectedPipelines := []FrontendPipeline{
		{
			Id:                jobs.Pipeline{InternalId: 2, Name: "mock running pipeline 2"}.Id(),
			Pipeline:          jobs.Pipeline{InternalId: 2, Name: "mock running pipeline 2", IsCreatedByUser: true, CreatedByUserId: 2},
			CreatedByUsername: "user-2",
		},
		{
			Id:                jobs.Pipeline{InternalId: 1, Name: "mock running pipeline 1"}.Id(),
			Pipeline:          jobs.Pipeline{InternalId: 1, Name: "mock running pipeline 1", IsCreatedByUser: true, CreatedByUserId: 1},
			CreatedByUsername: "user-1",
		},
	}
	checkPipelinesResponse(w, expectedPipelines, t)

	// Get pipelines for ref1 after id=5
	w = httptest.NewRecorder()
	q = url.Values{}
	pathValues = map[string]string{
		routes.PipelineRefPathPathParamName: "ref1/path",
		routes.PipelineRefNamePathParamName: "ref1/name",
	}
	q.Set(routes.AfterInternalJobIdQueryParamName, "5")
	req = newUserRepoMuxReq(54, pathValues, q)
	h.HandleGetRefPipelines(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	expectedPipelines = []FrontendPipeline{
		{
			Id:       jobs.Pipeline{InternalId: 4, Name: "mock running pipeline 4"}.Id(),
			Pipeline: jobs.Pipeline{InternalId: 4, Name: "mock running pipeline 4"},
		},
		{
			Id:       jobs.Pipeline{InternalId: 3, Name: "mock running pipeline 3"}.Id(),
			Pipeline: jobs.Pipeline{InternalId: 3, Name: "mock running pipeline 3"},
		},
	}
	checkPipelinesResponse(w, expectedPipelines, t)
}

func TestHandleGetPipelineWithoutUser(t *testing.T) {
	js := newFakeJobService()
	ref := jobs.PipelineRef{RepoId: 54, Path: "ref/path", Name: "ref/name"}
	pipeline := jobs.Pipeline{InternalId: 1, Name: "mock running pipeline 1"}
	js.refToPipelines[ref] = []jobs.Pipeline{
		pipeline,
	}
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)

	// Get pipeline by id
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{}
	req := newUserRepoPipelineMuxReq(pipeline, pathValues, q)
	h.HandleGetPipeline(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkPipelineResponse(w,
		FrontendPipeline{Id: pipeline.Id(), Pipeline: pipeline}, t)
}

func TestHandleGetPipelineWithUser(t *testing.T) {
	js := newFakeJobService()
	ref := jobs.PipelineRef{RepoId: 54, Path: "ref/path", Name: "ref/name"}
	pipeline := jobs.Pipeline{InternalId: 1, Name: "mock running pipeline 1", IsCreatedByUser: true, CreatedByUserId: 99}
	js.refToPipelines[ref] = []jobs.Pipeline{
		pipeline,
	}
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	ug.getUsername = func(id int64) (string, error) {
		if id == 99 {
			return "user-99", nil
		}
		return "", errors.New("not found")
	}
	h := NewHandler(ug, js, tc, p)

	// Get pipeline by id
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{}
	req := newUserRepoPipelineMuxReq(pipeline, pathValues, q)
	h.HandleGetPipeline(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkPipelineResponse(w,
		FrontendPipeline{Id: pipeline.Id(), Pipeline: pipeline, CreatedByUsername: "user-99"}, t)
}

func TestHandleGetPipelineStages(t *testing.T) {
	js := newFakeJobService()
	pipeline := jobs.Pipeline{InternalId: 1, Name: "mock running pipeline 1"}
	stages := []jobs.PipelineStage{
		{Name: "stage-0", Stage: 0},
		{Name: "stage-1", Stage: 1, IsResumedByUser: true, ResumedByUserId: 76},
	}
	js.pipelineIdToStages[pipeline.Id()] = stages
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	ug.getUsername = func(id int64) (string, error) {
		if id == 76 {
			return "user-76", nil
		}
		return "", errors.New("not found")
	}
	h := NewHandler(ug, js, tc, p)

	// Get pipeline stages by the pipeline id
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{}
	req := newUserRepoPipelineMuxReq(pipeline, pathValues, q)
	h.HandleGetPipelineStages(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	expectedStages := []FrontendPipelineStage{
		{Name: "stage-0", Stage: 0},
		{Name: "stage-1", Stage: 1, IsResumedByUser: true, ResumedByUsername: "user-76"},
	}
	checkStagesResponse(w, expectedStages, t)
}

func TestHandleGetStageCombinedOut(t *testing.T) {
	js := newFakeJobService()
	pipeline := jobs.Pipeline{InternalId: 1, Name: "mock running pipeline 1"}
	stages := []jobs.PipelineStage{
		{Name: "stage-0", Stage: 0, Status: jobs.JobStatusSuccess},
		{Name: "stage-1", Stage: 1, Status: jobs.JobStatusQueued},
	}
	js.pipelineIdToStages[pipeline.Id()] = stages
	tc := newMockTrackClient()
	stage0TrackJobId := pipeline.IdOfStage(0)
	tc.trackJobIdToCombinedOut[stage0TrackJobId] = "mock stage output"
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)

	// Get the combined output of the stage 0
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{
		routes.StagePathParamName: "0",
	}
	req := newUserRepoPipelineMuxReq(pipeline, pathValues, q)
	h.HandleGetStageCombinedOut(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkStringResponse(w, "mock stage output", t)

	// Get the combined output of stage 1
	w = httptest.NewRecorder()
	q = url.Values{}
	pathValues = map[string]string{
		routes.StagePathParamName: "1",
	}
	req = newUserRepoPipelineMuxReq(pipeline, pathValues, q)
	h.HandleGetStageCombinedOut(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkStringResponse(w, stageHasntStartedMsg, t)
}

func TestHandleManualResume(t *testing.T) {
	js := newFakeJobService()
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)
	mockPipe := jobs.Pipeline{RepoId: 1, Commit: 2, CommitVersion: 3, Path: "path", Name: "name", RunNumber: 4}

	// Allow only pilepile with id=mockPipe.Id() to resume at stage 3
	const userId = 64
	p.allowResume(mockPipe.Id(), 3, userId)

	// Resume pipe-id stage 3
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{
		routes.StagePathParamName: "3",
	}
	req := newUserRepoPipelineMuxReq(mockPipe, pathValues, q)
	req.UserWithWritePermission.Id = userId
	h.HandleManualResumeStage(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkStringResponse(w, "ok", t)
	p.checkResumed(mockPipe.Id(), 3, userId, t)

	// Try resuming a non resumable pipeline
	nonResumablePipe := jobs.Pipeline{RepoId: 1, Commit: 2, CommitVersion: 3, Path: "path", Name: "other-name", RunNumber: 5}
	w = httptest.NewRecorder()
	q = url.Values{}
	pathValues = map[string]string{
		routes.StagePathParamName: "0",
	}
	req = newUserRepoPipelineMuxReq(nonResumablePipe, pathValues, q)
	h.HandleManualResumeStage(w, req, context.Background())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status bad-req got %d", w.Code)
	}
	p.checkDidNotResumed(nonResumablePipe.Id(), 0, userId, t)
}

func TestHandleLaunchPipeline(t *testing.T) {
	js := newFakeJobService()
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)

	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{
		routes.PipelineRefPathPathParamName:             "job-path",
		routes.PipelineRefNamePathParamName:             "job-name",
		routes.LaunchPipelineCommitIdPathParamName:      "1",
		routes.LaunchPipelineCommitVersionPathParamName: "2",
	}
	req := newUserRepoMuxReq(54, pathValues, q)
	req.UserWithWritePermission.Id = 986
	h.HandleManuallyLaunchPipeline(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkStringResponse(w, "ok", t)
	p.checkLaunched(54, 1, 2, "job-path", "job-name", req.UserWithWritePermission.Id, t)
}

func TestCancelPostedPipelineStage(t *testing.T) {
	js := newFakeJobService()
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)

	// Create two stages of a pipeline
	// Stage0: success
	// Stage1: posted
	pipeline0 := jobs.Pipeline{Name: "mock pipeline 0"}
	pipeline0Stages := []jobs.PipelineStage{
		{Stage: 0, Status: jobs.JobStatusSuccess},
		{Stage: 1, Status: jobs.JobStatusPosted},
	}
	pipeline0Id := pipeline0.Id()
	js.pipelineIdToStages[pipeline0Id] = pipeline0Stages

	// Cancel pipeline0 stage1 -> expect the track client to cancel it bc the
	// job was posted
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{
		routes.StagePathParamName: "1",
	}
	req := newUserRepoPipelineMuxReq(pipeline0, pathValues, q)
	h.HandleCancelPipelineStage(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	trackJobIdOfPipeline0Stage1 := pipeline0Stages[1].Id()
	tc.checkJobWasCanceled(trackJobIdOfPipeline0Stage1, t)

	// Create another pipeline that was still not posted
	pipeline1 := jobs.Pipeline{Name: "mock pipeline 1"}
	pipeline1Stages := []jobs.PipelineStage{
		{Stage: 0, Status: jobs.JobStatusQueued},
		{Stage: 1, Status: jobs.JobStatusWaiting},
	}
	pipeline1Id := pipeline1.Id()
	js.pipelineIdToStages[pipeline1Id] = pipeline1Stages
	// Try canceling stage0 -> expect bad response bc it's still not posted
	w = httptest.NewRecorder()
	q = url.Values{}
	pathValues = map[string]string{
		routes.StagePathParamName: "0",
	}
	req = newUserRepoPipelineMuxReq(pipeline1, pathValues, q)
	h.HandleCancelPipelineStage(w, req, context.Background())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status badRequest got %d", w.Code)
	}
	trackJobIdOfPipeline1Stage0 := pipeline1Stages[0].Id()
	tc.checkJobWasNotCanceled(trackJobIdOfPipeline1Stage0, t)
}

func TestGetStageIsCanceled(t *testing.T) {
	js := newFakeJobService()
	tc := newMockTrackClient()
	p := newMockPipelineResumer()
	ug := newMockUsernameGetter()
	h := NewHandler(ug, js, tc, p)

	// Create two stages of a pipeline
	// Stage0: success
	// Stage1: canceled
	pipeline0 := jobs.Pipeline{Name: "mock pipeline 0"}
	pipeline0Stages := []jobs.PipelineStage{
		{Stage: 0, Status: jobs.JobStatusSuccess},
		{Stage: 1, Status: jobs.JobStatusCanceled},
	}
	pipeline0Id := pipeline0.Id()
	js.pipelineIdToStages[pipeline0Id] = pipeline0Stages

	// Stage0 was not canceled
	w := httptest.NewRecorder()
	q := url.Values{}
	pathValues := map[string]string{
		routes.StagePathParamName: "0",
	}
	req := newUserRepoPipelineMuxReq(pipeline0, pathValues, q)
	h.HandleGetPipelineStageIsCanceled(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkStringResponse(w, "0", t)

	// Stage1 was canceled
	w = httptest.NewRecorder()
	q = url.Values{}
	pathValues = map[string]string{
		routes.StagePathParamName: "1",
	}
	req = newUserRepoPipelineMuxReq(pipeline0, pathValues, q)
	h.HandleGetPipelineStageIsCanceled(w, req, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("expected status ok got %d", w.Code)
	}
	checkStringResponse(w, "1", t)
}

// Helper to create a request
func newUserRepoMuxReq(repoId uint64, pathValues map[string]string, q url.Values) wrappers.UserRepoMuxRequest {
	httpReq := httptest.NewRequest("GET", "/?"+q.Encode(), nil)
	for key, val := range pathValues {
		httpReq.SetPathValue(key, val)
	}
	return wrappers.UserRepoMuxRequest{
		Request: httpReq,
		Flags: featureflags.Flags{
			ShowCdJobs: true,
		},
		Repo: repo.Repo{Id: repoId},
	}
}

// Helper to create a request
func newUserRepoPipelineMuxReq(p jobs.Pipeline, pathValues map[string]string, q url.Values) wrappers.UserRepoPipelineMuxRequest {
	httpReq := httptest.NewRequest("GET", "/?"+q.Encode(), nil)
	for key, val := range pathValues {
		httpReq.SetPathValue(key, val)
	}
	return wrappers.UserRepoPipelineMuxRequest{
		Request: httpReq,
		Flags: featureflags.Flags{
			ShowCdJobs: true,
		},
		Repo:     repo.Repo{Id: p.RepoId},
		Pipeline: p,
	}
}

// Helper to decode and check the refs response
func checkRefsResponse(w *httptest.ResponseRecorder, expectedRefs []jobs.PipelineRef, t *testing.T) {
	var refs []jobs.PipelineRef
	err := json.NewDecoder(w.Body).Decode(&refs)
	if err != nil {
		t.Fatalf("failed to decode resp: %s", err)
	}
	if !reflect.DeepEqual(refs, expectedRefs) {
		t.Fatalf("unexpected refs: %#v", refs)
	}
}

// Helper to decode and check the response
func checkPipelinesResponse(w *httptest.ResponseRecorder, expectedPipelines []FrontendPipeline, t *testing.T) {
	var pipelines []FrontendPipeline
	err := json.NewDecoder(w.Body).Decode(&pipelines)
	if err != nil {
		t.Fatalf("failed to decode resp: %s", err)
	}
	if !reflect.DeepEqual(pipelines, expectedPipelines) {
		t.Fatalf("unexpected pipelines: %#v", pipelines)
	}
}

// Helper to decode and check the response
func checkPipelineResponse(w *httptest.ResponseRecorder, expectedPipeline FrontendPipeline, t *testing.T) {
	var pipeline FrontendPipeline
	err := json.NewDecoder(w.Body).Decode(&pipeline)
	if err != nil {
		t.Fatalf("failed to decode resp: %s", err)
	}
	if !reflect.DeepEqual(pipeline, expectedPipeline) {
		t.Fatalf("unexpected pipeline: %#v", pipeline)
	}
}

// Helper to decode and check the response
func checkStagesResponse(w *httptest.ResponseRecorder, expectedStages []FrontendPipelineStage, t *testing.T) {
	var stages []FrontendPipelineStage
	err := json.NewDecoder(w.Body).Decode(&stages)
	if err != nil {
		t.Fatalf("failed to decode resp: %s", err)
	}
	if !reflect.DeepEqual(stages, expectedStages) {
		t.Fatalf("unexpected stages: %#v", stages)
	}
}

func checkStringResponse(w *httptest.ResponseRecorder, expectedResp string, t *testing.T) {
	buff := bytes.NewBuffer(nil)
	_, err := io.Copy(buff, w.Body)
	if err != nil {
		t.Fatalf("failed to read body: %s", err)
	}
	gotResp := buff.String()
	if gotResp != expectedResp {
		t.Fatalf("expected resp %q got %q", expectedResp, gotResp)
	}
}

func newFakeJobService() fakeJobService {
	return fakeJobService{
		repoIdToRefs:       map[uint64][]jobs.PipelineRef{},
		refToPipelines:     map[jobs.PipelineRef][]jobs.Pipeline{},
		pipelineIdToStages: map[string][]jobs.PipelineStage{},
	}
}

type fakeJobService struct {
	repoIdToRefs       map[uint64][]jobs.PipelineRef
	refToPipelines     map[jobs.PipelineRef][]jobs.Pipeline
	pipelineIdToStages map[string][]jobs.PipelineStage
}

func (f fakeJobService) GetRepoPipelineRefs(tx context.Context,
	repoId uint64, afterPath string, afterJobName string) (iterator.I[jobs.PipelineRef], error) {
	refs, ok := f.repoIdToRefs[repoId]
	if !ok {
		return nil, errors.New("not found")
	}
	// Make a sorted copy
	sorted := make([]jobs.PipelineRef, len(refs))
	copy(sorted, refs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path == sorted[j].Path {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Path < sorted[j].Path
	})
	// Filter to start after
	filtered := sorted
	if afterPath != "" || afterJobName != "" {
		filtered = filtered[:0]
		for _, r := range sorted {
			if r.Path > afterPath || (r.Path == afterPath && r.Name > afterJobName) {
				filtered = append(filtered, r)
			}
		}
	}
	return iterator.NewIterFromSlice(filtered), nil
}

func (f fakeJobService) GetRepoPipelinesByRef(tx context.Context,
	repoId uint64, filePath string, jobName string, afterInternalJobId int64) (iterator.I[jobs.Pipeline], error) {
	ref := jobs.PipelineRef{
		RepoId: repoId,
		Path:   filePath,
		Name:   jobName,
	}
	pipelines, ok := f.refToPipelines[ref]
	if !ok {
		return iterator.NewIterFromSlice[jobs.Pipeline](nil), nil
	}
	// Make sorted copy
	sorted := make([]jobs.Pipeline, len(pipelines))
	copy(sorted, pipelines)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].InternalId > sorted[j].InternalId // DESC order
	})
	filtered := sorted
	if afterInternalJobId != 0 {
		filtered = filtered[:0]
		for _, p := range sorted {
			if p.InternalId < afterInternalJobId {
				filtered = append(filtered, p)
			}
		}
	}
	return iterator.NewIterFromSlice(filtered), nil
}

func (f fakeJobService) GetPipelineById(tx context.Context, id string) (jobs.Pipeline, error) {
	for _, pipelines := range f.refToPipelines {
		for _, pipeline := range pipelines {
			if pipeline.Id() == id {
				return pipeline, nil
			}
		}
	}
	return jobs.Pipeline{}, fmt.Errorf("pipeline with id=%s not found", id)
}
func (f fakeJobService) GetPipelineStagesById(tx context.Context, id string) (iterator.I[jobs.PipelineStage], error) {
	stages := f.pipelineIdToStages[id]
	sortedStages := make([]jobs.PipelineStage, len(stages))
	copy(sortedStages, stages)
	sort.Slice(sortedStages, func(i, j int) bool {
		return sortedStages[i].Stage < sortedStages[j].Stage
	})
	return iterator.NewIterFromSlice(sortedStages), nil
}
func (f fakeJobService) GetPipelineStage(tx context.Context, pipelineId string, stage int32) (jobs.PipelineStage, error) {
	stagesIter, err := f.GetPipelineStagesById(tx, pipelineId)
	if err != nil {
		return jobs.PipelineStage{}, err
	}
	for stagesIter.Next() {
		stg, err := stagesIter.Get()
		if err != nil {
			return jobs.PipelineStage{}, err
		}
		if stg.Stage == stage {
			return stg, nil
		}
	}
	err = stagesIter.Err()
	if err != nil {
		return jobs.PipelineStage{}, err
	}
	return jobs.PipelineStage{}, errors.New("not found")
}

type mockTrackClient struct {
	trackJobIdToCombinedOut map[string]string
	canceledTrackJobIds     []string
}

func newMockTrackClient() *mockTrackClient {
	return &mockTrackClient{
		trackJobIdToCombinedOut: map[string]string{},
	}
}

func (tc *mockTrackClient) GetCombinedOutput(trackJobId string) (io.ReadCloser, error) {
	if tc.trackJobIdToCombinedOut == nil {
		tc.trackJobIdToCombinedOut = map[string]string{}
	}
	out, ok := tc.trackJobIdToCombinedOut[trackJobId]
	if !ok {
		return nil, fmt.Errorf("trackJobId=%s not found", trackJobId)
	}
	return io.NopCloser(bytes.NewBufferString(out)), nil
}
func (tc *mockTrackClient) Cancel(jobId string) error {
	tc.canceledTrackJobIds = append(tc.canceledTrackJobIds, jobId)
	return nil
}

func (tc *mockTrackClient) checkJobWasCanceled(jobId string, t *testing.T) {
	for _, j := range tc.canceledTrackJobIds {
		if j == jobId {
			return
		}
	}
	t.Fatalf("jobId=%q was not canceled", jobId)
}
func (tc *mockTrackClient) checkJobWasNotCanceled(jobId string, t *testing.T) {
	for _, j := range tc.canceledTrackJobIds {
		if j == jobId {
			t.Fatalf("jobId=%q was canceled", jobId)
		}
	}
}

type mockPipelineResumer struct {
	okToResume map[string]bool
	resumed    map[string]bool
	launched   map[string]bool
}

func newMockPipelineResumer() *mockPipelineResumer {
	return &mockPipelineResumer{
		okToResume: map[string]bool{},
		resumed:    map[string]bool{},
		launched:   map[string]bool{},
	}
}

func (p *mockPipelineResumer) ManualResumePipeline(pipelineId string, currentStage int32, userId int64, w context.Context) (bool, error) {
	id := fmt.Sprintf("%s-%d-%d", pipelineId, currentStage, userId)
	_, ok := p.okToResume[id]
	if !ok {
		return true, errors.New("cant resume")
	}
	p.resumed[id] = true
	delete(p.okToResume, id) // Cant resume again
	return false, nil
}
func (p *mockPipelineResumer) allowResume(pipelineId string, stage int32, userId int64) {
	id := fmt.Sprintf("%s-%d-%d", pipelineId, stage, userId)
	p.okToResume[id] = true
}

func (p *mockPipelineResumer) checkResumed(pipelineId string, stage int32, userId int64, t *testing.T) {
	id := fmt.Sprintf("%s-%d-%d", pipelineId, stage, userId)
	_, ok := p.resumed[id]
	if !ok {
		t.Fatalf("pipelineId=%q stage=%d did not resume", pipelineId, stage)
	}
}
func (p *mockPipelineResumer) checkDidNotResumed(pipelineId string, stage int32, userId int64, t *testing.T) {
	id := fmt.Sprintf("%s-%d-%d", pipelineId, stage, userId)
	_, ok := p.resumed[id]
	if ok {
		t.Fatalf("pipelineId=%q stage=%d did resumed", pipelineId, stage)
	}
}
func (p *mockPipelineResumer) ManuallyLaunchCd(repoId, commitId, commitVersion uint64, jobPath, jobName string, userId int64, w context.Context) error {
	id := fmt.Sprintf("%d-%d-%d-%s-%s-%d", repoId, commitId, commitVersion, jobPath, jobName, userId)
	p.launched[id] = true
	return nil
}
func (p *mockPipelineResumer) checkLaunched(repoId, commitId, commitVersion uint64, jobPath, jobName string, userId int64, t *testing.T) {
	id := fmt.Sprintf("%d-%d-%d-%s-%s-%d", repoId, commitId, commitVersion, jobPath, jobName, userId)
	_, ok := p.launched[id]
	if !ok {
		t.Fatalf("cd didn't lanunch")
	}
}

type mockUsernameGetter struct {
	getUsername func(id int64) (string, error)
}

func newMockUsernameGetter() *mockUsernameGetter {
	return &mockUsernameGetter{}
}
func (m mockUsernameGetter) GetUsername(userId int64, tx context.Context) (string, error) {
	if m.getUsername == nil {
		return "", errors.New("mock method not implemented")
	}
	return m.getUsername(userId)
}
