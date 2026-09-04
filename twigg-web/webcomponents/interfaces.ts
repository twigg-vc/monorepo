import { html } from "lit";
import { UrlToRepo } from "./routes";

export interface Commit {
    L: number;
    Version: number;
    ParentL: number;
    Message: string;
    CreatedOn: string; // Go Time is serialized as a string
    IsSubmitted: boolean;
    HasRebaseConflicts: boolean;
    AuthorUsername: string;
    ReviewStatus: "missing-lgtm" | "missing-owners-approval" |"unresolved-comments" | "ready"
    HasDiffData: boolean;
    DiffDataLinesCreated: number;
    DiffDataLinesDeleted: number;
    DiffDataLinesModified: number;
    DiffDataFilesCreated: number;
    DiffDataFilesDeleted: number;
    DiffDataFilesModified: number;
}

// Thread as sent by the server (we use a modified one in the frontend)
export interface ServerThread {
    // (RepoId + commitId + threadId) uniquely identify a thread
    Id: number
    Type: "CommentsOnFileOnCommitVersion" | "CommentsOnCommitVersion" | "AddLGTM" | "RemoveLGTM"
    CommitVersion: number
    IsResolved: boolean
    Filename: string
    Line: number // 1-based. 0 means the thread is anchored to the whole file
    IsLgtm: boolean
    AuthorUsername: string
    CreatedOn: string // Go Time is serialized as a string
}


export interface Thread {
    Id: number // (RepoId + commitId + threadId) uniquely identify a thread
    Type: "CommentsOnFileOnCommitVersion" | "CommentsOnCommitVersion" | "AddLGTM" | "RemoveLGTM"
    CommitVersion: number
    Filename: string
    Line: number // 1-based. 0 means the thread is anchored to the whole file
    IsResolved: boolean
    Comments: Comment[]
    IsLgtm: boolean
    AuthorUsername: string
    CreatedOn: string // Go Time is serialized as a string
}


export interface Comment {
    ThreadId: number
    AuthorUsername: string
    Text: string
    T: string; // Go Time is serialized as a string
}


export interface ReviewData {
    Description: string
    ReviewStatus: "missing-lgtm" | "unresolved-comments" | "ready"
}

export interface User {
    Email: string
    Username: string
    PaymentPlan: "None" | "Solo" | "Team" | "Free"
	PlanQuantity: number
    HasOldCliKey: boolean
	TotalQuota: number
	QuotaUsed: number
    QuotaLimmitted: number
}

export interface UserEducation {
    WelcomeWasShown: boolean
}

// c = "created", d = "deleted", m = "modified"
export type FileStatus = "c" | "d" | "m";

export type JobStatus =
    "waiting-manual-start" |
    "waiting" |
    "queued" |
    "posted" |
    "running" |
    "success" |
    "fail" |
    "timeout" |
    "cancel" |
    "too-many-jobs" |
    "bad-file-format" |
    "bad-file-size" |
    "exceeds-plan-limits"

export interface Job {
    InternalId:    number
	RepoId:        number
	Commit:        number
	CommitVersion: number
	Path:          string
	Name:          string
    RunNumber:     number
    Status:        JobStatus
    CreatedTime:   string
    Id:            string
}

export interface PipelineRef {
	RepoId: number
	Path:   string
	Name:   string
}
export type PipelineStatus =
    "waiting-manual-start" |
    "running" |
    "success" |
    "fail" |
    "cancel"

export interface Pipeline{
    Id: string
	InternalId:     number
	RepoId:         number
	Commit:         number
	CommitVersion:  number
	Path:           string
	Name:           string
    RunNumber:      number
    NumberOfStages: number
	Status:         PipelineStatus
	CreatedTime:    string
    IsCreatedByUser: boolean
    CreatedByUsername: string
}
export interface PipelineStage {
	PipelineId:        string
	Name:              string
	Stage:             number
	CreatedTime:       string
	Status:            JobStatus
    IsResumedByUser:   boolean
    ResumedByUsername: string
}

export interface QueueItem {
    Id: number
    PayloadType: string
    Payload: string
    CreatedAt: string
    AvailableAt: string
    RetryCount: number
}
export interface DeadLetterItem {
    Id: number
    PayloadType: string
    Payload: string
    OriginalCreatedAt: string
    FailedAt: string
    RetryCount: number
}

export interface Secret { 
    Id: number
    Name: string 
};

export function GetSubscriptionDescription(
    target: User,
    maxTrackJobs: number,
    maxTrackMilliseconds: number,
): string {
    if (target.PaymentPlan === "None") {
        return "-"
    }
    return `${target.PaymentPlan} | ${getPlanSeatsString(target)} | ${formatBytes(target.QuotaUsed)}/${formatBytes(target.TotalQuota)} | ${getTrackLimitsDescription(maxTrackJobs, maxTrackMilliseconds)}`
}
function getTrackLimitsDescription(
    maxTrackJobs: number,
    maxTrackMilliseconds: number,
): string {
    if (maxTrackJobs <= 1) {
        return `Up to ${maxTrackJobs} parallel CI/CD jobs`
    }
    return `Up to ${maxTrackJobs} jobs or ${formatMilliseconds(maxTrackMilliseconds)} of parallel CI/CD`
}
function getPlanSeatsString(target: User): string {
    const n = GetPlanSeats(target)
    if (n === 1) {
        return "1 user"
    }
    return `${n} users`
}
export function GetPlanSeats(target: User): number {
    if (target.PaymentPlan === "Solo") {
        return 3
    }
    return target.PlanQuantity
}
function formatBytes(bytes) {
    if (bytes === 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    const value = bytes / Math.pow(1024, i)
    return `${value.toFixed(1)} ${units[i]}`
}
function formatMilliseconds(ms: number): string {
    const raw = Math.floor((ms / 60_000) * 1000) / 1000;

    const [intPart, fracPart = "0"] = raw.toString().split(".");

    // add thousand separators to integer part
    const formattedInt = Number(intPart).toLocaleString("en-US");

    return `${formattedInt}.${fracPart} min`;
}

export interface Repo {
    OwnerName:   string;
    DisplayName: string;
    Description?: string;
}

export function RenderRepo(r: Repo){
    return html`
        <a class="repo-link" href="${UrlToRepo(
            r.OwnerName, r.DisplayName)}">
            <div class="repo twigg-lift">
                <div class="repo-meta">
                    <h3 class="repo-name">${r.DisplayName}</h3>
                    <p class="repo-desc">
                        ${r.Description ? r.Description : 'No description'}
                    </p>
                </div>
                <span class="repo-arrow">›</span>
            </div>
        </a>
    `
}

export const OrgPermissionOwner = "3"
export const OrgPermissionMember = "4"

export function UsernameIsValid(username: string): { isValid: boolean; errorMsg ?: string } {
    if (username.length < 2) {
        return { isValid: false, errorMsg: "Username must be at least 2 characters" };
    }
    if (username.length > 20) {
        return { isValid: false, errorMsg: "Username can not be longer than 20 characters" };
    }

    const validUsername = /^[a-z][a-z0-9_-]+$/
    if (!validUsername.test(username)) {
        return { isValid: false, errorMsg: "Must start with letter. Only lowercase letters, numbers, dashes and underscores allowed" };
    }

    return { isValid: true };
}