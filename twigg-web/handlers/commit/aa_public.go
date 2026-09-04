package commit

import (
	"context"
	"io"
	"time"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-web/cicdqueue"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/jobs"
	"monorepo/twigg-web/services/repo"
	reviewservice "monorepo/twigg-web/services/review"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/server"
)

// Registers the handlers required for viewing the commit pages.
// CanSubmitCache can be nil.
func AddHandlers(
	rt routes.Router,
	db Db,
	rSrv repo.Service,
	revSrv reviewservice.Service,
	userS user.Service,
	jobsS jobs.Service,
	ciq CiCdQueue,
	parser Parser,
	trackClient TrackClient,
	userRepoMux wrappers.UserRepoMux,
	readMux wrappers.UserWithReadPermissionMux,
	queue Queue,
	canSubCache CanSubmitCache,
	configName string) {

	h := handler{
		db, rSrv, revSrv, rt, userS, jobsS, ciq, parser,
		trackClient, userRepoMux, queue, canSubCache, configName}
	readMux.HandleFuncR("GET "+routes.CommitPattern,
		h.handleGetCommit)
	readMux.HandleFuncR("GET "+routes.JobsPattern,
		h.handleGetCommitJobs)
	readMux.HandleFuncR("GET "+routes.JobCombinedOutPattern,
		h.handleGetJobCombinedOut)
	readMux.HandleFuncR("GET "+routes.ThreadsPattern,
		h.handleGetThreads)
	readMux.HandleFuncR("GET "+routes.DiffPattern,
		h.handleGetDiff)
	readMux.HandleFuncR("GET "+routes.FileBlobPattern,
		h.handleGetFile)
	userRepoMux.HandleFuncW("POST "+routes.CommitPattern,
		h.handlePostCommit)
	userRepoMux.HandleFuncW("POST "+routes.SubmitCommitPattern,
		h.handlePostSubmit)
	userRepoMux.HandleFuncW("POST "+routes.RollbackCommitPattern,
		h.handlePostRollback)
	userRepoMux.HandleFuncW("POST "+routes.RenameCommitPattern,
		h.handlePostRename)
	userRepoMux.HandleFuncW("POST "+routes.PostAddLgtmPattern,
		h.handlePostAddLgtm)
	userRepoMux.HandleFuncW("POST "+routes.PostRemoveLgtmPattern,
		h.handlePostRemoveLgtm)
	readMux.HandleFuncR("GET "+routes.ReviewDataPattern,
		h.handleGetReviewData)

	userRepoMux.HandleFuncW("POST "+routes.NewThreadPattern,
		h.handlePostNewThread)
	userRepoMux.HandleFuncW("POST "+routes.ThreadPattern,
		h.handlePostToThread)
	readMux.HandleFuncR("GET "+routes.ThreadCommentsPattern,
		h.handleGetComments)
	userRepoMux.HandleFuncW("POST "+routes.PostAddReviewersPattern,
		h.handlePostAddReviewers)
	userRepoMux.HandleFuncW("POST "+routes.PostRemoveReviewersPattern, h.handlePostRemoveReviewers)
	readMux.HandleFuncR("GET "+routes.GetReviewersPattern,
		h.handleGetReviewers)
	readMux.HandleFuncR("GET "+routes.GetCanSubmitCommitsPattern,
		h.handleGetCanSubmitCommits)
}

type CanSubmitResult struct {
	CanSubmit        bool
	CantSubmitReason string
}

type FrontendThread struct {
	Id             int64
	Type           string
	CommitVersion  uint64
	IsResolved     bool
	Filename       string
	Line           uint64
	IsLgtm         bool
	AuthorUsername string
	CreatedOn      time.Time
}

// commit id to canSubmitResult
type HandleGetCanSubmitCommitsResponse = map[string]CanSubmitResult

type Db interface {
	CreateNotification(writeCtx context.Context, userId int64, message string, assetPath string) error
}

type TrackClient interface {
	GetCombinedOutput(jobId string) (io.ReadCloser, error)
}

type Parser interface {
	ParseCiFile(ciFile []byte) (payloads []runnerlib.CiJob, ok bool, notOkMsg string)
	ParseCdFile(cdFile []byte) (payloads []runnerlib.CdJob, ok bool, notOkMsg string)
}

type Queue interface {
	Enqueue(payloadType string, payload []byte) error
}

type CiCdQueue interface {
	EnqueueCiCdRun(repoId, commitId, commitVersion uint64,
		trigger runnerlib.JobTrigger, w context.Context) (int64, error)
	GetCiCdLatestRunStatus(repoId, commitId, commitVersion uint64, r context.Context) (cicdqueue.CiCdStatus, error)
}

// "Best effort" cache for storing if commits can/cant be submitted
type CanSubmitCache interface {
	GetCanSubmit(commitId, commitVersion, topCommitId uint64) (canSubmit bool, cantSubReason server.CantSubmitReason, cacheFound bool)
	PutCanSubmit(commitId, commitVersion, topCommitId uint64, canSubmit bool, cantSubReason server.CantSubmitReason)
}

const CiJobsPageSize = 50

var _ = func() int {
	// This value is used in the webcomponents to preemptivelly know when there
	// are no more jobs to load. Make sure to update it there as well
	if CiJobsPageSize != 50 {
		panic("CiJobsPageSize changed - did you change the webcomponents?")
	}
	return 1
}()