import { html, css, LitElement } from 'lit';
import { Bug, GetBugsResponse } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { PathToBugs, UrlToBug } from './routes';
import { MinDurationTimer } from './min-duration-timer';
import { fetchGetWithRetry } from './fetch-get-with-retry';
import { FormatRelativeTime } from './helpers';

/**
* "Bugs" tab of the repo display
*/
export class RepoBugsTab extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        page: { type: Object, state: true },
        loadFailed: { type: Boolean, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare private page: GetBugsResponse | undefined;
    declare private loadFailed: boolean;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.page = undefined;
        this.loadFailed = false;
    }

    firstUpdated() {
        this.fetchPage();
    }

    render() {
        if (!GetFeatureFlags().ShowBugs) {
            return html``
        }
        if (this.loadFailed) {
            return html`<div class="bugs-tab empty-msg retry" @click=${this.fetchPage}>Failed to load bugs - click to retry</div>`
        }
        if (this.page === undefined) {
            return html`<div class="bugs-tab"><simple-loader></simple-loader></div>`
        }
        if (this.page.Bugs.length === 0) {
            return html`<div class="bugs-tab empty-msg">No bugs yet</div>`
        }
        return html`
            <div class="bugs-tab bug-list">
                ${this.page.Bugs.map(b => this.renderBugRow(b))}
            </div>
        `
    }

    private renderBugRow(b: Bug) {
        return html`
            <a class="bug-row" href=${UrlToBug(this.RepoOwnerName, this.RepoName, b.Number)}>
                <span class="bug-title">${b.Title}</span>
                <span class="bug-meta">
                    b/${b.Number} opened ${FormatRelativeTime(b.CreatedOn)} by
                    <username-tag username=${b.AuthorUsername}></username-tag>
                </span>
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
            flex-direction: column;
            gap: var(--space1);
            padding: var(--space3) var(--space4);
            color: var(--color-text);
            text-decoration: none;
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
