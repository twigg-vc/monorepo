package routes

import (
	"fmt"
	"monorepo/twigg/client"
	"monorepo/twigg/commit"
	"net/http"
	"strings"
)

const (
	NewRepoNameParameterName        = `new-repo-name`
	NewRepoDescriptionParameterName = `description`

	DocumentationPage2 = "/docs/v/2"
	DocumentationPage  = "/docs"
	BlogPage           = "/blog"

	TermsPage   = "/terms"
	PrivacyPage = "/privacy"

	AdminDash                    = "/admindash"
	AdminDashRequestCounts       = AdminDash + "/request-counts"
	AdminDashMetricTimeSeries    = AdminDash + "/metric/ts/{name}"
	AdminDashLogs                = AdminDash + "/logs"
	AdminDashRequeueDeadLetter   = AdminDash + "/requeue/dl/{id}"
	BackupFilenameQueryParamName = "name"
	DeadLetterIdQueryParamName   = "id"

	LandingPage                     = "/"
	LoginPage                       = "/login"
	Logout                          = "/logout"
	StartLoginWithGoogleOAuth       = "/start-login-with-google-oauth"
	CallbackLoginWithGoogleOAuth    = "/google-oauth-callback"
	StartLoginWithMicrosoftOAuth    = "/start-login-with-microsoft-oauth"
	CallbackLoginWithMicrosoftOAuth = "/microsoft-oauth-callback"
	Home                            = "/home"
	WelcomePage                     = "/welcome"
	NeedUpgradePage                 = "/need-upgrade-page"

	UserEducation                = "/user-education"
	UserEducationWelcomeWasShown = "/user-education/welcome-was-shown"

	Notifications           = "/notifications"
	NotificationMarkRead    = "/notifications/read"
	NotificationMarkSeen    = "/notifications/seen"
	NotificationUnseenCount = "/notifications/unseen-count"
	NotificationMarkAllSeen = "/notifications/seen-all"

	UserSettings                    = "/user-settings"
	GenerateCLIKey                  = "/generate-cli-key"
	DeleteCLIKey                    = "/delete-cli-key"
	CliKeyParamName                 = "cli-key"
	SetUsernamePath                 = "/set-username"
	SetUsernameParamName            = "set-username"
	NewRepoUrl                      = "/new-repo"
	PlansPage                       = "/plans"
	IsChoosingPlanForOrgParamName   = "isChoosingPlanForOrg"
	SubscribeWithStripeUrl          = "/subscribe/with-stripe"
	SubscribeTrialUrl               = "/subscribe/trial"
	ManageSubscriptionWithStripeUrl = "/manage/subscription/with-stripe"
	StripePriceIdParamName          = "stripe-price-id-param"
	StripeQuantityParamName         = "stripe-quantity-param"
	StripeWebhook                   = "/stripe-webhook"

	OrganizationNameParamName            = "orgName"
	OrganizationsPattern                 = "/orgs"
	OrganizationPattern                  = "/orgs/v1/org/{orgName}"
	NewOrganizationPattern               = "/new-org"
	NewOrganizationNameParamName         = "new-org-name"
	GrantOwnerOrMemberPermToUserPattern  = OrganizationPattern + "/add"
	RevokeOwnerOrMemberPermToUserPattern = OrganizationPattern + "/remove"
	PermissionParamName                  = "permission"
	ManageOrgSubscriptionWithStripe      = OrganizationPattern + "/manage/subscription/with-stripe"

	StripeSuccessPaymentPath = "/success-payment"

	TrackWebhooksPath    = "/track-wh"
	TrackWebhooksSecrets = TrackWebhooksPath + "/secrets"

	TwiggLogoWhiteUrl = "/logo.png"
	TwiggLogoBlackUrl = "/logo-black.png"

	LogInEmailFieldName    = "email"
	LogInPasswordFieldName = "password"

	RepoNameParamName                         = "repo"
	RepoOwnerParamName                        = "owner"
	RepoPattern                               = "/{owner}/{repo}"
	RepoSearchCommitsPattern                  = RepoPattern + "/search-c"
	RepoSearchCommitsSeachQueryQueryParamName = "q" // name of the query parameter of the search query
	RepoPullPattern                           = RepoPattern + client.PullEndpoint
	RepoPushPattern                           = RepoPattern + client.PushEndpoint
	RepoSetServerIdPattern                    = RepoPattern + client.SetServerIdEndpoint
	RepoSettingsPattern                       = RepoPattern + "/settings"
	RepoLoadMoreSubmitted                     = RepoPattern + "/load-s"
	RepoLoadMorePending                       = RepoPattern + "/load-p"
	RepoTwiggDocPattern                       = RepoPattern + "/docs/"

	PipelineRefsPattern              = RepoPattern + "/cd-refs"
	RefPipelinesPattern              = PipelineRefsPattern + "/{path}/{name}"
	PipelineRefPathPathParamName     = "path" // name of the path parameter that specifies the `path` of a ref
	PipelineRefNamePathParamName     = "name" // name of the path parameter that specifies the `name` of a ref
	AfterRefPathQueryParamName       = "after-cd-ref-path"
	AfterRefNameQueryParamName       = "after-cd-name-path"
	PipelinePattern                  = RefPipelinesPattern + "/{pipelineid}"
	PipelineIdPathParamName          = "pipelineid"
	PipelineStagesPattern            = PipelinePattern + "/stages"
	PipelineStagePattern             = PipelineStagesPattern + "/{stage}"
	StagePathParamName               = "stage"
	PipelineStageCombinedOutPattern  = PipelineStagePattern + "/out"
	ManualResumePipelineStagePattern = PipelineStagePattern + "/manual-resume"
	CancelPipelineStagePattern       = PipelineStagePattern + "/cancel"
	PipelineStageIsCancelledPattern  = PipelineStagePattern + "/is-cancelled"

	ManuallyLaunchPipelinePattern            = RepoPattern + "/cd-manual-launch/{path}/{name}/{c}/{v}"
	LaunchPipelineCommitIdPathParamName      = "c"
	LaunchPipelineCommitVersionPathParamName = "v"

	RepoAddPermissionPattern    = RepoSettingsPattern + "/add"
	RepoRemovePermissionPattern = RepoSettingsPattern + "/remove"
	RepoArchivePattern          = RepoSettingsPattern + "/archive"
	RepoSetPublicPattern        = RepoSettingsPattern + "/set-public"
	RepoSetPrivatePattern       = RepoSettingsPattern + "/set-private"
	RepoSetDescriptionPattern   = RepoSettingsPattern + "/set-description"
	RepoGitMirrorEnabledPattern = RepoSettingsPattern + "/git-mirror-enabled"
	RepoGitMirrorUrlPattern     = RepoSettingsPattern + "/git-mirror-url"
	RepoSettingsSecret          = RepoSettingsPattern + "/repo-secret"
	RepoDescriptionParamName    = "description"
	GitMirrorEnabledParamName   = "git-mirror-enabled"
	RepoSecretNameParamName     = "repo-secret-name"
	RepoSecretValueParamName    = "repo-secret-value"
	UsernameParameterName       = "username"

	CommitParamName                  = "c"
	CommitPattern                    = RepoPattern + "/c/{c}"
	ReviewDataPattern                = CommitPattern + "/rev-data"
	JobsPattern                      = CommitPattern + "/jobs"
	JobCombinedOutPattern            = JobsPattern + "/out"
	JobIdQueryParamName              = "job-id"
	AfterInternalJobIdQueryParamName = "after-internal-job-id"

	SubmitCommitPattern   = CommitPattern + "/submit"
	RollbackCommitPattern = CommitPattern + "/rollback"
	RenameCommitPattern   = CommitPattern + "/rename"

	DiffPattern     = CommitPattern + "/diff"
	FileBlobPattern = CommitPattern + "/blob"

	ThreadIdParamName = "id"
	NewThreadPattern  = CommitPattern + "/new-thread"
	ThreadPattern     = CommitPattern + "/thread/{id}"

	ThreadsPattern        = CommitPattern + "/threads"
	ThreadCommentsPattern = ThreadsPattern + "/comments"

	PostAddLgtmPattern    = CommitPattern + "/lgtm"
	PostRemoveLgtmPattern = CommitPattern + "/r-lgtm"

	GetReviewersPattern        = CommitPattern + "/get-reviewers"
	PostAddReviewersPattern    = CommitPattern + "/reviewers"
	PostRemoveReviewersPattern = CommitPattern + "/rm-reviewers"

	GetCanSubmitCommitsPattern = RepoPattern + "/can-submit-commits"
)

