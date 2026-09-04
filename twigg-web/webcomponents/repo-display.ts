import { html, LitElement, css } from 'lit';
import { keyed } from 'lit/directives/keyed.js';
import { TwiggCss } from './css';
import { FirstCommitMsg, IsWipCommit } from './commit-display'
import { Commit } from './interfaces'
import { UrlToRepo, PathToRepoSettings, UrlToCommit, PathToMoreSubmittedCommits, UrlToCanSubmitCommits, PathToMorePendingCommits } from './routes';
import { MinDurationTimer } from './min-duration-timer';
import './commit-graph';
import { GetFeatureFlags } from './feature-flags';
import { FormatRelativeTime } from './helpers';

export type TabName = "commits" | "commits graph" | "CD";

type CanSubmitByCommitId = Record<string, {
    CanSubmit: boolean
    CantSubmitReason: string
}>

// Flag to enable the show more btn
const enableShowMoreBtn = true

// Must match the limit enforced by handleGetCanSubmitCommits
// which rejects requests with more than 20 commit ids.
const maxCanSubmitCommitsPerRequest = 20


export class RepoDisplay extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        Description: { type: String },
        PendingCommits: { type: Array },
        SubmittedCommits: { type: Array },
        HaveMorePendingCommitsToFetch: { type: Boolean },

        isFetchingMorePendingCommits: { type: Boolean, state: true },
        fetchMorePendingCommitsFailed: { type: Boolean, state: true },
        isLoadingMoreSubmittedCommits: { type: Boolean, state: true },
        fetchMoreSubmittedCommitsFailed: { type: Boolean, state: true },
        TabName: { type: String },
        pendingCommitSubmitWillConflict: { type: Array, state: true },
        isLoadingWillConflict: { type: Boolean, state: true },
        fetchWillConflictFailed: { type: Boolean, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Description: string;
    declare PendingCommits: Commit[];
    declare SubmittedCommits: Commit[];
    declare HaveMorePendingCommitsToFetch: boolean;
    declare private isFetchingMorePendingCommits: boolean;
    declare private fetchMorePendingCommitsFailed: boolean;
    declare private isLoadingMoreSubmittedCommits: boolean;
    declare private fetchMoreSubmittedCommitsFailed: boolean;
    declare private TabName: TabName;
    declare private pendingCommitSubmitWillConflict: boolean[];
    declare private isLoadingWillConflict: boolean;
    declare private fetchWillConflictFailed: boolean;

    constructor() {
        super();
        this.RepoName = "";
        this.Description = "";
        this.PendingCommits = [];
        this.SubmittedCommits = [];
        this.HaveMorePendingCommitsToFetch = false;
        this.isFetchingMorePendingCommits = false;
        this.fetchMorePendingCommitsFailed = false;
        this.isLoadingMoreSubmittedCommits = false;
        this.fetchMoreSubmittedCommitsFailed = false;
        this.TabName = "commits";
        this.pendingCommitSubmitWillConflict = []
        this.isLoadingWillConflict = false
        this.fetchWillConflictFailed = false
    }

    // If set to true, will fetch "canSubmit?" for each pending commit.
    // This means that it'll show which commits would cause conflicts.
    fetchCanSubmitForPendingIsEnabled = true;


    render() {
        return html`
        <div class="main">
            <div class="crumbs">
                <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs id="current-crumb" Name=${this.RepoName} Link="${UrlToRepo(this.RepoOwnerName,this.RepoName)}"></bread-crumbs>
            </div>
            <div class="header">
                <a>
                    <twigg-icon class="icon-repo" icon="DataTable"></twigg-icon>
                </a>
                <div class="title">
                    <h1>${this.RepoName}</h1>
                    <p class="repo-desc">${this.Description || 'No description'}</p>
                </div>
                <div class="header-actions">
                    <a class="repo-settings-icon" href=${PathToRepoSettings(this.RepoOwnerName, this.RepoName)}>
                        <twigg-icon class="tab-icon" icon="Cog"></twigg-icon>
                    </a>
                    <details class="dropdown">
                    <summary>Clone</summary>
                    <div class="menu">
                        <p>Clone</p>
                        <div class="menu-item">
                            <code>tw clone ${this.RepoOwnerName}/${this.RepoName}</code>
                            <button class="copy-btn"
                            @click=${(e) => {
                                const codeEl = e.currentTarget.previousElementSibling;
                                if (codeEl) navigator.clipboard.writeText(codeEl.textContent);
                            }}
                            title="Copy cmd">
                                <twigg-icon icon="ContentCopy"></twigg-icon>
                            </button>
                        </div>
                    </div>
                    </details>
                </div>
            </div>

           ${this.renderTabs()}

        </div>
        `;
    }
    
    private renderTabs() {

        return html`
            <div class="tabs-and-content-container ${this.TabName}">
                <div class="tabs">
                    <div
                    class="tab ${this.TabName === "commits" ? "active" : ""}"
                    @click=${() => (this.TabName = "commits")}
                    >
                        <twigg-icon class="tab-icon" icon="Bars"></twigg-icon>
                        <span class="tab-text">Commits</span>
                    </div>
                        <div
                            class="tab ${this.TabName === "commits graph" ? "active" : ""}"
                            @click=${() => (this.TabName = "commits graph")}
                        >
                            <twigg-icon class="tab-icon" icon="Tree"></twigg-icon>
                            <span class="tab-text">Commit graph</span>
                    </div>
                    ${GetFeatureFlags().ShowCdJobs ?
                    html`
                    </div>
                        <div
                            class="tab ${this.TabName === "CD" ? "active" : ""}"
                            @click=${() => (this.TabName = "CD")}
                        >
                            <twigg-icon class="tab-icon" icon="RocketLaunch"></twigg-icon>
                            <span class="tab-text">CD</span>
                    </div>
                    `
                    : html``}
                </div>

                <div class="tabs-content">
                    ${this.renderTabContent()}
                </div>
            </div>
        `;
    }

    private renderTabContent() {
        switch (this.TabName) {
            case "commits":
                return html`
                    <section class="commit-section card">
                        <h2 style="padding-top: var(--space6)">
                            Pending Commits
                        </h2>
                        <div class="commit-list">
                            ${this.renderPendingCommitsList()}
                        </div>
                        <div class="view-more-btn-container">
                            ${this.renderFetchMorePendingCommitsBtn()}
                        </div>
                    </section>

                    <section class="commit-section card">
                        <h2>Submitted Commits</h2>
                        <div class="commit-list">
                            ${this.SubmittedCommits.map((c) => this.renderCommit(c, false))}
                        </div>
                        <div class="view-more-btn-container">
                            ${this.renderLoadMoreSubmittedCommitsBtn()}
                        </div>
                    </section>
                `;

            case "commits graph":
                return html`
                    <section class="commit-section card">
                        <h2  style="padding-top: var(--space6)">
                            Commit graph
                        </h2>
                        <p>
                            Shows commit relationships. Click opens a commit in 
                            a new tab (Ctrl+Click stays). 
                            Drag and zoom to navigate.
                        </p>
                        ${this.renderCommitGraphTab()}
                    </section>
                `;
            case "CD":
                return html`<repo-cd-tab
                    .RepoName=${this.RepoName}
                    .RepoOwnerName=${this.RepoOwnerName}>
                </repo-cd-tab>`;

            default:
                throw "unknown tab";
        }
    }
    private renderCommitGraphTab() {
        // Use lit keyed to ensure refresh when the commits change.
        const graphKey = `${this.PendingCommits.length}-${this.SubmittedCommits.length}`;
        return html`
            ${keyed(graphKey, html`
            <commit-graph
            .PendingCommits=${this.PendingCommits}
            .SubmittedCommits=${this.SubmittedCommits}
            ></commit-graph>`)}
            <div class="view-more-btn-container">
                ${this.renderGraphViewMoreBtn()}
            </div>
        `;
    }

    private renderGraphViewMoreBtn() {
        if (!this.hasMorePendingToFetch() && !this.hasMoreSubmittedToFetch()) {
            return html``
        }
        if (this.isFetchingMorePendingCommits || this.isLoadingMoreSubmittedCommits) {
            return html`<simple-loader></simple-loader>`
        }
        return html`
            <button
                id="view-more-btn"
                @click=${this.onGraphViewMoreClick}>
                Load more commits
            </button>
        `
    }

    private async onGraphViewMoreClick() {
        if (this.hasMorePendingToFetch()) {
            await this.fetchMorePendingCommits()
        }
        if (this.hasMoreSubmittedToFetch()) {
            await this.fetchMoreSubmittedCommits()
        }
    }

    private hasMorePendingToFetch(): boolean {
        return this.HaveMorePendingCommitsToFetch
    }

    private hasMoreSubmittedToFetch(): boolean {
        return this.SubmittedCommits[this.SubmittedCommits.length - 1].L != 0
    }


    private renderPendingCommitsList() {
        if (this.PendingCommits.length === 0) {
            return this.renderNonePending()
        }

        if (this.fetchWillConflictFailed) {
            return html`
                <div class="retry-fetch-container">
                    <span
                    class="retry-fetch-text"
                    @click=${this.fetchCanSubmitForPending}>
                    Failed to load data. Click to reload.
                    </span>
                </div>
            `
        }

        return html`${this.PendingCommits.map((commit, i) =>
            this.renderCommit(commit, this.pendingCommitSubmitWillConflict[i]))}`
    }
    private renderNonePending(){
        return html`
            <span class="no-pending" >No pending commits &#128578;</span>
        `
    }

    private renderFetchMorePendingCommitsBtn() {
        if (!this.HaveMorePendingCommitsToFetch) {
            return html``
        }
        if (this.isFetchingMorePendingCommits) {
            return html`<simple-loader></simple-loader>`
        }
        if (this.fetchMorePendingCommitsFailed){
            return html`
                <div class="retry-fetch-container">
                    <span
                    class="retry-fetch-text"
                    @click=${this.fetchMorePendingCommits}>
                    Failed to load data. Click to reload.
                    </span>
                </div>
            `
        }
        return html`
            <button
                id="view-more-btn"
                @click=${this.fetchMorePendingCommits}>
                View more
            </button>
        `
    }

    private renderLoadMoreSubmittedCommitsBtn() {
        if (this.isLoadingMoreSubmittedCommits){
            return html`<simple-loader></simple-loader>`
        }
        if (this.fetchMoreSubmittedCommitsFailed){
            return html`
                <div class="retry-fetch-container">
                    <span
                    class="retry-fetch-text"
                    @click=${this.fetchMoreSubmittedCommits}>
                    Failed to load data. Click to reload.
                    </span>
                </div>`
        }
        if (!enableShowMoreBtn){
            return html``
        }
        if (this.SubmittedCommits[this.SubmittedCommits.length-1].L == 0){
            return html``
        }
        return html`
        <button
            id="view-more-btn"
            @click=${this.fetchMoreSubmittedCommits}>
            View more
        </button>`
    }

    private renderCommit(commit: Commit, submitWillConflict: boolean) {
        let statusClass =
            commit.IsSubmitted ?
                'commit-submitted ' : 'commit-pending';
        let message = ""
        let urlToCommit = UrlToCommit(this.RepoOwnerName,this.RepoName,commit.L, "feed")
        if (commit.L != 0){
            message = commit.Message
        }else{
            statusClass = 'first-commit'
            message = FirstCommitMsg
            urlToCommit = ""
        }
        const isWip = commit.L != 0 && !commit.IsSubmitted && IsWipCommit(message)
        const lastUpdated = FormatRelativeTime(commit.CreatedOn);
        var commitLift= html`
            <div class="commit twigg-lift ${statusClass}">
                <span class="commit-size-tag-span">
                    <commit-size-tag .Commit=${commit}></commit-size-tag>
                </span>
                <span class="commit-author" ?hidden=${commit.L == 0}>
                    <username-tag username=${commit.AuthorUsername}></username-tag>
                </span>
                <commit-number .Number=${commit.L}></commit-number>
                <span class="commit-message">${message}</span>
                <span class="commit-last-updated" ?hidden=${commit.L == 0}>
                    ${!commit.IsSubmitted ? "Last updated: ": "Submitted: "}
                    ${lastUpdated}
                </span>
                <div>
                    ${this.renderCommitStatus(commit, submitWillConflict, isWip)}
                </div>
            </div>
        `
        return html`${urlToCommit
            ? html`<a href=${urlToCommit}>${commitLift}</a>`
            : html`<a>${commitLift}</a>`}
        `;
    }

    private renderCommitStatus(commit, submitWillConflict ,isWip) {
        if (commit.IsSubmitted) {
            return null
        }
        if (this.isLoadingWillConflict) {
            return html`<simple-loader></simple-loader>`
        }
        if (commit.HasRebaseConflicts) {
            return html`<commit-status Status="has-conflict" TooltipSide="left"></commit-status>`
        }

        if (isWip && commit.ReviewStatus === "ready") {
            return html`<commit-status Status="WIP"></commit-status>`
        }

        if (isWip) {
            return html`
            <commit-status Status="WIP"></commit-status>
            <commit-status .Status=${commit.ReviewStatus}></commit-status>
        `
        }
        if (submitWillConflict) {
            return html`
            <commit-status Status="will-conflict"  TooltipSide="left"></commit-status>
            <commit-status .Status=${commit.ReviewStatus}></commit-status>
        `
        }

        return html`<commit-status .Status=${commit.ReviewStatus}></commit-status>`
    }

    private _onDocClick?: (e: MouseEvent) => void;

    connectedCallback() {
        super.connectedCallback();
        this._onDocClick = (e: MouseEvent) => {
            const dropdown = this.renderRoot.querySelector('details.dropdown') as HTMLDetailsElement | null;
            if (!dropdown) return;

            const clickedInside = e.composedPath().includes(dropdown);

            if (dropdown.open && !clickedInside) {
                dropdown.open = false;
            }
        };
        document.addEventListener('click', this._onDocClick, { capture: true });
        this.fetchCanSubmitForPending()
    }

    disconnectedCallback() {
        document.removeEventListener('click', this._onDocClick as EventListener, { capture: true } as any);
        super.disconnectedCallback();
    }

    private async fetchMorePendingCommits() {
        if (!this.HaveMorePendingCommitsToFetch) {
            alert("Does not have more pending commits to fetch")
            return
        }
        if (this.isFetchingMorePendingCommits) {
            return
        }
        

        const lastCommit = this.PendingCommits[this.PendingCommits.length - 1]

        try {
            this.isFetchingMorePendingCommits = true
            this.fetchMorePendingCommitsFailed = false

            const res = await fetch(
                PathToMorePendingCommits(this.RepoOwnerName, this.RepoName, lastCommit.L)
            )
            if (!res.ok) {
                throw new Error(`request failed with status ${res.status}`)
            }

            const data = await res.json() as {
                PendingFrontendCommits: Commit[]
                HaveMorePendingCommitsToFetch: boolean
            }

            this.PendingCommits = [...this.PendingCommits, ...data.PendingFrontendCommits]

            this.HaveMorePendingCommitsToFetch = data.HaveMorePendingCommitsToFetch
        } catch (e) {
            console.error("failed to fetch more pending commits:", e)
            this.fetchMorePendingCommitsFailed = true
        } finally {
            this.isFetchingMorePendingCommits = false
        }
    }

    private async fetchMoreSubmittedCommits(){
        if (this.isLoadingMoreSubmittedCommits) {
            return
        }
        const lastCommit = this.SubmittedCommits[this.SubmittedCommits.length-1]
        try {
            this.isLoadingMoreSubmittedCommits = true
            this.fetchMoreSubmittedCommitsFailed = false
            const tm = new MinDurationTimer()

            const res = await fetch(
                PathToMoreSubmittedCommits(
                    this.RepoOwnerName, this.RepoName, lastCommit.ParentL));
            if (!res.ok) {
                throw new Error(`request failed with status ${res.status}`)
            }
            const commits = await res.json() as Commit[];

            await tm.Wait()
            this.SubmittedCommits = [...this.SubmittedCommits, ...commits]
        } catch (e) {
            console.error("failed to fetch more submitted commits: ", e);
            this.fetchMoreSubmittedCommitsFailed = true
        } finally {
            this.isLoadingMoreSubmittedCommits = false
        }
    }

    private async fetchCanSubmitForPending() {
        if (!this.fetchCanSubmitForPendingIsEnabled){
            const newPendingCommitSubmitWillConflict: boolean[] = []
            for (const commit of this.PendingCommits) {
                newPendingCommitSubmitWillConflict.push(false)
            }
            this.pendingCommitSubmitWillConflict = newPendingCommitSubmitWillConflict
            return
        }
        if (this.isLoadingWillConflict) { 
            return 
        }
        if (this.PendingCommits.length === 0) {
            this.pendingCommitSubmitWillConflict = []
            this.fetchWillConflictFailed = false
            return
        }

        this.isLoadingWillConflict = true
        this.fetchWillConflictFailed = false
        let newPendingCommitSubmitWillConflict: boolean[] = []
        const tm = new MinDurationTimer()

        try {
            const commitIdsToFetch = this.PendingCommits
                .filter(c => !c.HasRebaseConflicts)
                .map(c => c.L)
            if (commitIdsToFetch.length === 0) {
                // Every pending commit already has HasRebaseConflicts, so none
                // need the can-submit check. Their conflict state is already
                // shown via HasRebaseConflicts, so submitWillConflict is false.
                this.pendingCommitSubmitWillConflict = this.PendingCommits.map(() => false)
                return
            }
            
            const data: CanSubmitByCommitId = {}
            for (let i = 0; i < commitIdsToFetch.length; i += maxCanSubmitCommitsPerRequest) {
                const batch = commitIdsToFetch.slice(i, i + maxCanSubmitCommitsPerRequest)
                const resp = await fetch(
                    UrlToCanSubmitCommits(this.RepoOwnerName, this.RepoName, batch),
                    { method: 'GET' },
                )
                if (!resp.ok) {
                    throw "Request failed"
                }
                const batchData = await resp.json() as CanSubmitByCommitId
                for (const commitId in batchData) {
                    data[commitId] = batchData[commitId]
                }
            }
            newPendingCommitSubmitWillConflict = this.PendingCommits.map(commit => {
                if (commit.HasRebaseConflicts) { 
                    return false 
                }
                const item = data[String(commit.L)]
                return !item.CanSubmit && item.CantSubmitReason === "would-cause-rebase-conflict"
            })
            await tm.Wait()
        } catch (error) {
            this.fetchWillConflictFailed = true
            alert("Failed to load data :(")
            console.log(`Error fetching can submit for pending: ${error}`)
            return
        } finally {
            this.isLoadingWillConflict = false
        }

        this.pendingCommitSubmitWillConflict = newPendingCommitSubmitWillConflict
    }

    updated(changedProperties: Map<string, unknown>) {
        if (changedProperties.has('PendingCommits')) {
            this.fetchCanSubmitForPending()
        }
    }

    static styles = [
        TwiggCss,
        css`
        .main{
            max-width: var(--size4);
            margin: auto;
        }
		.header {
			display: flex;
			align-items: center;
            gap: var(--space2);
            margin: var(--space1) 0px var(--space3) 0px;
		}
		.header img {
			height: var(--space6);
			width: auto;
		}
        .title  { 
            display: flex; 
            flex-direction: column; 
        }
        .header .repo-settings-icon{
            margin-left: auto;
        }
        .header .repo-settings-icon .tab-icon{
            font-size: var(--space5p)
        }
        .commit-section {
            margin-bottom: var(--space6);
        }
        .commit-section h3 {
            margin-bottom: var(--space2);
        }
        .commit-list {
            display: flex;
            flex-direction: column;
            gap: var(--space1);
        }
        .view-more-btn-container{
            display: flex;
            justify-content: center;
        }

        .commit {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: var(--space2) var(--space4);
            margin: var(--space1) 0;
            border-radius: var(--radius1);
            background: var(--color-surface);
            box-shadow: var(--shadow-surface);
        }
        .commit-pending {
            border: 1px solid var(--color-primary);
        }
        .commit-submitted {
            border: 1px solid var(--color-border);
        }
        .first-commit {
            opacity: var(--disable-opacity-value);
            border: 1px dashed var(--color-border);
            cursor: not-allowed;
        }

        a {
            color: inherit;
            text-decoration: none;
        }
        a:hover {
            color: inherit;
            text-decoration: none;
        }
        .no-pending{
            text-align: center;
            color: var(--muted)
        }
        .commit-size-tag-span {
            font-size: var(--space4);
            padding-right: var(--space2);
        }
        .commit-author {
            margin-right: var(--space3);
        }
        .commit-message {
            flex: 1;
            margin: 0 var(--space3);
        }
        .repo-desc {
            margin: 0;
            color: var(--muted);        
        }
        .header-actions {
            margin-left: auto;
            display: inline-flex;
            align-items: center;
            gap: var(--space2);
        }

        .dropdown { position: relative; }

        .dropdown > summary {
            cursor: pointer;
            background: var(--color-primary);
            color: var(--color-text-on-primary);
            border: 1px solid var(--color-primary);
            border-radius: var(--radius2);
            height: var(--space6);
            padding: 0 var(--space4);
            display: inline-flex;
            align-items: center;
        }

        .dropdown > summary::after { 
            content: "▾"; 
            margin-left: var(--space2); 
            opacity: 0.9; 
        }

        .dropdown[open] .menu {
            position: absolute;
            top: calc(100% + var(--space1));
            right: 0;
            min-width: 220px;
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            box-shadow: var(--shadow-surface);
            padding: var(--space2);
        }

        .menu-item {
            display: flex;
            align-items: center;
            padding: var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            white-space: nowrap;
            cursor: text;
            background: var(--color-surface-alt);
        }
        .tab-icon{
            cursor: pointer;
            margin-left: var(--space2);
        }
        .icon-repo{
            font-size: var(--space5p);
        }
        .copy-btn {
            background: var(--color-surface);
            color: var(--color-primary);
            border: none;
            transition: background 0.2s, transform 0.1s;
            margin-left: var(--space2);
        }
        .copy-btn:hover {
            background: var(--color-primary-pop);
            color: var(--color-text-on-primary);
            transform: translateY(-1px);
        }
        .copy-btn:active {
            transform: translateY(1px);
        }
        #view-more-btn{
            background: var(--color-surface);
            color: var(--color-text);
        }
        .commit-last-updated {
            margin-left: var(--space3);
            margin-right: var(--space3);
            font-size: var(--space3);
            color: var(--muted);
        }
        
        @media (max-width: 600px) {
            .commit {
                flex-wrap: wrap;
                align-items: flex-start;
                gap: var(--space1);
            }

            .commit-message {
                flex-basis: 100%;
                margin: 0;
                margin-top: var(--space2);
                font-size: 1.2rem;
            }

            .commit-last-updated {
                margin: 0;
                margin-top: var(--space1);
                margin-right: auto;
                font-size: 1rem;
            }

            .commit > div:last-child {
                margin-top: var(--space1);
                margin-left: 0;
            }
        }
        .tabs-and-content-container{
            margin: 0 auto;
            display: flex;
            flex-direction: column;
            align-items: center;
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

        .retry-fetch-container {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: var(--space2);
            padding: var(--space4);
        }

        .retry-fetch-text {
            color: var(--muted);
            text-align: center;
        }
    `];
}

customElements.define("repo-display", RepoDisplay);
declare global {
    interface HTMLElementTagNameMap {
        'repo-display': RepoDisplay;
    }
}