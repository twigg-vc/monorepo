package jobshandler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"monorepo/base/iterator"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/wrappers"
	"net/http"
	"strconv"
)

type handler struct {
	jobsService    JobsService
	tc             TrackClient
	pr             PipelineResumer
	usernameGetter UsernameGetter
}

// The frontend uses these values to know wheather to show a "load more" or not
const (
	getPipelineRefsPageSize = 20
	getRefPipelinesPageSize = 10
)

// Returns the PipelineRefs of a repository, i.e. the "names" of all the
// pipelines in a repository.
func (h handler) HandleGetPipelineRefs(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbRead context.Context) {
	if !r.Flags.ShowCdJobs {
		http.Error(w, "disabled", http.StatusBadRequest)
		return
	}
	afterPath := ""
	afterPaths := r.Request.URL.Query()[routes.AfterRefPathQueryParamName]
	if len(afterPaths) != 0 && len(afterPaths) != 1 {
		http.Error(w, "bad num of afterPaths", http.StatusBadRequest)
		return
	}
	if len(afterPaths) == 1 {
		afterPath = afterPaths[0]
	}
	afterName := ""
	afterNames := r.Request.URL.Query()[routes.AfterRefNameQueryParamName]
	if len(afterNames) != 0 && len(afterNames) != 1 {
		http.Error(w, "bad num of afterNames", http.StatusBadRequest)
		return
	}
	if len(afterNames) == 1 {
		afterName = afterNames[0]
	}
	refsIter, err := h.jobsService.GetRepoPipelineRefs(
		dbRead, r.Repo.Id, afterPath, afterName)
	if err != nil {
		log.Printf("failed to get pipeline refs: %s", err)
		http.Error(w, "failed to read refs", http.StatusInternalServerError)
		return
	}
	refs, err := iterator.GetFirstN(getPipelineRefsPageSize, refsIter)
	if err != nil {
		log.Printf("failed to iterate pipeline refs: %s", err)
		http.Error(w, "failed to iterate refs", http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(refs)
	if err != nil {
		log.Printf("failed to write refs resp: %s", err)
		http.Error(w, "failed to write refs resp", http.StatusInternalServerError)
		return
	}
}

// Returns the Pipelines of PipelineRef, i.e. the actual executions of a
// pipeline of a specific name
func (h handler) HandleGetRefPipelines(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbRead context.Context) {
	if !r.Flags.ShowCdJobs {
		http.Error(w, "disabled", http.StatusBadRequest)
		return
	}
	refPath := r.Request.PathValue(routes.PipelineRefPathPathParamName)
	refName := r.Request.PathValue(routes.PipelineRefNamePathParamName)
	var afterInternalJobId int64 = 0
	if p := r.URL.Query().Get(routes.AfterInternalJobIdQueryParamName); p != "" {
		var err error
		afterInternalJobId, err = strconv.ParseInt(p, 10, 64)
		if err != nil {
			log.Printf("err parsing afterJobId int: %s", err)
			http.Error(w, "invalid afterJobId value", http.StatusBadRequest)
			return
		}
	}
	pipelinesIter, err := h.jobsService.GetRepoPipelinesByRef(
		dbRead, r.Repo.Id, refPath, refName, afterInternalJobId)
	if err != nil {
		log.Printf("failed to get pipelines by ref: %s", err)
		http.Error(w, "failed to read pipelines", http.StatusInternalServerError)
		return
	}
	pipelines, err := iterator.GetFirstN(getRefPipelinesPageSize, pipelinesIter)
	if err != nil {
		log.Printf("failed to iterate pipelines: %s", err)
		http.Error(w, "failed to iterate", http.StatusInternalServerError)
		return
	}
	frontendPipeline := make([]FrontendPipeline, 0, len(pipelines))
	for i := range pipelines {
		var username string
		var err error
		if pipelines[i].IsCreatedByUser {
			username, err = h.usernameGetter.GetUsername(pipelines[i].CreatedByUserId, dbRead)
			if err != nil {
				log.Printf("failed to get pipeline creator username: %s", err)
				http.Error(w, "failed to get username", http.StatusInternalServerError)
				return
			}
		}
		frontendPipeline = append(frontendPipeline, FrontendPipeline{
			Id:                pipelines[i].Id(),
			Pipeline:          pipelines[i],
			CreatedByUsername: username,
		})
	}
	err = json.NewEncoder(w).Encode(frontendPipeline)
	if err != nil {
		log.Printf("failed to write pipelines resp: %s", err)
		http.Error(w, "failed to write pipelines resp", http.StatusInternalServerError)
		return
	}
}

// Returns a pipeline by its id
func (h handler) HandleGetPipeline(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest, dbRead context.Context) {
	if !r.Flags.ShowCdJobs {
		http.Error(w, "disabled", http.StatusBadRequest)
		return
	}
	pipeline := r.Pipeline
	pId := FrontendPipeline{
		Id:       pipeline.Id(),
		Pipeline: pipeline,
	}
	if pipeline.IsCreatedByUser {
		username, err := h.usernameGetter.GetUsername(pipeline.CreatedByUserId, dbRead)
		if err != nil {
			log.Printf("failed to get pipeline creator username: %s", err)
			http.Error(w, "failed to get username", http.StatusInternalServerError)
			return
		}
		pId.CreatedByUsername = username
	}
	err := json.NewEncoder(w).Encode(pId)
	if err != nil {
		log.Printf("failed to write pipeline resp: %s", err)
		http.Error(w, "failed to write pipeline resp", http.StatusInternalServerError)
		return
	}
}

// Returns the stages of a pipeline execution
func (h handler) HandleGetPipelineStages(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest, dbRead context.Context) {
	if !r.Flags.ShowCdJobs {
		http.Error(w, "disabled", http.StatusBadRequest)
		return
	}
	stagesIter, err := h.jobsService.GetPipelineStagesById(dbRead, r.Pipeline.Id())
	if err != nil {
		log.Printf("failed to get stages by pipelineId: %s", err)
		http.Error(w, "failed to get stages", http.StatusInternalServerError)
		return
	}
	// Read 1 more than the maxStages to know if it has more than the max
	// expected number of stages. This shouldn't really happen because
	// we should consider pipeline that have too many stages to be invalid.
	const maxStages = 200
	stages, err := iterator.GetFirstN(maxStages+1, stagesIter)
	if err != nil {
		log.Printf("failed to iterate on stages: %s", err)
		http.Error(w, "failed to iterate on stages", http.StatusInternalServerError)
		return
	}
	if len(stages) == maxStages+1 {
		log.Printf("pipeline %s has too many stages (>%d)", r.Pipeline.Id(), maxStages)
		http.Error(w, "too many stages", http.StatusInternalServerError)
		return
	}
	frontendStages := make([]FrontendPipelineStage, len(stages))
	for i := range stages {
		frontendStages[i] = FrontendPipelineStage{
			PipelineId:        stages[i].PipelineId,
			Name:              stages[i].Name,
			Stage:             stages[i].Stage,
			IsResumedByUser:   stages[i].IsResumedByUser,
			ResumedByUsername: "",
			CreatedTime:       stages[i].CreatedTime,
			Status:            stages[i].Status,
		}
		if !stages[i].IsResumedByUser {
			continue
		}
		username, err := h.usernameGetter.GetUsername(stages[i].ResumedByUserId, dbRead)
		if err != nil {
			log.Printf("failed to read username: %s", err)
			http.Error(w, "failed to read username", http.StatusInternalServerError)
			return
		}
		frontendStages[i].ResumedByUsername = username
	}

	err = json.NewEncoder(w).Encode(frontendStages)
	if err != nil {
		log.Printf("failed to write stages resp: %s", err)
		http.Error(w, "failed to write stages resp", http.StatusInternalServerError)
		return
	}
}

const stageHasntStartedMsg = "stage hasn't started yet"

func (hl handler) HandleGetStageCombinedOut(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest, dbRead context.Context) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	pipelineId := r.Pipeline.Id()
	stageS := r.Request.PathValue(routes.StagePathParamName)
	var stage64 int64 = -1
	stage64, err := strconv.ParseInt(stageS, 10, 64)
	if err != nil || stage64 < 0 || stage64 >= math.MaxInt32 {
		http.Error(w, "bad stage", http.StatusBadRequest)
		return
	}
	stage := int32(stage64) // This is safe bc we check if `stage64 >= math.MaxInt32`
	stg, err := hl.jobsService.GetPipelineStage(dbRead, pipelineId, stage)
	if err != nil {
		log.Printf("failed to get pipelineId=%q stage %d: %s", pipelineId, stage64, err)
		http.Error(w, "failed to get stage", http.StatusInternalServerError)
		return
	}

	switch stg.Status {
	case jobs.JobStatusWaitingManualStart:
		_, _ = w.Write([]byte(stageHasntStartedMsg))
		return
	case jobs.JobStatusWaiting:
		_, _ = w.Write([]byte(stageHasntStartedMsg))
		return
	case jobs.JobStatusQueued:
		_, _ = w.Write([]byte(stageHasntStartedMsg))
		return
	case jobs.JobStatusPosted:
		_, _ = w.Write([]byte(stageHasntStartedMsg))
		return
	case jobs.JobStatusRunning:
	case jobs.JobStatusSuccess:
	case jobs.JobStatusFail:
	case jobs.JobStatusTimeout:
	case jobs.JobStatusCanceled:
	case jobs.JobStatusTooManyJobs:
		_, _ = w.Write([]byte("failed to start: too many jobs"))
		return
	case jobs.JobStatusBadFileFormat:
		_, _ = w.Write([]byte("failed to start: bad file format"))
		return
	case jobs.JobStatusBadFileSize:
		_, _ = w.Write([]byte("failed to start: bad file size"))
		return
	case jobs.JobStatusExceedsPlanLimits:
		_, _ = w.Write([]byte("failed to start: exceeded plan limits"))
		return
	default:
		log.Printf("unexpected stage status: %q", stg.Status)
		http.Error(w, "unexpected status", http.StatusInternalServerError)
		return
	}
	stageId := jobs.PipelineStageId(pipelineId, stage)
	combinedOut, err := hl.tc.GetCombinedOutput(stageId)
	if err != nil {
		log.Printf("failed to get combinedOut stageId %s: %s", stageId, err)
		http.Error(w, "internal error getting logs",
			http.StatusInternalServerError)
		return
	}
	defer combinedOut.Close()
	_, _ = io.Copy(w, combinedOut)
}

func (hl handler) HandleManualResumeStage(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	pipelineId := r.Pipeline.Id()
	stage, ok := parseStage(w, r)
	if !ok {
		return
	}
	isCantResumeErr, err := hl.pr.ManualResumePipeline(pipelineId, stage, r.UserWithWritePermission.Id, dbWrite)
	if isCantResumeErr {
		log.Printf("pipelineId=%q stage=%d cant yet resume", pipelineId, stage)
		http.Error(w, "cant resume yet", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("failed to manual-resume pipelineId=%q stage=%d: %s", pipelineId, stage, err)
		http.Error(w, "failed to resume", http.StatusInternalServerError)
		return
	}
	_, err = w.Write([]byte("ok"))
	if err != nil {
		log.Printf("failed to get write back ok: %s", err)
		return
	}
	shouldCommit = true
	return
}
func (hl handler) HandleCancelPipelineStage(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	pipelineId := r.Pipeline.Id()
	stageN, ok := parseStage(w, r)
	if !ok {
		return
	}
	stage, err := hl.jobsService.GetPipelineStage(dbWrite, pipelineId, stageN)
	if err != nil {
		log.Printf("failed to get pipelineId=%q stage %d: %s", pipelineId, stageN, err)
		http.Error(w, "failed to get stage", http.StatusInternalServerError)
		return
	}
	switch stage.Status {
	case jobs.JobStatusPosted, jobs.JobStatusRunning:
		err = hl.tc.Cancel(stage.Id())
		if err != nil {
			log.Printf("failed to cancel pipelineId=%q stage %d: %s", pipelineId, stageN, err)
			http.Error(w, "failed to cancel running/posted job", http.StatusInternalServerError)
			return
		}
		shouldCommit = false // Nothing to commit
		return
	default:
		http.Error(w, fmt.Sprintf("status %s cant be canceled", stage.Status), http.StatusBadRequest)
		return
	}
}
func (hl handler) HandleGetPipelineStageIsCanceled(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest, dbRead context.Context) {
	pipelineId := r.Pipeline.Id()
	stageN, ok := parseStage(w, r)
	if !ok {
		return
	}
	stage, err := hl.jobsService.GetPipelineStage(dbRead, pipelineId, stageN)
	if err != nil {
		log.Printf("failed to get pipelineId=%q stage %d: %s", pipelineId, stageN, err)
		http.Error(w, "failed to get stage", http.StatusInternalServerError)
		return
	}
	if stage.Status == jobs.JobStatusCanceled {
		_, err = w.Write([]byte("1"))
	} else {
		_, err = w.Write([]byte("0"))
	}
	if err != nil {
		log.Printf("failed to write pipelineId=%q stage %d isCanceled: %s", pipelineId, stageN, err)
		http.Error(w, "failed to write resp", http.StatusInternalServerError)
		return
	}
}

func (hl handler) HandleManuallyLaunchPipeline(w http.ResponseWriter,
	r wrappers.UserRepoMuxRequest, dbWrite context.Context) (shouldCommit bool) {
	pipelinePath := r.Request.PathValue(routes.PipelineRefPathPathParamName)
	pipelineName := r.Request.PathValue(routes.PipelineRefNamePathParamName)
	cIdString := r.Request.PathValue(routes.LaunchPipelineCommitIdPathParamName)
	cVersionString := r.Request.PathValue(routes.LaunchPipelineCommitVersionPathParamName)

	cId, err := strconv.ParseUint(cIdString, 10, 64)
	if err != nil {
		http.Error(w, "bad commitId", http.StatusBadRequest)
		return
	}
	cV, err := strconv.ParseUint(cVersionString, 10, 64)
	if err != nil {
		http.Error(w, "bad commit version", http.StatusBadRequest)
		return
	}

	err = hl.pr.ManuallyLaunchCd(r.Repo.Id, cId, cV, pipelinePath, pipelineName, r.UserWithWritePermission.Id, dbWrite)
	if err != nil {
		log.Printf("failed to launch cd: c%dv%d path=%s name=%s: %s", cId, cV, pipelinePath, pipelineName, err)
		http.Error(w, "failed to launch", http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte("ok"))
	if err != nil {
		log.Printf("failed to get write back ok: %s", err)
		return
	}
	shouldCommit = true
	return
}

// Writes error resp when applicable
func parseStage(w http.ResponseWriter,
	r wrappers.UserRepoPipelineMuxRequest) (stageN int32, ok bool) {
	stageS := r.Request.PathValue(routes.StagePathParamName)
	var stage64 int64 = -1
	stage64, err := strconv.ParseInt(stageS, 10, 64)
	if err != nil || stage64 < 0 || stage64 >= math.MaxInt32 {
		http.Error(w, "bad stage", http.StatusBadRequest)
		return
	}
	// This is safe bc we check if `stage64 >= math.MaxInt32`
	stageN = int32(stage64)
	ok = true
	return
}

// Call a validation function to let us know if we screwed up
var _ = func() int {
	if getPipelineRefsPageSize != 20 {
		panic("getPipelineRefsPageSize changed. Did you also change the frontend?")
	}
	if getRefPipelinesPageSize != 10 {
		panic("getRefPipelinesPageSize changed. Did you also change the frontend?")
	}
	return 0
}()
