import { html, css, LitElement } from 'lit';
import { Bug, GetBugsResponse } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { GetCsrfHeaders, PathToBugs, UrlToBug } from './routes';
import { MinDurationTimer } from './min-duration-timer';
import { fetchGetWithRetry } from './fetch-get-with-retry';
import { FormatRelativeTime } from './helpers';
import './bug-status-badge';

/**
* "Bugs" tab of the repo display
*/
export class RepoBugsTab extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        page: { type: Object, state: true },
        loadFailed: { type: Boolean, state: true },
        isWritingNewBug: { type: Boolean, state: true },
        newBugTitle: { type: String, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare private page: GetBugsResponse | undefined;
    declare private loadFailed: boolean;
    declare private isWritingNewBug: boolean;
    declare private newBugTitle: string;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.page = undefined;
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
        if (this.loadFailed) {
            return html`<div class="bugs-tab empty-msg retry" @click=${this.fetchPage}>Failed to load bugs - click to retry</div>`
        }
        if (this.page === undefined) {
            return html`<div class="bugs-tab"><simple-loader></simple-loader></div>`
        }
        if (this.page.Bugs.length === 0) {
            return html`
                ${this.renderNewBugBtn()}
                <div class="bugs-tab empty-msg">No bugs yet</div>
            `
        }
        return html`
            ${this.renderNewBugBtn()}
            <div class="bugs-tab bug-list">
                ${this.page.Bugs.map(b => this.renderBugRow(b))}
            </div>
        `
    }

    private renderNewBugBtn() {
        if (!this.page?.CanCreate) {
            return html``
        }
        return html`<button class="primary new-bug-btn" @click=${() => { this.isWritingNewBug = true }}>New bug</button>`
    }

    private async createBug() {
        try {
            const resp = await fetch(PathToBugs(this.RepoOwnerName, this.RepoName), {
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
        return html`
            <a class="bug-row" href=${UrlToBug(this.RepoOwnerName, this.RepoName, b.Number)}>
                <bug-status-badge .Status=${b.Status} IconOnly></bug-status-badge>
                <div class="bug-main">
                    <span class="bug-title">${b.Title}</span>
                    <span class="bug-meta">
                        b/${b.Number} opened ${FormatRelativeTime(b.CreatedOn)} by
                        <username-tag username=${b.AuthorUsername}></username-tag>
                    </span>
                </div>
            </a>
        `
    }

    private async fetchPage() {
        this.loadFailed = false
        const tm = new MinDurationTimer()
        try {
            const resp = await fetchGetWithRetry(PathToBugs(this.RepoOwnerName, this.RepoName), { method: 'GET' })
            await tm.Wait()
            if (!resp.ok) {
                throw "Bad response"
            }
            this.page = await resp.json()
        } catch (error) {
            console.log("error getting bugs: ", error)
            this.loadFailed = true
        }
    }

    static styles = css`
        .bugs-tab {
            margin-top: var(--space3);
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
        .new-bug-btn {
            margin: var(--space3) 0 0 auto;
            display: block;
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
        .empty-msg {
            padding: var(--space4);
            color: var(--color-text-muted);
            font-style: italic;
            text-align: center;
        }
        .retry {
            cursor: pointer;
        }
        .bug-list {
            display: flex;
            flex-direction: column;
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius2);
        }
        .bug-row {
            display: flex;
            align-items: center;
            gap: var(--space3);
            padding: var(--space3) var(--space4);
            color: var(--color-text);
            text-decoration: none;
        }
        .bug-main {
            display: flex;
            flex-direction: column;
            gap: var(--space1);
        }
        .bug-row:not(:last-child) {
            border-bottom: 1px solid var(--color-border);
        }
        .bug-row:hover {
            background: var(--color-surface-alt);
        }
        .bug-title {
            font-weight: var(--weight-semi-bold);
        }
        .bug-meta {
            color: var(--color-text-muted);
            font-size: var(--space3p);
        }
    `;
}
customElements.define('repo-bugs-tab', RepoBugsTab);
declare global {
    interface HTMLElementTagNameMap {
        'repo-bugs-tab': RepoBugsTab;
    }
}