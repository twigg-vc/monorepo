import { html, css, LitElement, TemplateResult } from 'lit';
import { TwiggCss } from './css';
import { Thread, Commit, Comment, ServerThread, ReviewData, User, Job } from './interfaces'
import { GetCommentsUrl, GetCsrfHeaders, GetThreadsUrl, UrlToCommit, UrlToCommitVersion, UrlToGetReviewData, UrlToPostNewThread, UrlToRepo, PathToPostRollback, GetJobsAfter, UrlToPostAddReviewers, UrlToGetReviewers, UrlToPostRemoveReviewers, UrlToPostRenameCommit } from './routes';
import { NewComment } from './comments';
import { GetFeatureFlags } from './feature-flags';
import { FormatDateTime } from "./helpers";
import { MinDurationTimer } from './min-duration-timer';


export const FirstCommitMsg = "First commit"

export type TabName = "feed" | "changes" | "ci"

// Returns true if the commit message marks the commit as WIP.
// Same logic in the backend.
export function IsArchivedCommit(message: string) {
    if (!message) return false
    return message.startsWith('#ARCHIVED')
}

export function IsWipCommit(message: string) {
     // A commit is considered WIP when the message starts with "wip"
     // (case-insensitive) and the next character is NOT a letter or number.
     // This prevents matching messages like "wip1" or "wipFix",
     // while allowing "WIP", "wip ", "wip:" etc.
     const wipRegex = /^wip(?![a-z0-9])/i
     if (!message) return false
     return wipRegex.test(message.trim())
}

// DiffCounts contains the data shown as "FilesChanged files +Plus -Minus"
interface DiffCounts {
    Plus: number
    Minus: number
    FilesChanged: number
}

// Helper to compute the diffcounts from a commit
function getCommitDiffCounts(commit: Commit): DiffCounts {
    return {
        Plus: commit.DiffDataLinesCreated + commit.DiffDataLinesModified,
        Minus: commit.DiffDataLinesDeleted + commit.DiffDataLinesModified,
        FilesChanged: commit.DiffDataFilesCreated +
            commit.DiffDataFilesDeleted +
            commit.DiffDataFilesModified,
    }
}

export class CommitDisplay extends LitElement {
    declare RepoOwnerName: string
    declare RepoName: string
    // Use -1 to show parent in the left
    declare LeftVersion: number
    // Use -1 to show latest on the right
    declare RightVersion: number
    // Contains each version of the commit v0, v1, ...
    declare CommitVersions: Commit[]
    // Contains parent of each commit version
    declare CommitParents: Commit[]
    // Commits whose latest version has this commit as parent
    declare Children: number[]
    declare Description: string
    declare PostUrl: string
    declare DiffFileNames: string[];
    declare DiffStatus: string[];
    declare DiffUrls: string[];
    declare TooManyDiffs: boolean;
    declare SubmitWouldConflict: boolean;
    declare LatestParentIsSubmitted: boolean;
    declare ReviewStatus: "missing-lgtm" | "missing-owners-approval" |"unresolved-comments" | "ready"
    // Indicates the latest version is LGTM by the user who is viewing the page
    declare HasLgtmFromCurrentUser: boolean;
    declare PostSubmitUrl: string;
    declare CommitAuthorUser: User;
    declare CurrentUser: User;
    declare Reviewers: string[];
    declare LgtmAuthors: string[];
    declare Jobs: Job[];
    declare CiStatus: "prepared" | "started";
    declare showAddReviewerModal: boolean;

    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        CommitVersions: { type: Array },
        CommitParents: { type: Array },
        Children: { type: Array },
        GetThreadsUrl: { type: String },
        LeftVersion: { type: Number },
        RightVersion: { type: Number },
        Description: { type: String },
        PostUrl: { type: String },
        DiffFileNames: { type: Array },
        DiffStatus: { type: Array },
        DiffUrls: { type: Array },
        TooManyDiffs: { type: Boolean },
        SubmitStatus: { type: String },
        PostSubmitUrl: { type: String },
        TabName: { type: String },
        SubmitWouldConflict: { type: Boolean },
        LatestParentIsSubmitted: { type: Boolean },
        ReviewStatus: { type: String },
        HasLgtmFromCurrentUser: { type: Boolean },
        CommitAuthorUser: { type: Object },
        CurrentUser: { type: Object },
        Reviewers: { type: Array },
        LgtmAuthors: { type: Array },
        Jobs: { type: Array },
        CiStatus: { type: String },

        isLoading: { type: Boolean, state: true },
        isLoadingSubmitOrRollbackBtn: { type: Boolean, state: true },
        threads_: { type: Array, state: true },
        isLoadingThreads_: { type: Boolean, state: true },
        showRollbackModal: { type: Boolean, state: true },

        ShowCiTab: { type: Boolean },

        showAddReviewerModal: {type: Boolean, state: true },
        addReviewerError: { type: String, state: true },
        addReviewerLoading: { type: Boolean, state: true },
        getReviewersLoading: { type: Boolean, state: true },

        showRenameModal: { type: Boolean, state: true },
        renameLoading: { type: Boolean, state: true },
        renameError: { type: String, state: true },
        renameMessage: { type: String, state: true },

        rollbackError: { type: String, state: true },

