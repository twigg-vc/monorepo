package webcomponents

import (
	"embed"
	"encoding/json"
	"fmt"
	"monorepo/buildmeta"
	"monorepo/twigg-web/cacheheaders"
	"monorepo/twigg-web/cicdqueue"
	"monorepo/twigg-web/featureflags"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/review"
	"monorepo/twigg-web/services/user"
	"monorepo/twigg-web/webcomponents/bundles"
	"monorepo/twigg-web/wrappers"
	"monorepo/twigg/commit"
	"net/http"

	g "monorepo/maragudk/gomponents"
	h "monorepo/maragudk/gomponents/html"
)

func page(hideNavBar bool, featureFlags featureflags.Flags, bodyContents ...g.Node) g.Node {
	return pageWithTitle("Twigg", hideNavBar, featureFlags, bodyContents...)
}

func pageWithTitle(title string, hideNavBar bool, featureFlags featureflags.Flags, bodyContents ...g.Node) g.Node {
	if hideNavBar {
		bodyContents = append(bodyContents, g.Attr("HideNavBar", "true"))
	}
	featureFlagsJson, err := json.Marshal(featureFlags)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal featureFlags:%s", err))
	}
	bodyContents = append(bodyContents, g.Attr("FeatureFlags", string(featureFlagsJson)))
	bodyContents = append(bodyContents, g.Attr("Version", buildmeta.Version))
	return h.Doctype(
		h.HTML(
			h.Head(
				h.TitleEl(g.Text(title)),
				h.Meta(
					h.Charset("utf-8"),
				),
				h.Meta(
					h.Name("viewport"),
					h.Content("width=device-width, initial-scale=1"),
				),
				bundles.Header(),
				h.Link(
					h.Rel("stylesheet"),
					h.Href(fmt.Sprintf("/twigg-%s.css", buildmeta.Version)),
				),
				g.Raw(`
				<style>
					html, body {
					height: 100%;
					margin: 0;
					padding: 0;
					}
				</style>`),
			),
			h.Body(
				g.El("twigg-app", bodyContents...),
			),
		))
}

func login(wrongLoginInfo bool) g.Node {
	return g.El("twigg-login-page",
		g.If(wrongLoginInfo, g.Attr("WrongLoginInfo")),
	)
}

func welcomePage() g.Node {
	return g.El("welcome-page")
}

func needUpgradePage() g.Node {
	return g.El("need-upgrade-page")
}

func home(u user.User, myRepos, sharedRepos []FrontendRepo) g.Node {
	if len(myRepos) == 0 {
		myRepos = []FrontendRepo{}
	}
	myReposJson, err := json.Marshal(myRepos)
	if err != nil {
		panic("Unable to Marshal myRepos")
	}

	if len(sharedRepos) == 0 {
		sharedRepos = []FrontendRepo{}
	}
	sharedReposJson, err := json.Marshal(sharedRepos)
	if err != nil {
		panic("Unable to Marshal sharedRepos")
	}

	return g.El(
		"home-page",
		passUserAsAttribute("User", u),
		g.Attr("MyRepos", string(myReposJson)),
		g.Attr("SharedRepos", string(sharedReposJson)),
	)

}

func repoDisplay(repoOwnerName, repoDisplayName string,
	description string,
	pendingCommitsMarshaled,
	submittedCommitsMarshaled []FrontendCommit,
	haveMorePendingCommitsToFetch bool) g.Node {

	pendingJson, err := json.Marshal(pendingCommitsMarshaled)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal pending:%s", err))
	}
	submittedJson, err := json.Marshal(submittedCommitsMarshaled)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal submitted:%s", err))
	}

	return g.El(
		"repo-display",
		g.Attr("RepoOwnerName", repoOwnerName),
		g.Attr("RepoName", repoDisplayName),
		g.Attr("Description", description),
		g.Attr("PendingCommits", string(pendingJson)),
		g.Attr("SubmittedCommits", string(submittedJson)),
		g.If(haveMorePendingCommitsToFetch, g.Attr("HaveMorePendingCommitsToFetch")),
	)
}

func plansCards(
	u user.User,
	stripeSoloPriceId string,
	stripeTeamPriceId string,
	isChoosingPlanForOrg bool,
	org user.User,
) g.Node {
	return g.El("plans-cards",
		passUserAsAttribute("User", u),
		g.Attr("StripeSoloPriceId", stripeSoloPriceId),
		g.Attr("StripeTeamPriceId", stripeTeamPriceId),
		g.If(isChoosingPlanForOrg, g.Attr("isChoosingPlanForOrg")),
		g.If(isChoosingPlanForOrg, passUserAsAttribute("Org", org)),
	)
}

