import { html, css, LitElement, TemplateResult } from 'lit';
import { TwiggCss } from './css';
import { Bug, BugPriorities, BugPriority, BugStatus, GetBugsResponse, MaxBugTitleLen } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { GetCsrfHeaders, PathToBugs, PathToBugSearch, PathToNewBug, UrlToBug } from './routes';
import { MinDurationTimer } from './min-duration-timer';
import { fetchGetWithRetry } from './fetch-get-with-retry';
import { FormatRelativeTime } from './helpers';
import { IconName } from './icons';
import { UserSelected } from './username-input';
import './bug-status-badge';
import './bug-tag';
import './username-input';

type View = "list" | "new"
type Filter = BugStatus | ""

/**
* "Bugs" tab of the repo display
*/
export class RepoBugsTab extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },

        view: { type: String, state: true },
        filter: { type: String, state: true },
        assigneeFilter: { type: String, state: true },
        priorityFilter: { type: String, state: true },
        page: { type: Object, state: true },
        isLoadingPage: { type: Boolean, state: true },
        isLoadingMore: { type: Boolean, state: true },
        loadFailed: { type: Boolean, state: true },
        draftTitle: { type: String, state: true },
        draftBody: { type: String, state: true },
        draftPriority: { type: String, state: true },
        isCreating: { type: Boolean, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare private view: View;
    declare private filter: Filter;
    // "" (no assignee filter), "unassigned", "me", or a specific username.
    declare private assigneeFilter: string;
    // "" (no priority filter), "none", or a priority.
    declare private priorityFilter: string;
    declare private page: GetBugsResponse | undefined;
    declare private isLoadingPage: boolean;
    declare private isLoadingMore: boolean;
    declare private loadFailed: boolean;
    declare private draftTitle: string;
    declare private draftBody: string;
    declare private draftPriority: BugPriority;
    declare private isCreating: boolean;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.view = "list";
        this.filter = "open";
        this.assigneeFilter = "";
        this.priorityFilter = "";
        this.page = undefined;
        this.isLoadingPage = false;
        this.isLoadingMore = false;
        this.loadFailed = false;
        this.draftTitle = "";
        this.draftBody = "";
        this.draftPriority = "";
        this.isCreating = false;
    }

    firstUpdated() {
        this.fetchPage();
    }

    render() {
        if (!GetFeatureFlags().ShowBugs) {
            return html``
        }
        switch (this.view) {
            case "list":
                return html`<div class="bugs-tab">${this.renderList()}</div>`
            case "new":
                return html`<div class="bugs-tab">${this.renderNewBug()}</div>`
        }
    }

    private renderList() {
        var openCount: number | undefined
        if (this.page !== undefined && this.page.HasOpenCount) {
            openCount = this.page.OpenCount
        } else {
            openCount = undefined
        }
        var closedCount: number | undefined
        if (this.page !== undefined && this.page.HasClosedCount) {
            closedCount = this.page.ClosedCount
        } else {
            closedCount = undefined
        }
        return html`
            <div class="toolbar">
                <div class="filter-group" role="group" aria-labelledby="status-label">
                    <span class="filter-label" id="status-label">Status</span>
                    <div class="filters">
                        ${this.renderFilterBtn("open", "Bug", "Open", openCount)}
                        ${this.renderFilterBtn("closed", "Check", "Closed", closedCount)}
                        ${this.renderFilterBtn("", "Bars", "All", undefined)}
                    </div>
                </div>
                ${this.renderNewBugBtn()}
            </div>
            ${this.renderSearchFilters()}
            ${this.renderListBody()}
        `
    }

    private renderSearchFilters() {
        if (!GetFeatureFlags().SearchBugsUi) {
            return html``
        }
        return html`
            <div class="search-filters">
                <div class="filter-group" role="group" aria-labelledby="assignee-label">
                    <span class="filter-label" id="assignee-label">Assignee</span>
                    <div class="filters">
                        ${this.renderAssigneeFilterBtn("", "Anyone")}
                        ${this.renderAssigneeFilterBtn("unassigned", "Unassigned")}
                        ${this.renderAssigneeFilterBtn("me", "Me")}
                        ${this.renderAssigneeUsernameFilter()}
                    </div>
                </div>
                <div class="filter-group" role="group" aria-labelledby="priority-label">
                    <span class="filter-label" id="priority-label">Priority</span>
                    <div class="filters">
                        ${this.renderPriorityFilterBtn("", "Any")}
                        ${this.renderPriorityFilterBtn("P0", "P0")}
                        ${this.renderPriorityFilterBtn("P1", "P1")}
                        ${this.renderPriorityFilterBtn("P2", "P2")}
                        ${this.renderPriorityFilterBtn("none", "None")}
                    </div>
                </div>
            </div>
        `
    }

    private renderFilterBtn(f: Filter, icon: IconName, label: string, count: number | undefined) {
        var cls: string | undefined = undefined
        if (this.filter === f) {
            cls = "filter active"
        } else {
            cls = "filter"
        }
        var countTemplate: TemplateResult | undefined = undefined
        if (count !== undefined) {
            countTemplate = html`<span class="count">${count}</span>`
        } else {
            countTemplate = html``
        }
        return html`
            <button class=${cls} aria-pressed=${String(this.filter === f)} @click=${() => this.setFilter(f)}>
                <twigg-icon .icon=${icon}></twigg-icon>
                <span>${label}</span>
                ${countTemplate}
            </button>
        `
    }

    private renderAssigneeFilterBtn(value: string, label: string) {
        return html`
            <button
                class=${this.searchFilterCls(this.assigneeFilter === value)}
                aria-pressed=${String(this.assigneeFilter === value)}
                @click=${() => this.setAssigneeFilter(value)}>
                ${label}
            </button>
        `
    }

    private setAssigneeFilter(value: string) {
        if (this.assigneeFilter === value) {
            return
        }
        this.assigneeFilter = value
        this.fetchPage()
    }

    private renderPriorityFilterBtn(value: string, label: string) {
        return html`
            <button
                class=${this.searchFilterCls(this.priorityFilter === value)}
                aria-pressed=${String(this.priorityFilter === value)}
                @click=${() => this.setPriorityFilter(value)}>
                ${label}
            </button>
        `
    }

    private setPriorityFilter(value: string) {
        if (this.priorityFilter === value) {
            return
        }
        this.priorityFilter = value
        this.fetchPage()
    }

    private searchFilterCls(active: boolean): string {
        if (active) {
            return "filter active"
        } else {
            return "filter"
        }
    }

    private renderAssigneeUsernameFilter() {
        if (this.assigneeFilter === "" || this.assigneeFilter === "unassigned" || this.assigneeFilter === "me") {
            return html`
                <username-input
                    class="username-filter"
                    Appearance="embedded"
                    HideButtonWhenEmpty
                    Label="Assignee username"
                    Placeholder="Username"
                    ButtonText="Filter"
                    @user-selected=${(e: CustomEvent<UserSelected>) => this.setAssigneeFilter(e.detail.Username)}>
                </username-input>
            `
        }
        return html`
            <button class="filter active" aria-pressed="true" title="Clear" @click=${() => this.setAssigneeFilter("")}>
                @${this.assigneeFilter}
                <twigg-icon icon="XMark"></twigg-icon>
            </button>
        `
    }

    private renderNewBugBtn() {
        if (!this.page?.CanCreate) {
            return html``
        }
        return html`
            <button class="primary-btn" @click=${() => { this.view = "new" }}>
                <twigg-icon icon="Bug"></twigg-icon>
                <span>New bug</span>
            </button>
        `
    }

    private renderListBody() {
        if (this.loadFailed) {
            return html`<div class="empty-msg retry" @click=${this.fetchPage}>Failed to load bugs - click to retry</div>`
        }
        if (this.isLoadingPage || this.page === undefined) {
            return html`<simple-loader class="loader"></simple-loader>`
        }
        if (this.page.Bugs.length === 0) {
            return html`<div class="empty-state card">${this.renderEmptyMsg()}</div>`
        }
        return html`
            <div class="bug-list card">
                ${this.page.Bugs.map(b => this.renderBugRow(b))}
            </div>
            ${this.renderLoadMoreBtn()}
        `
    }

    private renderEmptyMsg() {
        if (this.isSearching()) {
            return html`
                <twigg-icon class="empty-icon" icon="Bug"></twigg-icon>
                <h3>No bugs match this filter</h3>
            `
        }
        const p = this.page!
        const total = p.OpenCount + p.ClosedCount
        if (total === 0) {
            return html`
                <twigg-icon class="empty-icon" icon="Bug"></twigg-icon>
                <h3>No bugs yet</h3>
                <p>A blank slate. Bug reports, feature requests and wild ideas all welcome.</p>
            `
        }
        if (this.filter === "open") {
            return html`
                <twigg-icon class="empty-icon success" icon="Check"></twigg-icon>
                <h3>No open bugs</h3>
                <p>All ${p.ClosedCount} wrapped up. Enjoy the quiet while it lasts.</p>
            `
        }
        return html`
            <twigg-icon class="empty-icon" icon="Bug"></twigg-icon>
            <h3>Nothing closed yet</h3>
            <p>${p.OpenCount} waiting for some love.</p>
        `
    }

    private renderLoadMoreBtn() {
        if (this.page?.NextCursor === "") {
            return html``
        }
        if (this.isLoadingMore) {
            return html`<simple-loader></simple-loader>`
        }
        return html`<button class="load-more-btn" @click=${this.fetchMore}>Load more</button>`
    }

    private setFilter(f: Filter) {
        if (this.filter === f) {
            return
        }
        this.filter = f
        this.fetchPage()
    }

    private renderBackLink() {
        return html`
            <button class="back-link" @click=${this.showList}>
                <twigg-icon icon="ChevronLeft"></twigg-icon>
                <span>All bugs</span>
            </button>
        `
    }

    private renderNewBug() {
        const canSubmit = this.draftTitle.trim() !== "" && !this.isCreating
        var submitBtn: TemplateResult | undefined = undefined
        if (this.isCreating) {
            submitBtn = html`<simple-loader></simple-loader>`
        } else {
            submitBtn = html`
                <button class="primary-btn" ?disabled=${!canSubmit} @click=${this.createBug}>
                    Create
                </button>
            `
        }
        return html`
            ${this.renderBackLink()}
            <div class="new-bug card">
                <input
                    class="title-input"
                    placeholder="Title"
                    maxlength=${MaxBugTitleLen}
                    .value=${this.draftTitle}
                    @input=${(e: Event) => { this.draftTitle = (e.target as HTMLInputElement).value }}
                />
                <md-input2
                    .Content=${this.draftBody}
                    .InputIsOpen=${true}
                    .CloseInputBtnIsHidden=${true}
                    .OpenInputBtnIsHidden=${true}
                    .HasSubmitBtn=${false}
                    ContentPlaceholder="Enter description (markdown supported)"
                    .RepoOwnerName=${this.RepoOwnerName}
                    .RepoName=${this.RepoName}
                    @md-input-changed=${(e: CustomEvent) => { this.draftBody = e.detail.NewContent }}>
                </md-input2>
                <div class="form-actions">
                    <select class="priority-select"
                        @change=${(e: Event) => { this.draftPriority = (e.target as HTMLSelectElement).value as BugPriority }}>
                        ${BugPriorities.map(p => html`<option value=${p} ?selected=${p === this.draftPriority}>${this.priorityText(p)}</option>`)}
                    </select>
                    <button class="secondary-btn" @click=${this.showList}>Cancel</button>
                    ${submitBtn}
                </div>
            </div>
        `
    }

    // "-" when not set
    private priorityText(p: BugPriority): string {
        if (p === "") {
            return "-"
        } else {
            return p
        }
    }

    private async createBug() {
        if (this.draftTitle.trim() === "" || this.isCreating) {
            return
        }
        this.isCreating = true
        try {
            const resp = await fetch(PathToNewBug(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                body: JSON.stringify({ Title: this.draftTitle, Body: this.draftBody, Priority: this.draftPriority }),
                headers: { ...GetCsrfHeaders(), "Content-Type": "application/json" },
            })
            if (!resp.ok) {
                alert(await resp.text())
                return
            }
            const b = await resp.json() as Bug
            this.draftTitle = ""
            this.draftBody = ""
            this.draftPriority = ""
            window.location.href = UrlToBug(this.RepoOwnerName, this.RepoName, b.Number)
        } catch (err) {
            console.log("failed to create bug: ", err)
            alert("Failed to create bug :(")
        } finally {
            this.isCreating = false
        }
    }

    private showList() {
        this.view = "list"
        this.fetchPage()
    }

    private renderBugRow(b: Bug) {
        var comments: TemplateResult | undefined = undefined
        if (b.CommentCount > 0) {
            comments = html`
                <span class="bug-comments" title="${b.CommentCount} comments">
                    <twigg-icon icon="ChatBubbleLeft"></twigg-icon>${b.CommentCount}
                </span>
            `
        } else {
            comments = html``
        }
        return html`
            <a class="bug-row" href=${UrlToBug(this.RepoOwnerName, this.RepoName, b.Number)}>
                <bug-status-badge class="row-status" .Status=${b.Status} IconOnly></bug-status-badge>
                <div class="bug-main">
                    <span class="bug-title">${this.renderPriorityTag(b)}${b.Title}</span>
                    <span class="bug-meta">
                        b/${b.Number} ${this.renderAssignee(b)} opened ${FormatRelativeTime(b.CreatedOn)} by @${b.AuthorUsername}
                    </span>
                </div>
                ${comments}
            </a>
        `
    }

    private renderPriorityTag(b: Bug) {
        if (b.Priority === "") {
            return html``
        } else {
            return html`<bug-tag .Priority=${b.Priority}></bug-tag>`
        }
    }

    private renderAssignee(b: Bug) {
        if (b.AssigneeUsername !== "") {
            return html`assigned to <username-tag .Username=${b.AssigneeUsername}></username-tag>`
        } else {
            return html`<strong class="no-assignee">unassigned</strong>`
        }
    }

    private pageRequestId = 0

    private isSearching(): boolean {
        return this.assigneeFilter !== "" || this.priorityFilter !== ""
    }

    // Plain status filter stays on the simple /bugs endpoint (counts + can
    // create). Any search filter switches to /bug-search instead, which
    // only returns the matching bugs.
    private pathToPage(cursor: string): string {
        if (!this.isSearching()) {
            return PathToBugs(this.RepoOwnerName, this.RepoName, this.filter, cursor)
        }
        var terms: string[] = []
        if (this.filter !== "") {
            terms.push(`is:${this.filter}`)
        }
        if (this.assigneeFilter !== "") {
            terms.push(`assignee:${this.assigneeFilter}`)
        }
        if (this.priorityFilter !== "") {
            terms.push(`priority:${this.priorityFilter}`)
        }
        return PathToBugSearch(this.RepoOwnerName, this.RepoName, terms.join(" "), cursor)
    }

    private async fetchPage() {
        const requestId = ++this.pageRequestId
        this.isLoadingPage = true
        this.loadFailed = false
        const tm = new MinDurationTimer()
        try {
            const resp = await fetchGetWithRetry(this.pathToPage(""))
            await tm.Wait()
            if (!resp.ok) {
                throw `request failed with status ${resp.status}`
            }
            const page = await resp.json() as GetBugsResponse
            if (requestId === this.pageRequestId) {
                this.page = page
            }
        } catch (e) {
            console.log("failed to load bugs: ", e)
            if (requestId === this.pageRequestId) {
                this.loadFailed = true
            }
        } finally {
            if (requestId === this.pageRequestId) {
                this.isLoadingPage = false
            }
        }
    }

    private async fetchMore() {
        if (this.isLoadingMore || this.page === undefined) {
            return
        }
        this.isLoadingMore = true
        const requestId = this.pageRequestId
        try {
            const resp = await fetchGetWithRetry(this.pathToPage(this.page.NextCursor))
            if (!resp.ok) {
                throw `request failed with status ${resp.status}`
            }
            const more = await resp.json() as GetBugsResponse
            if (requestId !== this.pageRequestId) {
                return
            }
            this.page = { ...more, Bugs: [...this.page!.Bugs, ...more.Bugs] }
        } catch (e) {
            console.log("failed to load more bugs: ", e)
            alert("Failed to load more bugs :(")
        } finally {
            this.isLoadingMore = false
        }
    }

    static styles = [
        TwiggCss,
        css`
        .card {
            background: var(--color-surface);
            border: var(--border-width) solid var(--color-border);
            border-radius: var(--radius1);
            box-shadow: var(--shadow-surface);
        }
        .loader {
            padding: var(--space4);
        }
        .empty-msg {
            padding: var(--space4);
            color: var(--color-text-muted);
            font-style: italic;
            text-align: center;
        }
        .empty-msg.retry {
            cursor: pointer;
        }
        .bugs-tab {
            padding-top: var(--space4);
        }
        .primary-btn {
            background: var(--color-primary);
            color: var(--color-text-on-primary);
            border-color: var(--color-primary);
            gap: var(--space1);
            font: inherit;
            padding: var(--space1) var(--space4);
        }
        .primary-btn:hover {
            box-shadow: var(--shadow-pop);
        }
        .primary-btn[disabled] {
            opacity: var(--disable-opacity-value);
            cursor: not-allowed;
            transform: none;
            box-shadow: none;
        }
        .secondary-btn {
            background: var(--color-surface);
            color: var(--color-text);
            font: inherit;
            padding: var(--space1) var(--space4);
        }
        .toolbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            flex-wrap: wrap;
            gap: var(--space2);
            margin-bottom: var(--space3);
        }
        .search-filters {
            display: flex;
            align-items: center;
            flex-wrap: wrap;
            gap: var(--space2) var(--space5p);
            margin-bottom: var(--space3);
        }
        .filter-group {
            display: flex;
            align-items: center;
            max-width: 100%;
            flex-wrap: wrap;
            gap: var(--space2);
        }
        .filter-label {
            min-width: calc(var(--space7) + var(--space4));
            color: var(--color-text-muted);
            font-size: var(--space3p);
            font-weight: var(--weight-semi-bold);
        }
        .username-filter {
            width: calc(2 * var(--space7));
            max-width: 100%;
            flex-shrink: 0;
            min-width: 0;
            border-left: var(--border-width) solid var(--color-border);
        }
        .username-filter:focus-within {
            width: calc(4 * var(--space7));
        }
        .filters:has(.username-filter:focus-within) {
            border-color: var(--color-primary-pop);
        }
        .filters {
            display: inline-flex;
            flex-wrap: wrap;
            max-width: 100%;
            border: var(--border-width) solid var(--color-border);
            border-radius: 999px;
            overflow: hidden;
            background: var(--color-surface);
        }
        .filter {
            border: none;
            border-radius: 0;
            background: none;
            color: var(--color-text-muted);
            padding: var(--space1) var(--space3);
            gap: var(--space1);
            font: inherit;
        }
        .filter + .filter {
            border-left: var(--border-width) solid var(--color-border);
        }
        .filter:hover {
            transform: none;
            color: var(--color-text);
        }
        .filter.active {
            background: var(--color-surface-alt);
            color: var(--color-primary-pop);
            font-weight: var(--weight-semi-bold);
        }
        .count {
            background: var(--color-surface-alt);
            border-radius: 999px;
            padding: 0 var(--space2);
            font-size: var(--space3);
            color: var(--color-text);
        }
        .filter.active .count {
            background: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        .row-status {
            flex-shrink: 0;
        }
        .no-assignee {
            color: var(--color-text-muted);
            font-weight: var(--weight-bold);
            font-style: italic;
        }
        .bug-meta {
            color: var(--color-text-muted);
            font-size: var(--space3p);
            display: inline-flex;
            align-items: center;
            flex-wrap: wrap;
            gap: var(--space1);
        }
        .bug-list {
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        a.bug-row {
            display: flex;
            align-items: center;
            gap: var(--space3);
            padding: var(--space3) var(--space4);
            color: var(--color-text);
            text-decoration: none;
            transition: background .15s;
        }
        a.bug-row + a.bug-row {
            border-top: var(--border-width) solid var(--color-border);
        }
        a.bug-row:hover {
            background: var(--color-surface-alt);
            text-decoration: none;
        }
        a.bug-row:hover .bug-title {
            color: var(--color-primary-pop);
        }
        .bug-main {
            display: flex;
            flex-direction: column;
            gap: var(--space1);
            flex: 1;
            min-width: 0;
        }
        .bug-title {
            font-weight: var(--weight-semi-bold);
            overflow-wrap: anywhere;
        }
        bug-tag {
            margin-right: var(--space2);
        }
        .load-more-btn {
            margin: var(--space3) auto;
            background: var(--color-surface);
            color: var(--color-text);
        }
        .empty-state {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: var(--space2);
            padding: var(--space6) var(--space4);
            text-align: center;
            color: var(--color-text-muted);
        }
        .empty-icon {
            font-size: var(--space6);
            color: var(--color-primary-pop);
        }
        .empty-icon.success {
            color: var(--color-success);
        }
        .back-link {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            color: var(--color-text-muted);
            margin-bottom: var(--space3);
            background: none;
            border: none;
            padding: 0;
            font: inherit;
        }
        .new-bug {
            display: flex;
            flex-direction: column;
            gap: var(--space3);
            padding: var(--space4);
        }
        .title-input {
            font: inherit;
            font-size: var(--space5);
            padding: var(--space2) var(--space3);
            border: var(--border-width) solid var(--color-border);
            border-radius: var(--radius1);
            background: var(--color-surface-alt);
            color: var(--color-text);
            outline: none;
        }
        .title-input:focus {
            border-color: var(--color-primary-pop);
            box-shadow: var(--shadow-pop);
        }
        .form-actions {
            display: flex;
            justify-content: flex-end;
            gap: var(--space2);
        }
        .priority-select {
            margin-right: auto;
            font: inherit;
            padding: var(--space1) var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background: var(--color-surface-alt);
            color: var(--color-text);
        }
        .bug-comments {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            color: var(--color-text-muted);
            font-size: var(--space3p);
        }
        @media (max-width: 600px) {
            .filter-group {
                flex-direction: column;
                align-items: flex-start;
                gap: var(--space1);
            }
            .filters {
                flex-wrap: wrap;
                border-radius: var(--radius2);
            }
            .username-filter {
                flex-basis: 100%;
                border-left: none;
                border-top: var(--border-width) solid var(--color-border);
            }
        }
        `,
    ];
}
customElements.define('repo-bugs-tab', RepoBugsTab);
declare global {
    interface HTMLElementTagNameMap {
        'repo-bugs-tab': RepoBugsTab;
    }
}