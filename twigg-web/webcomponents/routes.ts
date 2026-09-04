import { TabName } from "./commit-display"
import { Commit, Pipeline, PipelineRef, User } from "./interfaces"

// Contants that are used in the backend to define routes
export const LeftVersionParamName = "left"
export const RightVersionParamName = "right"
export const FileParamName = "file"
export const CommentParameterName = "comment"
export const ResolvedParamName = "resolved"
export const IsResolvedParamValue = "1"
export const IsNotResolvedParamValue = "0"


export const TwiggLogoWhiteUrl = "/logo.png"
export const TwiggLogoBlackUrl = "/logo-black.png"

export const PlansPagePath = "/plans"
export const IsChoosingPlanForOrgParamName = "isChoosingPlanForOrg"


export function UrlToCommit(owner: string, repo: string, commitId: number, tabName: TabName): string {
    return `/${owner}/${repo}/c/${commitId}${tabName == "changes" ? "?tab=changes" : ""}`
}

// Use leftV=-1 to show parent on the left. Use rightV=-1 to show latest on
// right.
export function UrlToCommitVersion(owner: string, repo: string, commitId: number, leftV: number, rightV: number): string {
    let queryParams = "?tab=changes"
    if (leftV != -1){
        queryParams += `&left=${leftV}`
    }
    if (rightV != -1) {
        queryParams += `&right=${rightV}`
    }
    return `/${owner}/${repo}/c/${commitId}${queryParams}`
}

export function GetThreadsUrl(owner: string, repo: string, commitId: number): string {
    return `/${owner}/${repo}/c/${commitId}/threads`
}

export function GetCommentsUrl(owner: string, repo: string, commitId: number): string {
    return `/${owner}/${repo}/c/${commitId}/threads/comments`
}

export function UrlToPostNewThread(owner: string,
    repo: string, commitId: number,
    commitVersion: number, filename: string): string{
    return `/${owner}/${repo}/c/${commitId}/new-thread?version=${commitVersion}&file=${filename}`
}

// line is 1-based: i.e. 1 -> first line.
export function UrlToPostNewThreadOnLine(owner: string,
    repo: string, commitId: number,
    commitVersion: number, filename: string, line: number): string {
    return UrlToPostNewThread(owner, repo, commitId, commitVersion, filename)
        + `&line=${line}`
}

export function UrlToPostNewApprovalThread(owner: string,
    repo: string, commitId: number,
    commitVersion: number): string {
    return `/${owner}/${repo}/c/${commitId}/new-thread?version=${commitVersion}`
}

export function UrlToGetFile(owner: string,
    repo: string, commitId: number, version: number, filename: string): string {
    return `/${owner}/${repo}/c/${commitId}/blob?file=${filename}&version=${version}`
}

export function UrlToGetJobLogFile(owner: string,
    repo: string, commitId: number, jobId: string): string {
    return `/${owner}/${repo}/c/${commitId}/jobs/out?job-id=${encodeURIComponent(jobId)}`
}

export function UrlToPostToThread(owner: string,
    repo: string, commitId: number,
    threadId: number): string{
    return `/${owner}/${repo}/c/${commitId}/thread/${threadId}`
}

export function UrlToPostAddLgtm(owner: string,
    repo: string, commitId: number,
    commitVersion: number): string{
    return `/${owner}/${repo}/c/${commitId}/lgtm?version=${commitVersion}`
}
export function UrlToPostRemoveLgtm(owner: string,
    repo: string, commitId: number): string{
    return `/${owner}/${repo}/c/${commitId}/r-lgtm`
}

export function UrlToGetReviewData(owner: string,
    repo: string, commitId: number): string{
    return `/${owner}/${repo}/c/${commitId}/rev-data`
}

export function PathToPostRollback(owner: string,
    repo: string, commitId: number): string {
    return `/${owner}/${repo}/c/${commitId}/rollback`
}

export function GetJobsAfter(owner: string, repo: string,
    commitId: number, afterInternalJobId: number): string {
    if (afterInternalJobId == 0){
        return `/${owner}/${repo}/c/${commitId}/jobs`
    }
    return `/${owner}/${repo}/c/${commitId}/jobs?after-internal-job-id=${afterInternalJobId}`
}

export function UrlToRepo(owner: string, repoName: string): string {
    return `/${owner}/${repoName}`
}


export function PathToMoreSubmittedCommits(owner: string, repoName: string,
    startingAtCommitId: number): string {
    return `${UrlToRepo(owner, repoName)}/load-s?starting-at=${startingAtCommitId}`
}
export function PathToMorePendingCommits(owner: string, repoName: string,
    afterCommitId: number): string {
    return `${UrlToRepo(owner, repoName)}/load-p?after-commit=${afterCommitId}`
}

export function PathToSearchCommits(owner: string, repoName: string,
    searchQuery: string): string {
    return `${UrlToRepo(owner, repoName)}/search-c?q=${searchQuery}`
}


