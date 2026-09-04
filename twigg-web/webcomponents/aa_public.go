package webcomponents

import (
	"encoding/json"
	"fmt"
	"monorepo/twigg-web/cicdqueue"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/secrets"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"net/http"
	"time"

	g "monorepo/maragudk/gomponents"
)

// Returns the whole HTML that contains the necessary setup and the main
// <twigg-app> with the provided body contents.
func Page(hideNavBar bool, featureFlags featureflags.Flags, bodyContents ...g.Node) g.Node {
	return page(hideNavBar, featureFlags, bodyContents...)
}

// Same as Page, but with a custom <title> instead of the default "Twigg".
func PageWithTitle(title string, hideNavBar bool, featureFlags featureflags.Flags, bodyContents ...g.Node) g.Node {
	return pageWithTitle(title, hideNavBar, featureFlags, bodyContents...)
}

// Returns the <twigg-login-page> component
func Login(wrongLoginInfo bool) g.Node {
	return login(wrongLoginInfo)
}

func WelcomePage() g.Node {
	return welcomePage()
}

func NeedUpgradePage() g.Node {
	return needUpgradePage()
}

// Returns the <home-page> component
func Home(u user.User, myRepos []FrontendRepo,
	sharedRepos []FrontendRepo) g.Node {
	return home(u, myRepos, sharedRepos)
}

// Returns the <user-settings> component
func UserSettings(u user.User,
	maxTrackJobs int,
	maxTrackMilliseconds int64) g.Node {
	return userSettings(u, maxTrackJobs,
		maxTrackMilliseconds)
}

// Returns the <organizations-page> component
func OrganizationsPage(orgs []user.User) g.Node {
	return organizationsPage(orgs)
}

// Returns the <organization-page> component
func OrganizationPage(org user.User,
	orgMaxTrackJobs int,
	orgMaxTrackMilliseconds int64,
	usersWithOwnerPermission []user.User,
	usersWithMemberPermission []user.User,
	currentUserIsOrgOwner bool,
	orgRepos []repo.Repo) g.Node {
	return organizationPage(org,
		orgMaxTrackJobs,
		orgMaxTrackMilliseconds,
		usersWithOwnerPermission,
		usersWithMemberPermission,
		currentUserIsOrgOwner,
		orgRepos,
	)
}

// Returns the <set-username-page> component
func SetUsernamePage() g.Node {
	return setUsernamePage()
}

// Returns the <new-repo> component.
// Pass orgName="" when creating a personal repo.
func NewRepo(orgName string) g.Node {
	return g.El("new-repo",
		g.If(orgName != "", g.Attr("OrgName", orgName)),
	)
}

type RepositorySettingsMember struct {
	Username string
	Role     string
}

// Returns the <repo-settings> component
func RepoSettings(
	repoOwnerName, repoDisplayName, description string, isGitMirrorEnabled bool,
	gitMirrorUrl string, membersList []RepositorySettingsMember,
	secrets []secrets.SecretRef, isPublic bool) g.Node {
	membersJson, err := json.Marshal(membersList)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal members list:%s", err))
	}
	secretsJson, err := json.Marshal(secrets)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal secrets:%s", err))
	}

	return g.El("repo-settings",
		g.Attr("RepoOwnerName", repoOwnerName),
		g.Attr("RepoName", repoDisplayName),
		g.Attr("Description", description),
		g.If(isGitMirrorEnabled, g.Attr("IsGitMirrorEnabled")),
		g.Attr("GitMirrorUrl", gitMirrorUrl),
		g.If(isPublic, g.Attr("IsPublic")),
		g.Attr("Members", string(membersJson)),
		g.Attr("Secrets", string(secretsJson)),
	)
}

// Returns the HTML element that shows the repository pending/submitted commits
func RepoDisplay(
	repoOwnerName, repoDisplayName string, description string,
	pending, submitted []FrontendCommit,
	haveMorePendingCommitsToFetch bool) g.Node {
	return repoDisplay(
		repoOwnerName, repoDisplayName, description, pending, submitted, haveMorePendingCommitsToFetch)
}

// Returns a <plans-cards> component. If isChoosingPlanForOrg=false pass org as
// a empty user (user.User{})
func PlansCards(
	u user.User,
	stripeSoloPriceId string,
	stripeTeamPriceId string,
	isChoosingPlanForOrg bool,
	org user.User,
) g.Node {
	return plansCards(u, stripeSoloPriceId, stripeTeamPriceId, isChoosingPlanForOrg, org)
}

