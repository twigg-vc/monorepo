import { html, css, LitElement, TemplateResult } from 'lit';
import { TwiggCss } from './css';
import { Bug, BugStatus, GetBugsResponse } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { GetCsrfHeaders, PathToBugs, PathToNewBug, UrlToBug } from './routes';
import { MinDurationTimer } from './min-duration-timer';
import { fetchGetWithRetry } from './fetch-get-with-retry';
import { FormatRelativeTime } from './helpers';
import { IconName } from './icons';
import './bug-status-badge';

type Filter = BugStatus | ""

/**
* "Bugs" tab of the repo display
*/
export class RepoBugsTab extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        filter: { type: String, state: true },
        page: { type: Object, state: true },
        isLoadingPage: { type: Boolean, state: true },
        loadFailed: { type: Boolean, state: true },
        isWritingNewBug: { type: Boolean, state: true },
        newBugTitle: { type: String, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare private filter: Filter;
    declare private page: GetBugsResponse | undefined;
    declare private isLoadingPage: boolean;
    declare private loadFailed: boolean;
    declare private isWritingNewBug: boolean;
    declare private newBugTitle: string;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.filter = "open";
        this.page = undefined;
        this.isLoadingPage = false;
        this.loadFailed = false;
        this.isWritingNewBug = false;
        this.newBugTitle = "";
    }

    firstUpdated() {
        this.fetchPage();
    }

    render() {
        if (!GetFeatureFlags().ShowBugs) {
            return html``
        }
        if (this.isWritingNewBug) {
            return html`
                <div class="bugs-tab new-bug">
                    <input placeholder="Title" .value=${this.newBugTitle}
                        @input=${(e: Event) => { this.newBugTitle = (e.target as HTMLInputElement).value }}/>
                    <button @click=${() => { this.isWritingNewBug = false }}>Cancel</button>
                    <button class="primary" ?disabled=${this.newBugTitle.trim() === ""} @click=${this.createBug}>Create</button>
                </div>
            `
        }
        return html`<div class="bugs-tab">${this.renderList()}</div>`
    }

    private renderList() {
        return html`
            <div class="toolbar">
                <div class="filters">
                    ${this.renderFilterBtn("open", "Bug", "Open", this.page?.OpenCount)}
                    ${this.renderFilterBtn("closed", "Check", "Closed", this.page?.ClosedCount)}
                    ${this.renderFilterBtn("", "Bars", "All", undefined)}
                </div>
                ${this.renderNewBugBtn()}
            </div>
            ${this.renderListBody()}
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
            <button class=${cls} @click=${() => this.setFilter(f)}>
                <twigg-icon .icon=${icon}></twigg-icon>
                <span>${label}</span>
                ${countTemplate}
            </button>
        `
    }

    private renderNewBugBtn() {
        if (!this.page?.CanCreate) {
            return html``
        }
        return html`
            <button class="primary-btn" @click=${() => { this.isWritingNewBug = true }}>
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
            return html`<div class="empty-msg">No bugs yet</div>`
        }
        return html`
            <div class="bug-list card">
                ${this.page.Bugs.map(b => this.renderBugRow(b))}
            </div>
        `
    }

    private setFilter(f: Filter) {
        if (this.filter === f) {
            return
        }
        this.filter = f
        this.fetchPage()
    }

    private async createBug() {
        try {
            const resp = await fetch(PathToNewBug(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                body: JSON.stringify({ Title: this.newBugTitle }),
                headers: { ...GetCsrfHeaders(), "Content-Type": "application/json" },
            })
            if (!resp.ok) {
                alert(await resp.text())
                return
            }
            this.newBugTitle = ""
            this.isWritingNewBug = false
            this.fetchPage()
        } catch (error) {
            console.log("failed to create bug: ", error)
            alert("Failed to create the bug :(")
        }
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
                    <span class="bug-title">${b.Title}</span>
                    <span class="bug-meta">
                        b/${b.Number} opened ${FormatRelativeTime(b.CreatedOn)} by
                        <username-tag .Username=${b.AuthorUsername}></username-tag>
                    </span>
                </div>
                ${comments}
            </a>
        `
    }

    private async fetchPage() {
        this.isLoadingPage = true
        this.loadFailed = false
        const tm = new MinDurationTimer()
        try {
            const resp = await fetchGetWithRetry(
                PathToBugs(this.RepoOwnerName, this.RepoName, this.filter))
            await tm.Wait()
            if (!resp.ok) {
                throw `request failed with status ${resp.status}`
            }
            this.page = await resp.json() as GetBugsResponse
        } catch (e) {
            console.log("failed to load bugs: ", e)
            this.loadFailed = true
        } finally {
            this.isLoadingPage = false
        }
    }

    static styles = [
        TwiggCss,
        css`
        .card {
            background: var(--color-surface);
            border: 1px solid var(--color-border);
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
        button {
            border: 1px solid var(--color-border);
            border-radius: 999px;
            padding: var(--space1) var(--space3);
            background: var(--color-surface-alt);
            color: var(--color-text);
            cursor: pointer;
        }
        .primary {
            background: var(--color-primary);
            color: var(--color-text-on-primary);
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
        .toolbar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            flex-wrap: wrap;
            gap: var(--space2);
            margin-bottom: var(--space3);
        }
        .filters {
            display: inline-flex;
            border: 1px solid var(--color-border);
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
            border-left: 1px solid var(--color-border);
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
        .new-bug {
            display: flex;
            gap: var(--space2);
        }
        .new-bug input {
            flex: 1;
            padding: var(--space1) var(--space2);
            background: var(--color-surface);
            color: var(--color-text);
            border: 1px solid var(--color-border);
            border-radius: var(--radius0);
        }
        .row-status {
            flex-shrink: 0;
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
            border-top: 1px solid var(--color-border);
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
        .bug-comments {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            color: var(--color-text-muted);
            font-size: var(--space3p);
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