export function PathToRepoSettings(
    repoOwnerName: string, repoDisplayName: string): string{
    return `${UrlToRepo(repoOwnerName, repoDisplayName)}/settings`
}

export function PathToPipelineRefs(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${UrlToRepo(repoOwnerName, repoDisplayName)}/cd-refs`
}
export function PathToPipelineRefsAfter(
    repoOwnerName: string, repoDisplayName: string, ref: PipelineRef): string {
    return `${UrlToRepo(repoOwnerName, repoDisplayName)}/cd-refs?after-cd-ref-path=${encodeURIComponent(ref.Path)}&after-cd-name-path=${encodeURIComponent(ref.Name)}`
}
export function PathToPipelines(
    repoOwnerName: string, repoDisplayName: string, ref: PipelineRef): string {
    return `${PathToPipelineRefs(repoOwnerName, repoDisplayName)}/${encodeURIComponent(ref.Path)}/${encodeURIComponent(ref.Name)}`
}
export function PathToPipelinesAfter(
    repoOwnerName: string, repoDisplayName: string, pipeline: Pipeline): string {
    return `${PathToPipelineRefs(repoOwnerName, repoDisplayName)}/${encodeURIComponent(pipeline.Path)}/${encodeURIComponent(pipeline.Name)}?after-internal-job-id=${encodeURIComponent(pipeline.InternalId)}`
}
export function PathToPipeline(
    repoOwnerName: string, repoDisplayName: string, p: Pipeline): string {
    return `${PathToPipelineRefs(repoOwnerName, repoDisplayName)}/${encodeURIComponent(p.Path)}/${encodeURIComponent(p.Name)}/${p.Id}`
}
export function PathToPipelineStages(
    repoOwnerName: string, repoDisplayName: string, p: Pipeline): string {
    return `${PathToPipeline(repoOwnerName, repoDisplayName, p)}/stages`
}
export function PathToPipelineStageOutput(
    repoOwnerName: string, repoDisplayName: string, p: Pipeline, stage: number): string {
    return `${PathToPipelineStages(repoOwnerName, repoDisplayName, p)}/${stage}/out`
}
export function PathToPipelineStageIsCanceled(
    repoOwnerName: string, repoDisplayName: string, p: Pipeline, stage: number): string {
    return `${PathToPipelineStages(repoOwnerName, repoDisplayName, p)}/${stage}/is-cancelled`
}

export function PathToManualResumePipelineStage(
    repoOwnerName: string, repoDisplayName: string, p: Pipeline, stage: number): string {
    return `${PathToPipelineStages(repoOwnerName, repoDisplayName, p)}/${stage}/manual-resume`
}
export function PathToCancelPipelineStage(
    repoOwnerName: string, repoDisplayName: string, p: Pipeline, stage: number): string {
    return `${PathToPipelineStages(repoOwnerName, repoDisplayName, p)}/${stage}/cancel`
}

export function PathToManuallyLaunchPipeline(
    repoOwnerName: string, repoDisplayName: string,
    ref: PipelineRef, commitId: number, commitVersion: number): string {
    return `${UrlToRepo(repoOwnerName, repoDisplayName)}/cd-manual-launch/${encodeURIComponent(ref.Path)}/${encodeURIComponent(ref.Name)}/${commitId}/${commitVersion}`
}


export function PathToAddRepoPermission(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/add`
}

export function PathToRemoveRepoPermission(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/remove`
}

export function PathToArchiveRepo(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/archive`
}

export function PathToSetRepoPublic(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/set-public`
}

export function PathToSetRepoPrivate(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/set-private`
}