        submitError: { type: String, state: true },
    };
    constructor() {
        super();
        this.RepoOwnerName = ""
        this.RepoName = ""
        this.CommitVersions = []
        this.CommitParents = []
        this.Children = []
        this.Description = ""
        this.PostUrl = ""
        this.DiffFileNames = []
        this.DiffStatus = []
        this.DiffUrls = []
        this.LeftVersion = -1
        this.RightVersion = -1
        this.SubmitWouldConflict = false
        this.HasLgtmFromCurrentUser = false
        this.PostSubmitUrl = ""
        this.isLoading = false
        this.isLoadingSubmitOrRollbackBtn = false
        this.threads_ = []
        this.isLoadingThreads_ = false
        this.showRollbackModal = false
        this.TabName = "feed"
        this.LatestParentIsSubmitted = false
        this.Reviewers = [];
        this.LgtmAuthors = []
        this.Jobs = []
        this.CiStatus = "prepared"
        this.showAddReviewerModal = false
        this.addReviewerError = ""
        this.addReviewerLoading = false
        this.getReviewersLoading = false

        this.showRenameModal = false
        this.renameLoading = false
        this.renameError = ""
        this.renameMessage = ""

        this.rollbackError = ""

        this.submitError = ""
    }
    declare private isLoading: boolean
    declare private isLoadingSubmitOrRollbackBtn: boolean
    declare private threads_: Thread[]
    declare private isLoadingThreads_: boolean
    declare private showRollbackModal: boolean
    declare private TabName: TabName
    declare private addReviewerError: string
    declare private addReviewerLoading: boolean
    declare private getReviewersLoading: boolean
    declare private showRenameModal: boolean
    declare private renameLoading: boolean
    declare private renameError: string
    declare private renameMessage: string
    declare private rollbackError: string
    declare private submitError: string

    connectedCallback() {
        super.connectedCallback();
        this.loadData();
        this.getReviewers();
    }

    render(){
        const latest = this.getLatestCommit()

        let msg = ""
        if (latest.L == 0){
            msg = FirstCommitMsg
        }else{
            msg = latest.Message
        }
        return html`
        <div class="top-part">
            <div class="crumbs">
                <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs Name=${this.RepoName} Link="${UrlToRepo(this.RepoOwnerName,this.RepoName)}"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs id="current-crumb" Name="c/${latest.L}" Link=""></bread-crumbs>
            </div>
            <div class="title">
                <div class="title-start">
                    <span class="commit-size-tag-span">
                        <commit-size-tag .Commit=${latest}></commit-size-tag>
                    </span>
                    <h1>${msg}</h1>
                    ${!latest.IsSubmitted ? html`
                        <button class="rename-btn" @click=${this.openRenameModal}>
                            <twigg-icon class="tab-icon" icon="PencilIcon"></twigg-icon>
                        </button>
                    ` : html``}
                </div>
                <div class="title-end">
                    <div>${this.renderSubmitOrRollbackBtn()}</div>
                    <div>${this.renderLgtmBtn()}</div>
                </div>
            </div>
            <div id="commit-tags">
                ${this.renderCommitTags()}
            </div>
            <div class="info-and-description">
                <div class="info">
                    <h3 class="info-title">Information:</h3>
                    <div class="info-table">
                        <span class="info-table-label">Author:</span>
                        <username-tag username=${this.CommitAuthorUser.Username}></username-tag>
                        <span class="info-table-label">Last updated:</span>
                        <span>${FormatDateTime(latest.CreatedOn)}</span>
                        ${this.renderChildrenRow()}
                        ${this.renderParentRow()}
                        ${this.renderChangesRow(latest)}
                        <span class="info-table-label">Reviewers:</span>
                        <span class="info-users-list">
                            ${this.getReviewersLoading ? html`<simple-loader></simple-loader>` : html``}
                            ${this.Reviewers.map(r => this.renderReviewerPill(r))}
                            <button
                                class="add-reviewer-btn"
                                ?disabled=${this.showAddReviewerModal}
                                @click=${this.openAddReviewerModal}
                            >+add</button>
                        </span>
                        <span class="info-table-label">LGTM:</span>
                        ${this.LgtmAuthors && this.LgtmAuthors.length > 0 ? html`
                            <span class="info-users-list">
                                ${this.LgtmAuthors.map(u => html`
                                    <username-tag username=${u}></username-tag>
                                `)}
                            </span>
                        ` : ''}
                    </div>
                </div>
                <div class="description">
                    <h3 class="info-title">Description:</h3>
                    <cl-description
                    .description=${this.getDisplayedDescription()}
                    .postDescriptionUrl=${this.PostUrl}
                    .canEdit=${!latest.IsSubmitted}
                    >
                    </cl-description>
                </div>
            </div>
        </div>

        ${this.renderTabs()}
        
        ${this.renderRollbackModal()}

        ${this.renderAddReviewerModal()}

        ${this.renderRenameModal()}
        `
    }
    private renderChildrenRow(): TemplateResult {
        var children = undefined
        if (this.Children.length == 0) {
            children = html`<span class="no-children">none</span>`
        } else {
            children = this.Children.map(childL => html`
                <bread-crumbs Name="c/${childL}" Link=${UrlToCommit(this.RepoOwnerName, this.RepoName, childL, "feed")}></bread-crumbs>
            `)
        }
        return html`
            <span class="info-table-label">Children:</span>
            <span class="info-users-list">${children}</span>
        `
    }

    private renderParentRow(): TemplateResult {
        var parent = undefined
        const latest = this.getLatestCommit()

        if (latest.ParentL == 0) {
            parent = html`<span class="no-children">none</span>`
        } else {
            parent = html`
                <bread-crumbs Name="c/${latest.ParentL}" Link=${UrlToCommit(this.RepoOwnerName, this.RepoName, latest.ParentL, "feed")}></bread-crumbs>
            `
        }
        return html`
            <span class="info-table-label">Parent:</span>
            <span class="info-users-list">${parent}</span>
        `
    }

    private renderChangesRow(commit: Commit): TemplateResult {
        if (!commit.HasDiffData) {
            return html``
        }
        const counts = getCommitDiffCounts(commit)
        var filesText = ""
        if (counts.FilesChanged == 1) {
            filesText = "1 file"
        } else {
            filesText = `${counts.FilesChanged} files`
        }
        return html`
            <span class="info-table-label">Changes:</span>
            <span class="diff-counts">
                <span class="diff-counts-added">+${counts.Plus}</span>
                <span class="diff-counts-removed">-${counts.Minus}</span>
                <span class="diff-counts-files">${filesText}</span>
            </span>
        `
    }
    private renderAddReviewerModal() {
        if (!this.showAddReviewerModal) {
            return html``
        }

        return html`
        <div class="modal-backdrop" @click=${this.closeAddReviewerModal}>
            <div class="modal" @click=${(e: Event) => e.stopPropagation()}>
                <h3>Reviewers:</h3>

                <div class="reviewer-modal-list">
                    ${this.Reviewers.length === 0
                        ? html`<span class="reviewer-modal-empty">No reviewers yet</span>`
                        : this.Reviewers.map(r => this.renderReviewerPill(r))
                    }
                </div>

                <form class="reviewer-modal-form">
                    <input
                        class="reviewer-modal-input"
                        type="text"
                        placeholder="Username"
                        name="username"
                        ?disabled=${this.addReviewerLoading}
                    />
                    <button class="reviewer-modal-submit-btn" ?disabled=${this.addReviewerLoading} @click=${this.onAddReviewerSubmitted}>
                        ${this.addReviewerLoading ? html`<simple-loader></simple-loader>` : '+add'}
                    </button>
                </form>

                ${this.addReviewerError ? html`
                    <span class="reviewer-modal-error">${this.addReviewerError}</span>
                ` : ''}
            </div>
        </div>
    `
    }
    private renderReviewerPill(username: string) {
        return html`
        <div class="reviewer-item">
            <username-tag username=${username}></username-tag>
            <button
                class="reviewer-remove-btn"
                ?disabled=${this.addReviewerLoading}
                @click=${() => this.onRemoveReviewerClicked(username)}
            >×</button>
        </div>
        `
    }
    private async onRemoveReviewerClicked(username: string) {
        const tm = new MinDurationTimer()
        this.addReviewerLoading = true
        this.addReviewerError = ""

        const latestCommit = this.getLatestCommit()
        var errMsg = ""
        try {
            const resp = await fetch(
                UrlToPostRemoveReviewers(this.RepoOwnerName, this.RepoName, latestCommit.L),
                {
                    method: 'POST',
                    headers: {
                        ...GetCsrfHeaders(),
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ Usernames: [username] }),
                }
            )
            await tm.Wait()

            if (!resp.ok) {
                errMsg = (await resp.text()).trim()
                throw "err"
            }

            await this.getReviewers()

        } catch (err) {
            console.log("error removing reviewer: ", err)
            if (!errMsg) { errMsg = "Failed to remove reviewer." }
            alert(errMsg)
        } finally {
            this.addReviewerLoading = false
        }
    }
    private async onAddReviewerSubmitted(e: Event) {
        e.preventDefault()

        const modal = this.shadowRoot!.querySelector('.modal')!
        const input = modal.querySelector('.reviewer-modal-input') as HTMLInputElement
        const username = input.value.trim()

        if (!username) {
            this.addReviewerError = "Please enter a username."
            return
        }
        if (this.Reviewers.includes(username)) {
            this.addReviewerError = `${username} is already a reviewer.`
            return
        }

        const tm = new MinDurationTimer()
        this.addReviewerLoading = true
        this.addReviewerError = ""

        const latestCommit = this.getLatestCommit()
        try {
            const resp = await fetch(
                UrlToPostAddReviewers(this.RepoOwnerName, this.RepoName, latestCommit.L),
                {
                    method: 'POST',
                    headers: {
                        ...GetCsrfHeaders(),
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ Usernames: [username] }),
                }
            )
            await tm.Wait()

            if (!resp.ok) {
                const errText = await resp.text()
                if (errText) {
                    this.addReviewerError = errText
                } else {
                    this.addReviewerError = "Failed to add reviewer."
                }
                return
            }

            await this.getReviewers()

            // Clear input
            input.value = ""

        } catch (err) {
            console.log("error adding reviewer: ", err)
            this.addReviewerError = "Network error. Please try again."
        } finally {
            this.addReviewerLoading = false
        }
    }

    private async getReviewers() {
        this.getReviewersLoading = true
        const latestCommit = this.getLatestCommit()
        var errMsg = ""
        try {
            const resp = await fetch(
                UrlToGetReviewers(this.RepoOwnerName, this.RepoName, latestCommit.L),
                { method: 'GET' }
            )
            if (!resp.ok) {
                errMsg = (await resp.text()).trim()
                throw "err"
            }
            this.Reviewers = await resp.json()
        } catch (err) {
            console.log("error getting reviewers: ", err)
            if (!errMsg) { errMsg = "Failed to load reviewers." }
            alert(errMsg)
        } finally {
            this.getReviewersLoading = false
        }
    }

    private openAddReviewerModal() {
        this.showAddReviewerModal = true
    }

    private closeAddReviewerModal() {
        this.showAddReviewerModal = false
        this.addReviewerError = ""
    }

    private openRenameModal() {
        this.showRenameModal = true
        this.renameError = ""
        this.renameMessage = this.getLatestCommit().Message
    }

    private onRenameInputChanged(e: Event) {
        this.renameMessage = (e.target as HTMLInputElement).value
    }

    private closeRenameModal() {
        this.showRenameModal = false
        this.renameError = ""
    }