func commitDisplay(
	repoOwnerName string,
	repoDisplayName string,
	commitVersions []commit.Commit,
	parentOfEachVersion []commit.Commit,
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
	diffFileNames, diffStatus, getDiffUrls []string, tooManyDiffs bool,
	tab string,
	commitAuthorUser user.User,
	currentUser *user.User, //current user can be nil
	ciStatus cicdqueue.CiCdStatus) g.Node {

	if currentUser == nil {
		currentUser = &user.User{}
	}
	cBytes, err := json.Marshal(commitVersions)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal commits:%s", err))
	}
	pBytes, err := json.Marshal(parentOfEachVersion)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal parent commits:%s", err))
	}
	if children == nil {
		children = []commit.LocalId{}
	}
	childrenBytes, err := json.Marshal(children)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal commit children:%s", err))
	}
	diffFileNamesJson, err := json.Marshal(diffFileNames)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal diffFileNames:%s", err))
	}
	diffStatusJson, err := json.Marshal(diffStatus)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal diffStatus:%s", err))
	}
	getDiffUrlsJson, err := json.Marshal(getDiffUrls)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal getDiffUrls:%s", err))
	}
	lgtmAuthorsJson, err := json.Marshal(lgtmAuthors)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal lgtmAuthors:%s", err))
	}
	if len(diffFileNames) != len(getDiffUrls) {
		// Len must be equal. One contains the name and the other the
		// url to get the unified diff string
		panic("len(diffFileNames) != len(getDiffUrls)")
	}
	if tab != "feed" && tab != "changes" {
		panic("tab must be feed/changes")
	}

	return g.El(
		"commit-display",
		g.Attr("RepoOwnerName", repoOwnerName),
		g.Attr("RepoName", repoDisplayName),
		g.Attr("CommitVersions", string(cBytes)),
		g.Attr("CommitParents", string(pBytes)),
		g.Attr("Children", string(childrenBytes)),
		g.If(hasLeftVersion,
			g.Attr("LeftVersion", fmt.Sprintf("%d", leftVersion))),
		g.If(hasRightVersion,
			g.Attr("RightVersion", fmt.Sprintf("%d", rightVersion))),
		g.Attr("Description", description),
		g.Attr("PostUrl", postDescriptionUrl),
		g.Attr("DiffFileNames", string(diffFileNamesJson)),
		g.Attr("DiffStatus", string(diffStatusJson)),
		g.Attr("DiffUrls", string(getDiffUrlsJson)),
		g.If(tooManyDiffs, g.Attr("TooManyDiffs", "true")),
		g.If(submitWouldConflict, g.Attr("SubmitWouldConflict")),
		g.If(hasLgtmFromCurrentUser, g.Attr("HasLgtmFromCurrentUser")),
		g.Attr("PostSubmitUrl", postSubmitUrl),
		g.Attr("TabName", tab),
		g.Attr("ReviewStatus", reviewStatusString(reviewStatus)),
		g.If(latestParentIsSubmitted, g.Attr("LatestParentIsSubmitted")),
		passUserAsAttribute("CommitAuthorUser", commitAuthorUser),
		passUserAsAttribute("CurrentUser", *currentUser),
		g.Attr("LgtmAuthors", string(lgtmAuthorsJson)),
		g.Attr("CiStatus", string(ciStatus)),
	)
}

func addHandler(mux wrappers.RlMux) {
	mux.HandleFunc(fmt.Sprintf("GET /twigg-%s.css", buildmeta.Version), handleGet)
}

//go:embed twigg.css
var files embed.FS

func handleGet(w http.ResponseWriter, r *http.Request) {
	cacheheaders.MediumCache(w)
	http.ServeFileFS(
		w,
		r,
		files,
		"twigg.css")
}

func reviewStatusString(r review.ReviewStatus) string {
	var reviewStatusString string
	switch r {
	case review.ReviewStatus_MissingLgtm:
		reviewStatusString = "missing-lgtm"
	case review.ReviewStatus_MissingOwnersApproval:
		reviewStatusString = "missing-owners-approval"
	case review.ReviewStatus_Ready:
		reviewStatusString = "ready"
	case review.ReviewStatus_Unresolved:
		reviewStatusString = "unresolved-comments"
	default:
		panic("unknown review status")
	}
	return reviewStatusString
}
func userSettings(u user.User,
	maxTrackJobs int,
	maxTrackMilliseconds int64) g.Node {
	return g.El(
		"user-settings",
		passUserAsAttribute("User", u),
		g.Attr("MaxTrackJobs", fmt.Sprintf("%d", maxTrackJobs)),
		g.Attr("MaxTrackMilliseconds", fmt.Sprintf("%d", maxTrackMilliseconds)),
	)
}
func setUsernamePage() g.Node {
	return g.El("set-username-page")
}

func organizationsPage(orgs []user.User) g.Node {

	frontendOrgs := make([]frontendUser, len(orgs))
	for i, u := range orgs {
		frontendOrgs[i] = userToFrontendUser(u)
	}

	return g.El(
		"organizations-page",
		g.Attr("Orgs", string(marshalOrDie(frontendOrgs))),
	)
}