// Used to setup server routes (with the *Patters methods) and get the
// routes/parameters for requests.
type Router interface {
	Commit(repoOwnerName, repoDisplayName string, cId commit.LocalId,
		hasLeftVersion bool, leftVersion uint64,
		hasRightVersion bool, rightVersion uint64) string
	GetCommitParameter(r *http.Request) (commit.LocalId, error)

	Submit(repoOwnerName, repoDisplayName string, cId commit.LocalId) string

	Diff(repoOwnerName, repoDisplayName string, cId commit.LocalId,
		hasLeftVersion bool, leftVersion uint64,
		hasRightVersion bool, rightVersion uint64, file string) string

	NewThread(repoOwnerName, repoDisplayName string, cId commit.LocalId) string
	Thread(repoOwnerName, repoDisplayName string, cId commit.LocalId, threadId int) string

	Threads(repoOwnerName, repoDisplayName string, cId commit.LocalId) string
	Comments(repoOwnerName, repoDisplayName string, cId commit.LocalId) string

	GetFileParam(r *http.Request) string
	GetLineParameter(r *http.Request) (hasLine bool, line uint64)
	GetLeftVersionParameter(r *http.Request) (hasV bool, v uint64)
	GetRightVersionParameter(r *http.Request) (hasV bool, v uint64)
	GetVersionParameter(r *http.Request) (hasV bool, v uint64)
}