export function PathToSetRepoDescription(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/set-description`
}

export const RepoDescriptionParamName = "description"

export function PathToSetGitMirrorEnabled(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/git-mirror-enabled`
}
export function PathToSetGitMirrorUrl(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/git-mirror-url`
}

export const GitMirrorEnabledParamName = "git-mirror-enabled"

export function PathToSetRepoSecret(
    repoOwnerName: string, repoDisplayName: string): string {
    return `${PathToRepoSettings(repoOwnerName, repoDisplayName)}/repo-secret`
}
export function UrlToDeleteRepoSecret(repoOwnerName: string, repoDisplayName: string, secretName): string {
    return PathToSetRepoSecret(repoOwnerName, repoDisplayName) + `?${RepoSecretNameParamName}=${encodeURIComponent(secretName)}`
}
export const RepoSecretNameParamName = "repo-secret-name"
export const RepoSecretValueParamName = "repo-secret-value"

export function GetCsrfHeaders(){
    return {"X-Twigg-Csrf": getCookie("csrf") ?? ""}
}

export const CsrfFormName = "csrf_token"

export function GetCsrfFormValue(){
    return  getCookie("csrf") ?? ""
}

function getCookie(name) {
    const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
    return match ? match[2] : null
}


export const OrganizationNameParamName = "orgName"
export const OrganizationsPattern = `/orgs`
export const NewOrganizationPattern = `/new-org`
export const NewOrganizationNameParamName = `new-org-name`
export function PathToOrganization(orgName: string): string {
    return `/orgs/v1/org/${orgName}`
}
export function PathToGrantOwnerOrMemberPermToUserPattern(orgName: string): string {
    return `${PathToOrganization(orgName)}/add`
}
export function PathToRevokeOwnerOrMemberPermToUserPattern(orgName: string): string {
    return `${PathToOrganization(orgName)}/remove`
}
export const PermissionParamName = `permission`
export function PathToManageOrgSubscriptionWithStripe(org: User): string {
    // Paid plans are managed in stripe portal.
    // Else, the user chooses a plan in the /plans page
    if (org.PaymentPlan == "None" || org.PaymentPlan == "Free") {
        return WithQueryParams(PlansPagePath, {
            [OrganizationNameParamName]: org.Username,
            [IsChoosingPlanForOrgParamName]: "true",
        });
    }
    return `${PathToOrganization(org.Username)}/manage/subscription/with-stripe`
}

export const PostLoginUrl = `/login`
export const Logout = "/logout"
export const StartLoginWithGoogleOAuth = "/start-login-with-google-oauth"
export const StartLoginWithMicrosoftOAuth = "/start-login-with-microsoft-oauth"

export const DocumentationPage = "/docs/v/2"
export const TermsPage = "/terms"
export const PrivacyPage = "/privacy"

export const NewRepoUrl = '/new-repo'
export function PathToCreateRepoForOrg(org: User) {
    return `${NewRepoUrl}?${OrganizationNameParamName}=${org.Username}`;
}

export const HomeUrl = '/home'
export const UserEducationUrl = "/user-education"
export const UserEducationWelcomeWasShownUrl = "/user-education/welcome-was-shown"
export const UserSettings = "/user-settings"
export const GenerateCLIKey = "/generate-cli-key"
export const DeleteCLIKey = "/delete-cli-key"
export const CLIKeyPramName = "cli-key"
export const SetUsernameUrl ="/set-username"
export const SetUsernameParamName = "set-username"


export function ManageSubscriptionPath(u: User): string{
    // Paid plans are managed in stripe portal.
    // Else, the user chooses a plan in the /plans page
    if (u.PaymentPlan == "None" || u.PaymentPlan == "Free") {
        return PlansPagePath
    }
    return "/manage/subscription/with-stripe"
}

export const SubscribeTrialUrl = "/subscribe/trial"
export const SubscribeWithStripeUrl = `/subscribe/with-stripe`
export const StripePriceIdParamName = `stripe-price-id-param`
export const StripeQuantityParamName = "stripe-quantity-param"

export const NewRepoNameParameterName = `new-repo-name`
export const NewRepoDescriptionParameterName = `description`

export const DiscordSupportUrl = `https://discord.gg/udpz3faxwQ`

export const RequestCountsPath = "/admindash/request-counts"
export const LogsPath = "/admindash/logs"
export const PprofPath = "/admindash/pprof"

export function AdminDashRequeueDeadLetter(id: number): string{
    return `/admindash/requeue/dl/${id}`
}
export function UrlToGetNotifications(lastReadNotificationId?: number) {
    if (lastReadNotificationId) {
        return "/notifications?LastReadNotificationId=" +
            encodeURIComponent(String(lastReadNotificationId));
    }
    return "/notifications";
}
export function UrlToMarkNotificationRead() {
    return "/notifications/read";
}
export function UrlToMarkNotificationSeen() {
    return "/notifications/seen";
}
export function UrlToGetNotificationsUnseenCount() {
    return "/notifications/unseen-count";
}
export function UrlToMarkAllNotificationsSeen() {
    return "/notifications/seen-all";
}

export const GitMirrorUrlSecretName = "git-mirror-secret-ulr"

export function UrlToPostRenameCommit(repoOwner: string, repoName: string, commit: number): string {
    return `/${repoOwner}/${repoName}/c/${commit}/rename`
}
export function UrlToPostAddReviewers(repoOwner: string, repoName: string, commit: number): string {
    return `/${repoOwner}/${repoName}/c/${commit}/reviewers`
}
export function UrlToGetReviewers(repoOwner: string, repoName: string, commit: number): string {
    return `/${repoOwner}/${repoName}/c/${commit}/get-reviewers`
}
export function UrlToPostRemoveReviewers(repoOwner: string, repoName: string, commit: number): string {
    return `/${repoOwner}/${repoName}/c/${commit}/rm-reviewers`
}
export function UrlToCanSubmitCommits(owner: string, repo: string, commitIds: number[]): string {
    const params = commitIds.map(id => `c=${id}`).join('&')
    return `/${owner}/${repo}/can-submit-commits?${params}`
}


function WithQueryParams(
    path: string,
    params: Record<string, string>,
): string {
    const searchParams = new URLSearchParams(params);

    return `${path}?${searchParams.toString()}`;
}