// Returns the HTML element that shows the commit
func CommitDisplay(
	repoOwnerName,
	repoDisplayName string,
	commitVersions []commit.Commit, // [v0, v1, ...]
	parentOfEachVersion []commit.Commit, // [parentOfV0, parentOfV1, ...]
	children []commit.LocalId,
	reviewStatus review.ReviewStatus,
	latestParentIsSubmitted bool,
	submitWouldConflict bool,
	hasLgtmFromCurrentUser bool,
	lgtmAuthors []string,
	postSubmitUrl string,
	hasLeftVersion bool,
	leftVersion uint64,
	hasRightVersion bool,
	rightVersion uint64,
	description, postDescriptionUrl string,
	diffFileNames, diffStatus, getDiffUrls []string,
	tooManyDiffs bool,
	tab string, // tab can be "feed"/"changes"
	commitAuthorUser user.User,
	currentUser *user.User, //current user can be nil
	ciStatus cicdqueue.CiCdStatus,
) g.Node {
	return commitDisplay(repoOwnerName, repoDisplayName, commitVersions, parentOfEachVersion,
		children, reviewStatus, latestParentIsSubmitted,
		submitWouldConflict, hasLgtmFromCurrentUser, lgtmAuthors, postSubmitUrl,
		hasLeftVersion, leftVersion, hasRightVersion, rightVersion,
		description, postDescriptionUrl,
		diffFileNames, diffStatus, getDiffUrls, tooManyDiffs, tab,
		commitAuthorUser, currentUser, ciStatus)
}

// Returns a <cl-description> web-component
func ClDescription(description, postDescriptionUrl string) g.Node {
	return g.El(
		"cl-description",
		g.Attr("description", description),
		g.Attr("postDescriptionUrl", postDescriptionUrl),
	)
}

// Returns the value of the description in a post request made by the
// <cl-description> web-component.
func ParsePostClDescription(r *http.Request) string {
	return r.FormValue("description")
}

// Adds a handler that handles GET requests to return twigg.css
func AddHandler(mux wrappers.RlMux) {
	addHandler(mux)
}

// Converts a ReviewStatus into the string representation used by the webcomp.
func ReviewStatusString(r review.ReviewStatus) string {
	return reviewStatusString(r)
}

type FrontendRepo struct {
	OwnerName   string
	DisplayName string
	Description string
}

func NewFrontendRepo(ownerName string, r repo.Repo) FrontendRepo {
	return FrontendRepo{
		OwnerName:   ownerName,
		DisplayName: r.DisplayName,
		Description: r.Description,
	}
}

type FrontendCommit struct {
	L                     uint64
	Version               uint64
	ParentL               uint64
	Message               string
	CreatedOn             time.Time
	IsSubmitted           bool
	HasRebaseConflicts    bool
	AuthorUsername        string
	ReviewStatus          string
	HasDiffData           bool
	DiffDataLinesCreated  int64
	DiffDataLinesDeleted  int64
	DiffDataLinesModified int64
	DiffDataFilesCreated  int64
	DiffDataFilesDeleted  int64
	DiffDataFilesModified int64
}

// Converts commit.Commit to a frontend interpretation of it
func CommitToFrontend(c commit.Commit, authorUsername string,
	reviewStatus review.ReviewStatus) FrontendCommit {
	return commitToFrontend(c, authorUsername, reviewStatus)
}

type FrontendQueueItem struct {
	Id          int64
	PayloadType string
	Payload     string
	CreatedAt   string
	AvailableAt string
	RetryCount  int64
}
type FrontendDeadLetterItem struct {
	Id                int64
	PayloadType       string
	Payload           string
	OriginalCreatedAt string
	FailedAt          string
	RetryCount        int64
}

func AdminDash(uptime time.Duration,
	allocMb, heapInUseMb, sysMb float64, numGcRuns uint32,
	nUsers int64, allUsers []user.User,
	allQueueItems []FrontendQueueItem,
	allDeadLetterItems []FrontendDeadLetterItem) g.Node {
	frontendUsers := make([]frontendUser, len(allUsers))
	for i, u := range allUsers {
		frontendUsers[i] = userToFrontendUser(u)
	}
	return g.El(
		"admin-dash",
		g.Attr("Uptime", fmt.Sprintf("%v", uptime)),
		g.Attr("AllocMb", fmt.Sprintf("%.2f", allocMb)),
		g.Attr("HeapInUseMb", fmt.Sprintf("%.2f", heapInUseMb)),
		g.Attr("SysMb", fmt.Sprintf("%.2f", sysMb)),
		g.Attr("NumGcRuns", fmt.Sprintf("%d", numGcRuns)),
		g.Attr("NumUsers", fmt.Sprintf("%d", nUsers)),
		g.Attr("LatestUsers", string(marshalOrDie(frontendUsers))),
		g.Attr("QueueItems", string(marshalOrDie(allQueueItems))),
		g.Attr("DeadLetterItems", string(marshalOrDie(allDeadLetterItems))),
	)
}

func NewOrganization() g.Node {
	return g.El("new-organization")
}