func New() Router {
	return router{}
}

func PathToOrganization(orgName string) string {
	return fmt.Sprintf("/orgs/v1/org/%s", orgName)
}

var _ = func() int {
	if !strings.Contains(RepoPattern, RepoNameParamName) {
		panic("RepoPattern should contain RepoNameParamName")
	}
	if !strings.Contains(RepoPattern, RepoOwnerParamName) {
		panic("RepoPattern should contain RepoOwnerParamName")
	}
	if !strings.Contains(RepoPullPattern, RepoPattern) {
		panic("RepoPullPattern should contain RepoPattern")
	}
	if !strings.Contains(RepoPushPattern, RepoPattern) {
		panic("RepoPushPattern should contain RepoPattern")
	}
	if !strings.Contains(CommitPattern, CommitParamName) {
		panic("CommitPattern should contain CommitParamName")
	}
	if !strings.Contains(SubmitCommitPattern, CommitPattern) {
		panic("SubmitCommitPattern should contain CommitPattern")
	}
	if !strings.Contains(DiffPattern, CommitPattern) {
		panic("DiffPattern should contain CommitPattern")
	}
	if !strings.Contains(ThreadPattern, ThreadIdParamName) {
		panic("ThreadPattern should contain ThreadIdParamName")
	}
	if !strings.Contains(PostAddLgtmPattern, CommitPattern) {
		panic("PostAddLgtmPattern should contain CommitPattern")
	}
	if !strings.Contains(PostAddReviewersPattern, CommitPattern) {
		panic("PostAddReviewersPattern should contain CommitPattern")
	}
	if !strings.Contains(RefPipelinesPattern, PipelineRefPathPathParamName) {
		panic("RefPipelinesPattern should contain PathOfPipelinePathParamName")
	}
	if !strings.Contains(RefPipelinesPattern, PipelineRefNamePathParamName) {
		panic("RefPipelinesPattern should contain PipelineRefNamePathParamName")
	}
	if !strings.Contains(PipelinePattern, PipelineIdPathParamName) {
		panic("PipelinePattern should contain PipelineIdPathParamName")
	}
	if !strings.Contains(PipelineStagePattern, StagePathParamName) {
		panic("PipelineStagePattern should contain StagePathParamName")
	}
	if !strings.Contains(OrganizationPattern, OrganizationNameParamName) {
		panic("OrganizationPattern should contain OrganizationNameParamName")
	}
	if !strings.Contains(GrantOwnerOrMemberPermToUserPattern, OrganizationPattern) {
		panic("GrantOwnerOrMemberPermToUserPattern should contain OrganizationPattern")
	}
	if !strings.Contains(RevokeOwnerOrMemberPermToUserPattern, OrganizationPattern) {
		panic("RevokeOwnerOrMemberPermToUserPattern should contain OrganizationPattern")
	}
	if !strings.Contains(ManageOrgSubscriptionWithStripe, OrganizationPattern) {
		panic("ManageOrgSubscriptionWithStripe should contain OrganizationPattern")
	}
	if !strings.Contains(PathToOrganization("{"+OrganizationNameParamName+"}"), OrganizationPattern) {
		panic("PathToOrganization should contain OrganizationPattern")
	}
	return 0
}()

// SuperPoliteResponseToBadActors is the text response we'll use to requests
// that clearly come from bad actors - such as those trying to access an
// admin-only url that is not visible in any UI.
// We keep it as a const in case we change our stance about how polite we need
// to be to them.
const SuperPoliteResponseToBadActors = "fuck off"