private renderRenameToWipBtn(message: string): TemplateResult {
        if (IsWipCommit(message)) {
            return html``
        } else {
            return html`
                <button class="quick-action-rename-btn" @click=${this.onMarkAsWip}>
                    <commit-status Status="WIP"></commit-status>
                </button>
            `
        }
    }

    private renderRenameToArchivedBtn(message: string): TemplateResult {
        if (IsArchivedCommit(message)) {
            return html``
        } else {
            return html`
                <button class="quick-action-rename-btn" @click=${this.onMarkAsArchived}>
                    <commit-status Status="archived"></commit-status>
                </button>
            `
        }
    }

    private async focusRenameInput() {
        await this.updateComplete
        this.shadowRoot?.querySelector<HTMLInputElement>('.reviewer-modal-input')?.focus()
    }

    private async onMarkAsWip() {
        this.renameMessage = 'WIP: ' + this.renameMessage
        await this.focusRenameInput()
    }

    private async onMarkAsArchived() {
        this.renameMessage = '#ARCHIVED ' + this.renameMessage
        await this.focusRenameInput()
    }

    private async onRenameSubmitted(e: Event) {
        e.preventDefault()

        const message = this.renameMessage.trim()

        if (!message) {
            this.renameError = "Message cannot be empty."
            return
        }

        if (message.length > 50) {
            this.renameError = `Message too long (${message.length}/50 characters).`
            return
        }

        if (message === this.getLatestCommit().Message) {
            this.renameError = "Message is unchanged."
            return
        }

        const tm = new MinDurationTimer()
        this.renameLoading = true
        this.renameError = ""

        const latest = this.getLatestCommit()
        try {
            const resp = await fetch(
                UrlToPostRenameCommit(this.RepoOwnerName, this.RepoName, latest.L),
                {
                    method: 'POST',
                    headers: {
                        ...GetCsrfHeaders(),
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ Message: message }),
                }
            )
            await tm.Wait()

            if (!resp.ok) {
                const errText = await resp.text()
                if (errText) {
                    this.renameError = errText
                } else {
                    this.renameError = "Failed to rename commit."
                }
                return
            }

            window.location.reload()

        } catch (err) {
            console.log("error renaming commit: ", err)
            this.renameError = "Network error. Please try again."
        } finally {
            this.renameLoading = false
        }
    }

    private renderRenameModal() {
        if (!this.showRenameModal) {
            return html``
        }

        const latest = this.getLatestCommit()

        return html`
        <div class="modal-backdrop" @click=${this.closeRenameModal}>
            <div class="modal rename-modal" @click=${(e: Event) => e.stopPropagation()}>
                <h3>Rename commit:</h3>
                <p>Mark as:</p>
                <div class="quick-actions-btns-modal-rename-content">
                    ${this.renderRenameToWipBtn(this.renameMessage)}
                    ${this.renderRenameToArchivedBtn(this.renameMessage)}
                </div>

                <form class="reviewer-modal-form" @submit=${this.onRenameSubmitted}>
                    <input
                        class="reviewer-modal-input"
                        type="text"
                        placeholder="New commit title"
                        name="message"
                        .value=${this.renameMessage}
                        ?disabled=${this.renameLoading}
                        @input=${this.onRenameInputChanged}
                    />
                    <button class="reviewer-modal-submit-btn" ?disabled=${this.renameLoading} @click=${this.onRenameSubmitted}>
                        ${this.renameLoading ? html`<simple-loader></simple-loader>` : 'Rename'}
                    </button>
                </form>

                ${this.renameError ? html`
                    <span class="reviewer-modal-error">${this.renameError}</span>
                ` : ''}
            </div>
        </div>
    `
    }

    private renderCommitTags(){
        let tags: TemplateResult[] = []
        const latest = this.getLatestCommit()
        if (IsArchivedCommit(latest.Message)) {
            tags.push(
                html`
                    <commit-status Status="archived">
                    </commit-status>
                `
            )
            return tags
        }
        if (IsWipCommit(latest.Message)) {
            tags.push(
                html`
                    <commit-status Status="WIP">
                    </commit-status>
                `
            )
        }
        if (this.SubmitWouldConflict) {
            tags.push(
                html`
                    <commit-status Status="will-conflict">
                    </commit-status>
                `
            )
        }
        if (this.getLatestCommit().HasRebaseConflicts) {
            tags.push(
                html`
                    <commit-status Status="has-conflict">
                    </commit-status>
                `
            )
        }
       
        if (this.ReviewStatus == "unresolved-comments") {
            tags.push(
                html`
                    <commit-status Status="unresolved-comments">
                    </commit-status>
                `
            )
        } 
        if (this.ReviewStatus == "missing-lgtm") {
            tags.push(
                html`
                    <commit-status Status="missing-lgtm">
                    </commit-status>
                `
            )
        }
        if (this.ReviewStatus == "missing-owners-approval") {
            tags.push(
                html`
                    <commit-status Status="missing-owners-approval">
                    </commit-status>
                `
            )
        }
        
        if (!this.LatestParentIsSubmitted) {
            tags.push(
                html`
                    <commit-status 
                    Status="pending-parent" 
                    Link="${UrlToCommit(this.RepoOwnerName,this.RepoName, this.getLatestCommit().ParentL, "feed")}"
                    CNumber="${this.getLatestCommit().ParentL}"
                    >
                    </commit-status>
                `
            )
        }
        if (this.getLatestCommit().IsSubmitted) {
            tags.push(
                html`
                    <commit-status Status="submitted">
                    </commit-status>
                `
            )
        }
        if (this.ReviewStatus == "ready" && !IsWipCommit(latest.Message)){
            if (!this.getLatestCommit().IsSubmitted){
                tags.push(
                    html`
                        <commit-status Status="ready">
                        </commit-status>
                    `
                )
            }
        }
        return tags
    }
    private renderTabs(){
        return html`
        <div
            class="tabs-and-content-container ${this.TabName}"
            @new-thread=${this.onNewThread}
            @new-comment=${this.onNewComment} 
        >
            <div class="tabs">
                <div
                class="tab ${this.TabName === "feed" ? "active" : ""}"
                @click=${() => this.TabName = "feed"}
                >
                    <twigg-icon class="tab-icon" icon="ClipboardCheck"></twigg-icon>
                    <span class="tab-text">Feed</span> 
                </div>
                <div
                class="tab ${this.TabName === "changes" ? "active" : ""}"
                @click=${() => this.TabName = "changes"}
                >
                    <twigg-icon class="tab-icon" icon="FileDiff"></twigg-icon>
                    <span class="tab-text">Changes</span> 
                </div>
                <div
                class="tab ${this.TabName === "ci" ? "active" : ""}"
                @click=${() => this.TabName = "ci"}
                >
                    <twigg-icon class="tab-icon" icon="Beaker"></twigg-icon>
                    <span class="tab-text">CI</span> 
                </div>
            </div>
            <div class="tabs-content">
                ${this.renderTabContent()}
            </div>
        </div>
        `;
    }
    private renderTabContent(){
        switch (this.TabName) {
            case "feed":
                return this.renderFeedTabContent()
            case "changes":
                return this.renderChangesTabContent()
            case "ci":
                return this.renderCiTabContent()
            default:
                throw "unknown tab";
        }
    }
    private renderFeedTabContent() {
        const unresolvedCount = this.threads_.filter(t => !t.IsResolved).length;
        const allCount = this.threads_.length;
        return html`
            <h2 style="padding-top: var(--space6)">
                Unresolved comments: ${unresolvedCount ? html`<span class="badge-yellow">${unresolvedCount}</span>` : ''}
            </h2>
            ${this.renderAllUnresolved()}

            <h2 style="padding-top: var(--space6)">
                All comments: <span class="badge-grey">${allCount}</span>
            </h2>
            ${this.renderAll()}
        `
    }
    private renderAllUnresolved(){
        if (this.isLoadingThreads_) {
            return html`<simple-loader></simple-loader>`
        }
        const unresolvedThreads = this.threads_.filter((th: Thread) => {
            return !th.IsResolved
        })
        if (unresolvedThreads.length == 0) {
            return html`
                <div class=feed-no-unresolved-comments-div>
                    <span class="feed-no-unresolved-comments-span">
                        No unresolved comments &#128578;
                    </span>
                </div>
            `
        }
        return html`${unresolvedThreads.map((thread: Thread) => {
            return this.renderThread(thread)
        })}`
    }
    private renderAll() {
        if (this.isLoadingThreads_) {
            return html`<simple-loader></simple-loader>`
        }
        return html`${this.threads_.map((thread: Thread) => {
            return this.renderThread(thread)
        })}`
    }

    private renderThread(thread: Thread){
        const threadCommit = this.CommitVersions.filter(
            (c, index) => c.Version == thread.CommitVersion)[0]
        return html`
        <div class="thread-container">
            <comment-thread
                .Thread=${thread}
                .RepoName=${this.RepoName}
                .RepoOwnerName=${this.RepoOwnerName}
                .Commit=${threadCommit}
                .CurrentUser=${this.CurrentUser}
                >
                <span slot="go-to">
                    ${this.renderGoToThread(thread)}
                </span>
            </comment-thread>
        </div>
        `
    }
    // Renders the component shown above the thread that, once clicked,
    // takes you to the thread on the changes tabs
    private renderGoToThread(thread: Thread){
        if (thread.Filename == "") {
           return html`
            <div class="feed-discussion-thread-header-div">
                <a href=${this.getThreadClickUrl(thread)}>
                    Version ${thread.CommitVersion}
                </a>
            </div>
            `
        }
        let displayText = `Version ${thread.CommitVersion} - ${thread.Filename}`
        if (thread.Line != 0){
            displayText += `:${thread.Line}`
        }
        return html`
        <div class="feed-comment-thread-header-div">
            <a href=${this.getThreadClickUrl(thread)}>
                ${displayText}
            </a>
        </div>
        `
    }

    private renderChangesTabContent(){
        return html`
            <version-selector
                .RepositoryName=${this.RepoName}
                .RepositoryOwner=${this.RepoOwnerName}
                .Versions=${this.CommitVersions}
                .Parents=${this.CommitParents}
                .LeftVersion=${this.LeftVersion}
                .RightVersion=${this.RightVersion}
                .Threads=${this.threads_}
            >
            </version-selector>
            <div class="changes-tab-bottom-part">
                ${this.renderVersionDiscussionThreads()}
                ${this.renderDiffs()}
                ${this.renderTooManyDiffs()}
            </div>
        `
    }
    private renderCiTabContent() {
        return html`
            <ci-list .RepoOwnerName=${this.RepoOwnerName} 
            .RepoName=${this.RepoName} 
            .CiStatus=${this.CiStatus} 
            .CommitServerId=${this.getLatestCommit().L}
            .CommitServerVersion=${this.getLatestCommit().Version}
            .Jobs=${this.Jobs}></ci-list>
        `
    }
    private renderLgtmBtn(){
        if (this.getLatestCommit().IsSubmitted){
            return html``
        }
        return html`
            <lgtm-btn
            RepoOwnerName=${this.RepoOwnerName}
            RepoName=${this.RepoName}
            .HasLgtm=${this.HasLgtmFromCurrentUser}
            .LatestCommit=${this.getLatestCommit()}
            @new-thread=${this.onNewThreadFromLgtmBtn}
            >
            </lgtm-btn>
        `
    }
    private renderSubmitOrRollbackBtn(){
        const latest = this.getLatestCommit()
        if (this.isLoadingSubmitOrRollbackBtn){
            return html`<simple-loader></simple-loader>`
        }
        if (latest.IsSubmitted) {
            return html`
                <button class="rollback-btn" @click=${this.openRollbackModal}>
                    <twigg-icon .icon=${"Rollback"}>Rollback</twigg-icon>
                </button>
            `
        }
        if (IsWipCommit(latest.Message)) {
            return html``
        }
        if (latest.HasRebaseConflicts) {
            return html``
        }
        if (this.SubmitWouldConflict) {
            return html``
        }
        if (!this.LatestParentIsSubmitted) {
            return html``
        }
        if (this.ReviewStatus == "missing-lgtm"){
            return html``
        }
        if (this.ReviewStatus == "missing-owners-approval") {
            return html``
        }
        if (this.ReviewStatus == "unresolved-comments"){
            return html``
        }
        if (this.ReviewStatus != "ready"){
            console.log(this.ReviewStatus)
            throw "unexpected review status"
        }
        return html`
        <div class="submit-container">
            <button class="submit-btn" @click=${this.onSubmitClicked}>
                <twigg-icon .icon=${"Rebase"}>Submit</twigg-icon>
            </button>
            ${this.submitError ? html`
                <span class="submit-error">${this.submitError}</span>
            ` : ''}
        </div>
        `
    }
    private renderDiffs(){
        if (this.DiffFileNames.length != this.DiffUrls.length){
            throw "num of files must equal num of urls to get diff"
        }
        return html`${this.DiffFileNames.map(
            (_, index) => this.renderDiff(index)
        )}`;
    }
    private renderDiff(i: number){
        return html`
            <diff-frame
            .RepoOwnerName=${this.RepoOwnerName}
            .RepoName=${this.RepoName}
            .filename=${this.DiffFileNames[i]}
            .diffGetUrl=${this.DiffUrls[i]}
            .LeftCommit=${this.getLeftCommit()}
            .RightCommit=${this.getRightCommit()}
            .LeftThreads=${this.getLeftThreads(i)}
            .RightThreads=${this.getRightThreads(i)}
            .CurrentUser=${this.CurrentUser}
            FileStatus=${this.DiffStatus[i]}
            ></diff-frame>
        `
    }
    private renderTooManyDiffs() {
        if (!this.TooManyDiffs){
            return html``
        }
        return html`<div>Whoa, that's a lot of files! We'd list them all but ran out of pixels..."</div>`
    }
    private renderVersionDiscussionThreads() {
        const leftV = this.getLeftCommit()
        let leftDiscussionThreads: Thread[] = []
        if (!this.leftCommitIsParent()){
            leftDiscussionThreads = this.threads_.filter((th: Thread) => {
                return th.Filename == "" && th.CommitVersion == leftV.Version
            })
        }
        const rightV = this.getRightCommit()
        const rightDiscussionThreads = this.threads_.filter((th: Thread) => {
            return th.Filename == "" && th.CommitVersion == rightV.Version
        })
        return html`
        <div class="version-discussion-side-by-side-threads">
            <div class="version-discussion-one-side-threads">
                ${ this.leftCommitIsParent() ? html`` :
                html`
                <h2 class="version-discussion-one-side-header">Discussions</h2>
                `
                }
                ${ this.leftCommitIsParent() ? html`` :
                html`
                <comment-threads
                .newThreadPostUrl=${
                UrlToPostNewThread(this.RepoOwnerName,
                    this.RepoName, leftV.L,
                    leftV.Version, "")}
                .Threads=${leftDiscussionThreads}
                .RepoOwnerName=${this.RepoOwnerName}
                .RepoName=${this.RepoName}
                .Commit=${leftV}
                .CurrentUser=${this.CurrentUser}>
                </comment-threads>
                `
                }
            </div>
            <div class="version-discussion-one-side-threads">
                <h2 class="version-discussion-one-side-header">Discussions</h2>
                <comment-threads
                .newThreadPostUrl=${
                UrlToPostNewThread(this.RepoOwnerName,
                        this.RepoName, rightV.L,
                        rightV.Version, "")}
                .Threads=${rightDiscussionThreads}
                .RepoOwnerName=${this.RepoOwnerName}
                .RepoName=${this.RepoName}
                .Commit=${rightV}
                .CurrentUser=${this.CurrentUser}>
                </comment-threads>
            </div>
        </div>
        `
    }

    private renderRollbackModal(){
        if (this.showRollbackModal){
            return html`
                <div class="modal-backdrop" @click=${this.closeRollbackModal}>
                    <div class="modal" @click=${(e: Event) => e.stopPropagation()}>
                        <h3>Are you sure?</h3>
                        <p>A new commit that reverts c/${this.getLatestCommit().L} will be created</p>
                        <div class="modal-buttons">
                            <button class="cancel-rollback-btn" @click=${this.closeRollbackModal}>
                                <twigg-icon .icon=${"XMark"}>Cancel</twigg-icon>
                            </button>
                            <button ?disabled=${this.isLoadingSubmitOrRollbackBtn} class="rollback-btn" @click=${this.onRollbackConfirmed}>
                                <twigg-icon .icon=${"Rollback"}>Yes</twigg-icon>
                            </button>
                        </div>
                        ${this.rollbackError ? html`
                            <span class="reviewer-modal-error">${this.rollbackError}</span>
                        ` : ''}
                    </div>
                </div>
            `
        }
        return html``
    }

    private getLatestCommit(): Commit{
        return this.CommitVersions[this.CommitVersions.length-1]
    }
    private getDisplayedDescription(): string{
        if (this.getLatestCommit().IsSubmitted && this.Description == ""){
            return "\`[no description]\`"
        }
        return this.Description
    }
    private leftCommitIsParent(): boolean {
        return this.LeftVersion == -1
    }
    private getLeftCommit(): Commit {
        if (this.LeftVersion == -1){
            if (this.RightVersion == -1){
                return this.CommitParents[this.CommitParents.length -1]
            }
            return this.CommitParents[this.RightVersion]
        }
        return this.CommitVersions[this.LeftVersion]
    }
    private getRightCommit(): Commit {
        if (this.RightVersion == -1) {
            return this.CommitVersions[this.CommitVersions.length - 1]
        }
        return this.CommitVersions[this.RightVersion]
    }
    // Returns the url that clicking on a thread should redirect to
    private getThreadClickUrl(th: Thread): string{
        // Always tries to put the version of the thread on the left
        // and the latest version on the right. Except if the thread version
        // is the latest (bc there's nothing to put on the right)
        // - in that case we put just the parent in the left and the
        // thread on the right
        const latestCommit = this.getLatestCommit()
        if (th.CommitVersion == latestCommit.Version){
            return UrlToCommitVersion(this.RepoOwnerName,this.RepoName, latestCommit.L,
                -1,th.CommitVersion
            )
        }
        return UrlToCommitVersion(this.RepoOwnerName,this.RepoName, latestCommit.L,
            th.CommitVersion, -1)
    }

    private async onSubmitClicked(){
        // Note: dont reset isLoadingSubmitOrRollbackBtn on success. Else we 
        // experience flickering bc we redirect the page on submit.
        // Just set it to false on errors
        this.isLoadingSubmitOrRollbackBtn = true
        this.submitError = ""
        try {
            const resp = await fetch(this.PostSubmitUrl, {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                const text = (await resp.text()).trim()
                this.submitError = text || "Failed to submit."
                this.isLoadingSubmitOrRollbackBtn = false
                return
            }
            // Remove any query parameters (in case any version is selected
            // on any side)
            window.location.href = window.location.origin +
                window.location.pathname +
                window.location.hash;
        } catch (error) {
            console.log("error when submitting: ", error)
            this.submitError = "Network error. Please try again."
            this.isLoadingSubmitOrRollbackBtn = false
        }
    }
    private async onRollbackConfirmed() {
        // Note: dont reset isLoadingSubmitOrRollbackBtn on success.
        // Else we experience flickering bc we redirect the page on rollback.
        // Just set it to false on errors
        this.isLoadingSubmitOrRollbackBtn = true
        this.rollbackError = ""
        try {
            const resp = await fetch(PathToPostRollback(
                this.RepoOwnerName, this.RepoName, this.getLatestCommit().L), {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            const text = (await resp.text()).trim();
            if (!resp.ok) {
                this.rollbackError = text || "Failed to create a rollback commit."
                this.isLoadingSubmitOrRollbackBtn = false
                return
            }
            window.location.assign(text);
        } catch (error) {
            console.log("error when creating a rollback commit.", error)
            this.rollbackError = "Network error. Please try again."
            this.isLoadingSubmitOrRollbackBtn = false
        }
    }
    private async loadData() {
        this.isLoading = true
        this.isLoadingThreads_ = true

        const latestCommit = this.getLatestCommit()

        var errMsg = ""
        var newThreads: Thread[] = []
        try {
            const resp = await fetch(GetThreadsUrl(this.RepoOwnerName,this.RepoName,
                latestCommit.L), {
                method: 'GET',
            });
            if (!resp.ok) {
                errMsg = (await resp.text()).trim()
                throw "err"
            }
            let newServerThreads: ServerThread[] = await resp.json();
            newThreads = new Array(newServerThreads.length)
            for (let i = 0; i < newServerThreads.length; i++) {
                newThreads[i] = {
                    Type: newServerThreads[i].Type,
                    Id: newServerThreads[i].Id,
                    CommitVersion: newServerThreads[i].CommitVersion,
                    Filename: newServerThreads[i].Filename,
                    Line: newServerThreads[i].Line,
                    IsResolved: newServerThreads[i].IsResolved,
                    Comments: [],
                    IsLgtm: newServerThreads[i].IsLgtm,
                    AuthorUsername: newServerThreads[i].AuthorUsername,
                    CreatedOn: newServerThreads[i].CreatedOn,
                }
            }
        } catch (error) {
            console.log("error getting threads: ", error)
            if (!errMsg) { errMsg = "Failed to load threads." }
            alert(errMsg)
            this.isLoading = false;
            this.isLoadingThreads_ = false;
            return;
        }
        errMsg = ""
        let comments: Comment[] = [];
        try {
            const resp = await fetch(GetCommentsUrl(this.RepoOwnerName,this.RepoName,
                latestCommit.L), {
                method: 'GET',
            });
            if (!resp.ok) {
                errMsg = (await resp.text()).trim()
                throw "err"
            }
            comments = await resp.json();
        } catch (error) {
            console.log("error getting comments: ", error)
            if (!errMsg) { errMsg = "Failed to load comments." }
            alert(errMsg)
            this.isLoading = false;
            this.isLoadingThreads_ = false;
            return;
        }
        // Add each comment to the respective thread.
        // TODO: this is currently O(n^2). Since they are sent in order we can
        // make it O(n).
        for (const cm of comments){
            let foundThread = false
            for (let i = 0; i< newThreads.length; i++){
                if (newThreads[i].Id == cm.ThreadId){
                    newThreads[i].Comments.push(cm)
                    foundThread = true
                    break
                }
            }
            if (!foundThread) {
                console.log("commend: ", cm)
                console.log("threads: ", newThreads)
                throw "thread not found"
            }
        }
        errMsg = ""
        let newJobs: Job[] = [];
        try {
            const resp = await fetch(GetJobsAfter(this.RepoOwnerName, this.RepoName,
                latestCommit.L, /*afterInternalJobId*/0), 
                {method: 'GET'},
            );
            if (!resp.ok) {
                errMsg = (await resp.text()).trim()
                throw "err"
            }
            newJobs = await resp.json();
        } catch (error) {
            console.log("error getting jobs: ", error)
            if (!errMsg) { errMsg = "Failed to load jobs." }
            alert(errMsg)
            this.isLoading = false;
            this.isLoadingThreads_ = false;
            return;
        }
        this.isLoading = false;
        this.Jobs = newJobs;
        this.threads_ = newThreads
        this.isLoadingThreads_ = false;
    }
    private async updateReviewData() {
        this.isLoading = true;
        var errMsg = ""
        let revData: ReviewData
        try {
            const resp = await fetch(UrlToGetReviewData(this.RepoOwnerName,this.RepoName,
                this.getLatestCommit().L), {
                method: 'GET',
            });
            if (!resp.ok) {
                errMsg = (await resp.text()).trim()
                throw "err"
            }
            revData = await resp.json();
            this.ReviewStatus = revData.ReviewStatus
            this.Description = revData.Description
        } catch (error) {
            console.log("error getting review data: ", error)
            if (!errMsg) { errMsg = "Failed to load review data." }
            alert(errMsg)
            return;
        } finally {
            this.isLoading = false;
        }
    }

    private onNewThreadFromLgtmBtn(event: CustomEvent<Thread>){
        const newThread = event.detail;
        this.updateLgtmAuthorsFromThread(newThread);
        this.onNewThread(event);
        this.updateReviewData();
    } 
    private updateLgtmAuthorsFromThread(th: Thread) {
        if (th.Type !== "AddLGTM" && th.Type !== "RemoveLGTM") {
            return;
        }

        const username = th.AuthorUsername;
        if (!username) return;

        if (!Array.isArray(this.LgtmAuthors)) {
            this.LgtmAuthors = [];
        }

        if (th.IsLgtm) {
            if (!this.LgtmAuthors.includes(username)) {
                this.LgtmAuthors = [...this.LgtmAuthors, username];
            }
        } else {
            this.LgtmAuthors = this.LgtmAuthors.filter(u => u !== username);
        }
    }

    private onNewThread(event: CustomEvent<Thread>){
        const newThread = event.detail
        this.threads_ = [...this.threads_, newThread]
        if (!newThread.IsResolved){
            this.updateReviewData()
        }
    }
    private onNewComment(event: CustomEvent<NewComment>) {
        const newComment = event.detail
        let modifiedThreadIndex = 0
        let foundModifiedThreadIndex = false
        for (let i = 0; i<this.threads_.length; i++){
            if (this.threads_[i].Id != newComment.C.ThreadId){
                continue
            }
            foundModifiedThreadIndex = true
            modifiedThreadIndex = i
        }
        if (!foundModifiedThreadIndex){
            console.log("newComment: ", newComment)
            console.log("this.threads: ", this.threads_)
            throw "could not find thread"
        }
        // Yes we need all this copying to force lit to re-render.
        // Using slices and objects in lit sucks.
        const oldThread = this.threads_[modifiedThreadIndex];
        const updatedThread: Thread = {
            ...oldThread,
            Comments: [...oldThread.Comments, newComment.C],
            IsResolved: newComment.ThreadIsResolved
        };
        this.threads_ = [
            ...this.threads_.slice(0, modifiedThreadIndex),
            updatedThread,
            ...this.threads_.slice(modifiedThreadIndex + 1),
        ];

        if (newComment.ChangedThreadResolveStatus){
            this.updateReviewData()
        }
    }

    // Returns the threads that are displayed on the left side of the i-th diff
    private getLeftThreads(i: number): Thread[]{
        return this.threads_.filter(
            (thread: Thread)=>{
                // No need to check commit id because threads can only be
                // posted to commit versions; not parents
                let leftV = this.getLeftCommit().Version
                return thread.Filename != "" &&
                thread.Filename == this.DiffFileNames[i] &&
                    thread.CommitVersion == leftV
            }
        )
    }
    // Returns the threads that are displayed on the right side of the i-th diff
    private getRightThreads(i: number): Thread[] {
        let rightThreads = this.threads_.filter(
            (thread: Thread) => {
                let rightV = this.getRightCommit().Version
                return thread.Filename != "" &&
                thread.Filename == this.DiffFileNames[i] &&
                    thread.CommitVersion == rightV
            }
        )
        return rightThreads
    }

    private openRollbackModal() {
        this.showRollbackModal = true;
    }

    private closeRollbackModal() {
        this.showRollbackModal = false;
        this.rollbackError = ""
    }

    static styles = [
        TwiggCss,
        css`
        .top-part{
            max-width: var(--size4);
            margin: auto;
        }
        .changes-tab-bottom-part{
            margin: 0px;
        }
        .title{
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: var(--space1);
        }
        .commit-size-tag-span {
            font-size: var(--space5);
            padding-right: var(--space1);
        }
        .rename-btn{
            background: transparent;
            border: none;
            cursor: pointer;
            padding: 0;
            color: var(--color-text-muted);
        }
        .rename-btn:hover{
            color: var(--color-primary);
        }
        .title-start{
            display: flex;
            align-items: center;
            gap: var(--space2);
        }
        .submit-btn{
            background: var(--color-success);
            color: var(--color-status-text);
            font-size: var(--space4);
            font-weight: var(--weight-bold);
        }
        .submit-container {
            display: flex;
            flex-direction: column;
            align-items: flex-end;
            gap: var(--space1);
        }
        .submit-error {
            color: var(--color-danger);
            font-size: var(--space4m);
        }
        .rollback-btn{
            background: var(--color-info);
            color: var(--color-status-text);
            font-size: var(--space4);
            font-weight: var(--weight-bold);
        }
        .submit-conflict-span{
            background: var(--color-danger);
            color: var(--color-status-text);
            padding: 0px;
            margin: 0px;
            font-weight: var(--weight-semi-bold);
        }
        .submit-conflict-container {
            position: relative;
            cursor: help;
            border-radius: var(--radius2);
            padding: var(--space1) var(--space3);
            background: var(--color-danger);
        }
        .submit-conflict-tooltip {
            visibility: hidden;
            background-color: var(--color-surface-alt);
            box-shadow: var(--shadow-surface-alt);
            border: 1px solid var(--color-primary-pop);
            color: var(--color-text);
            text-align: left;
            padding: 1ch 3ch;
            border-radius: 1ch;
            position: absolute;
            z-index: 9999;
            top: 100%;
            right: 0;

        }
        .submit-conflict-container:hover .submit-conflict-tooltip,
        .submit-conflict-container:focus-within .submit-conflict-tooltip {
            visibility: visible;
            opacity: 1;
        }
        /* Prevent line breaks inside tooltip paragraphs */
        .submit-conflict-tooltip p {
            white-space: nowrap;
        }
        /* Remove extra marging from ul */
        .submit-conflict-tooltip ul {
            margin-top: 0;
        }
        .submit-parent-must-be-first-span{
            padding: var(--space2);
            border-radius: var(--space4);
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            font-weight: var(--weight-semi-bold);
        }

        /**Side by side on arge screens; stacked on small screens */
        .info-and-description{
            display: flex;
            width: 100%;
            gap: 1rem;
            flex-wrap: wrap;
        }
        .info{
            flex: 1;
        }
        .description{
            flex: 1.5;
            max-width: 100%;
        }
        @media (max-width: 700px) {
            .info,
            .description {
                flex: 1 1 100%;
            }
        }
        .info-title{
            color: var(--color-primary);
            margin: var(--space2) 0px;
        }
        .info-table {
            display: grid;
            grid-template-columns: max-content auto; /* left column fits content, right fills remaining */
            gap: var(--space1) var(--space4); /* row gap, column gap */
            align-items: center; /* vertically center items */
        }
        .info-table-label {
            font-weight: bold;
        }
        .diff-counts {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            font-variant-numeric: tabular-nums;
        }
        .diff-counts-files {
            color: var(--color-text-muted);
            font-size: var(--space4m);
        }
        .diff-counts-added,
        .diff-counts-removed {
            font-size: var(--space4m);
        }
        .diff-counts-added {
            color: var(--diff-plus-count);
        }
        .diff-counts-removed {
            color: var(--diff-minus-count);
        }
        .add-reviewer-btn {
            font-size: var(--space3);
            padding: var(--space0) var(--space2);
            border-radius: var(--radius2);
            background: transparent;
            border: 1px dashed var(--color-border);
            border-radius: var(--radius0);
            color: var(--color-text);
            font-weight: var(--weight-semi-bold);
            font-size: var(--space4m);
            align-self: center;
            box-shadow: var(--shadow-surface);
        }
        .add-reviewer-btn:hover {
            background: var(--color-surface-alt);
            border-color: var(--color-primary);
            color: var(--color-primary);
        }
        .tabs-and-content-container{
            padding: var(--space4);
            margin: 0 auto;
            display: flex;
            flex-direction: column;
            align-items: center;
        }
        .feed{
            max-width: var(--size4);
        }
        .tabs {
            display: flex;
            width: 100%;
            border-bottom: 1px solid var(--color-border);
        }
        .tabs-content{
            width: 100%;
        }
        .tab-text{
            margin-left: var(--space2);
            font-size: var(--space4);
        }
        .tab-icon{
            font-size: var(--space4);
        }
        .tab {
            padding: var(--space2) var(--space4);
            cursor: pointer;
            border: none;
            background: none;
            font: inherit;
            outline: none;
            border-bottom: 2px solid transparent;
            position: relative;
            display: flex;
        }
        .tab.active {
            border-bottom-color: var(--color-primary);
            font-weight: var(--weight-bold);
            color: var(--color-primary);
            border-top: 1px solid var(--color-border);
            border-right: 1px solid var(--color-border);
            border-left: 1px solid var(--color-border);
            border-top-right-radius: var(--radius1);
            border-top-left-radius: var(--radius1);
        }
        .feed-comment-thread-header-div{
            display: flex;
            justify-content: center;
            align-items: center;
            gap: var(--space2);
        }
        .feed-no-unresolved-comments-div{
            display: flex;
            justify-content: center;
        }
        .feed-no-unresolved-comments-span{
            color: var(--color-text);
        }
        .feed-discussion-thread-header-div{
            display: flex;
            justify-content: center;
            align-items: center;
        }
        .version-discussion-side-by-side-threads{
            display: flex;
        }
        .version-discussion-one-side-threads{
           flex: 1;
           min-width: 0;
           max-width: 100%;
        }
        .version-discussion-one-side-header{
            text-align: center;
        }

        .badge-yellow{
            display:inline-flex; align-items:center; justify-content:center;
            min-width: calc(var(--fixedSpace5));          
            height:     calc(var(--fixedSpace5));
            padding: 0 var(--space3);                      
            border-radius: 999px;
            vertical-align: middle;
            transform: translateY(-0.08em);                
            font: inherit;
            font-weight: var(--weight-bold);
            font-size: var(--space4m);
            line-height: 1;
            background: var(--color-warning);
            color: var(--color-status-text);
        }
        .badge-grey{
            display:inline-flex; align-items:center; justify-content:center;
            min-width: calc(var(--fixedSpace5));
            height:     calc(var(--fixedSpace5));
            padding: 0 var(--space3);
            border-radius: 999px;
            vertical-align: middle;
            transform: translateY(-0.08em);
            font: inherit;
            font-weight: var(--weight-bold);
            font-size: var(--space4m);
            line-height: 1;
            background: var(--color-surface-alt);
            color: var(--color-text);
        }
        .title-end{
            display:flex;
            align-items: center;
            gap: var(--space3)
        }
        /* Thread links: brand color only */
        .feed-comment-thread-header-div a:any-link,
        .feed-discussion-thread-header-div a:any-link {
            color: var(--color-primary);
        }

        .feed-comment-thread-header-div a:hover,
        .feed-discussion-thread-header-div a:hover {
            color: var(--color-primary-pop);
        }

        .thread-container{
            margin-top: var(--space3);
        }

        .info-users-list {
            display: flex;
            flex-wrap: wrap;
            gap: var(--space1);
        }
        .no-children {
            color: var(--color-text-muted);
        }
        
        .cancel-rollback-btn{
            color: var(--color-status-text);
            font-size: var(--space4);
            font-weight: var(--weight-bold);
        }
        .reviewer-modal-list {
            display: flex;
            flex-wrap: wrap;
        }
        .reviewer-item {
            display: flex;
            align-items: center;
        }
        .reviewer-modal-empty {
            width: 100%;
            color: var(--color-text);
            font-style: italic;
            opacity: 0.6;
        }
        .rename-modal {
            min-width: var(--size2);
        }
        @media (max-width: 760px) {
            .rename-modal {
                min-width: unset;
            }
        }
        .reviewer-modal-form {
            display: flex;
            gap: var(--space0);
            margin-top: var(--space2);
            justify-content: center;
        }
        .reviewer-modal-input {
            flex: 1;
            padding: var(--space1) var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background: var(--color-surface);
            color: var(--color-text);
            font: var(--line-height);
        }
        .reviewer-modal-submit-btn {
            border-radius: var(--radius1);
            background: var(--color-primary);
            color: var(--color-text-on-primary);
            font-weight: var(--weight-bold);
        }
        .reviewer-modal-error {
            display: block;
            margin-top: var(--space2);
            color: var(--color-danger);
            font-size: var(--space4m);
            text-align: center;
        }
        .reviewer-remove-btn {
            background: transparent;
            border: none;
            color: var(--color-text-muted);
            font-size: var(--space4);
            font-weight: var(--weight-bold);
            cursor: pointer;
            padding: 0 var(--space1);
        }
        .reviewer-remove-btn:hover {
            opacity: 0.7;
        }
        .quick-actions-btns-modal-rename-content {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: var(--space2);
            margin-bottom: var(--space2);
        }
        .quick-action-rename-btn {
            background: transparent;
            border: none;
            padding: 0;
        }
        .quick-action-rename-btn:hover {
            color: var(--color-text);
        }
    `];
}
customElements.define('commit-display', CommitDisplay);
declare global {
    interface HTMLElementTagNameMap {
        'commit-display': CommitDisplay;
    }
}