func organizationPage(org user.User,
	orgMaxTrackJobs int,
	orgMaxTrackMilliseconds int64,
	usersWithOwnerPermission []user.User,
	usersWithMemberPermission []user.User,
	currentUserIsOrgOwner bool,
	orgRepos []repo.Repo) g.Node {
	// Owners
	frontendUsersWithOwnerPermission := make([]frontendUser, len(usersWithOwnerPermission))
	for i, u := range usersWithOwnerPermission {
		frontendUsersWithOwnerPermission[i] = userToFrontendUser(u)
	}
	// Members
	frontendUsersWithMemberPermission := make([]frontendUser, len(usersWithMemberPermission))
	for i, u := range usersWithMemberPermission {
		frontendUsersWithMemberPermission[i] = userToFrontendUser(u)
	}
	frontendOrgRepos := make([]FrontendRepo, len(orgRepos))
	for i, repo := range orgRepos {
		frontendOrgRepos[i] = NewFrontendRepo(org.Username, repo)
	}
	return g.El(
		"organization-page",
		passUserAsAttribute("Org", org),
		g.Attr("OrgMaxTrackJobs", fmt.Sprintf("%d", orgMaxTrackJobs)),
		g.Attr("OrgMaxTrackMilliseconds", fmt.Sprintf("%d", orgMaxTrackMilliseconds)),
		g.If(currentUserIsOrgOwner, g.Attr("CurrentUserIsOrgOwner")),
		g.Attr("UsersWithOwnerPermission", string(marshalOrDie(frontendUsersWithOwnerPermission))),
		g.Attr("UsersWithMemberPermission", string(marshalOrDie(frontendUsersWithMemberPermission))),
		g.Attr("OrgRepos", string(marshalOrDie(frontendOrgRepos))),
	)
}

// Helper function to pass user as:
// g.Attr(attributeName, string(userToFrontendUser()))
func passUserAsAttribute(attributeName string, u user.User) g.Node {
	return g.Attr(attributeName, string(marshalOrDie(userToFrontendUser(u))))
}

type frontendUser struct {
	Email          string
	Username       string
	PaymentPlan    string
	PlanQuantity   int64
	HasOldCliKey   bool
	TotalQuota     int64
	QuotaUsed      int64
	QuotaLimmitted int64
}

func userToFrontendUser(u user.User) frontendUser {
	pp := "None"
	if u.HasSub() {
		switch u.SelfPaidSubscription {
		case user.Subscription_Trial:
			pp = "Free"
		case user.Subscription_Team:
			pp = "Team"
		case user.Subscription_Solo:
			pp = "Solo"
		default:
			panic("invalid user payment plan")
		}
	}
	return frontendUser{
		Email:          u.Email,
		Username:       u.Username,
		PaymentPlan:    pp,
		PlanQuantity:   u.SelfPaidSubscriptionQuantity,
		HasOldCliKey:   u.CliKeyHash != "",
		TotalQuota:     u.TotalQuota,
		QuotaUsed:      u.QuotaUsed,
		QuotaLimmitted: u.QuotaLimmitted,
	}
}

func commitToFrontend(c commit.Commit, AuthorUsername string,
	reviewStatus review.ReviewStatus) FrontendCommit {

	reviewStatusStr := ""
	if reviewStatus == review.ReviewStatus_MissingLgtm {
		reviewStatusStr = "missing-lgtm"
	}
	if reviewStatus == review.ReviewStatus_MissingOwnersApproval {
		reviewStatusStr = "missing-owners-approval"
	}
	if reviewStatus == review.ReviewStatus_Ready {
		reviewStatusStr = "ready"
	}
	if reviewStatus == review.ReviewStatus_Unresolved {
		reviewStatusStr = "unresolved-comments"
	}

	return FrontendCommit{
		L:                     c.L,
		Version:               c.Version,
		ParentL:               c.ParentL,
		Message:               c.Message,
		CreatedOn:             c.CreatedOn,
		IsSubmitted:           c.IsSubmitted,
		HasRebaseConflicts:    c.HasRebaseConflicts,
		AuthorUsername:        AuthorUsername,
		ReviewStatus:          reviewStatusStr,
		HasDiffData:           c.HasDiffData,
		DiffDataLinesCreated:  c.DiffDataLinesCreated,
		DiffDataLinesDeleted:  c.DiffDataLinesDeleted,
		DiffDataLinesModified: c.DiffDataLinesModified,
		DiffDataFilesCreated:  c.DiffDataFilesCreated,
		DiffDataFilesDeleted:  c.DiffDataFilesDeleted,
		DiffDataFilesModified: c.DiffDataFilesModified,
	}
}

func marshalOrDie(x any) []byte {
	b, err := json.Marshal(x)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal %v: %s", x, err))
	}
	return b
}