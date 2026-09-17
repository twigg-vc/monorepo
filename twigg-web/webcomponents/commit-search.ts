import { html, LitElement, css } from 'lit';
import { TwiggCss } from './css';
import { Commit } from './interfaces';
import { FirstCommitMsg } from './commit-display';
import { PathToCommitSearch, UrlToCommit } from './routes';
import { FormatRelativeTime } from './helpers';
import { fetchGetWithRetry } from './fetch-get-with-retry';

// How long the search waits for the typing to stop before it runs.
const searchDebounceMs = 300;

interface CommitSearchResponse {
    Commits: Commit[]
    NextCursor: string
}

/**
 * Searches the commits of a repository with a query of terms such as
 * `is:pending author:me queue`.
 */
export class CommitSearch extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        query: { state: true },
        commits: { state: true },
        isSearching: { state: true },
        searchError: { state: true },
    }
    declare RepoOwnerName: string
    declare RepoName: string
    declare private query: string
    declare private commits: Commit[]
    declare private isSearching: boolean
    declare private searchError: string
    private debounceTimer: ReturnType<typeof setTimeout> | null = null

    constructor() {
        super()
        this.RepoOwnerName = ""
        this.RepoName = ""
        this.query = ""
        this.commits = []
        this.isSearching = false
        this.searchError = ""
    }

    connectedCallback() {
        super.connectedCallback()
        this.search()
    }

    private onQueryInput(e: Event) {
        this.query = (e.target as HTMLInputElement).value
        if (this.debounceTimer !== null) {
            clearTimeout(this.debounceTimer)
        }
        this.debounceTimer = setTimeout(() => this.search(), searchDebounceMs)
    }

    private async search() {
        try {
            this.isSearching = true
            const path = PathToCommitSearch(this.RepoOwnerName, this.RepoName,
                this.query, "")
            const resp = await fetchGetWithRetry(path)
            if (!resp.ok) {
                // The body of a refused search says what is wrong with it.
                this.searchError = await resp.text()
                this.commits = []
                return
            }
            const found = await resp.json() as CommitSearchResponse
            this.searchError = ""
            this.commits = found.Commits
        } catch (e) {
            console.error("failed to search the commits:", e)
            this.searchError = "The search failed"
            this.commits = []
        } finally {
            this.isSearching = false
        }
    }

    render() {
        return html`
            <input
                class="search-bar"
                type="search"
                placeholder="is:pending author:me queue"
                .value=${this.query}
                @input=${this.onQueryInput}>
            ${this.renderSearchError()}
            ${this.renderCommits()}
        `
    }

    private renderSearchError() {
        if (this.searchError === "") {
            return null
        }
        return html`<p class="search-error">${this.searchError}</p>`
    }

    private renderCommits() {
        if (this.isSearching && this.commits.length === 0) {
            return html`<simple-loader></simple-loader>`
        }
        if (this.commits.length === 0) {
            return html`<p class="no-commits">No commit matches this search</p>`
        }
        return html`
            <div class="commits">
                ${this.commits.map((c) => this.renderCommit(c))}
            </div>
        `
    }

    private renderCommit(commit: Commit) {
        var message = commit.Message
        if (commit.L === 0) {
            message = FirstCommitMsg
        }
        return html`
            <a href=${UrlToCommit(this.RepoOwnerName, this.RepoName, commit.L, "feed")}>
                <div class="commit twigg-lift">
                    <span class="commit-author" ?hidden=${commit.L === 0}>
                        <username-tag username=${commit.AuthorUsername}></username-tag>
                    </span>
                    <commit-number .Number=${commit.L}></commit-number>
                    <span class="commit-message">${message}</span>
                    <span class="commit-last-updated" ?hidden=${commit.L === 0}>
                        ${FormatRelativeTime(commit.CreatedOn)}
                    </span>
                </div>
            </a>
        `
    }

    static styles = [TwiggCss, css`
        .search-bar {
            width: 100%;
            padding: var(--space2) var(--space3);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background-color: var(--color-surface);
            color: var(--color-text);
            font-family: var(--font-family);
            font-size: var(--space4);
        }
        .search-error {
            color: var(--color-danger);
            font-size: var(--space3);
        }
        .no-commits {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .commits {
            display: flex;
            flex-direction: column;
            gap: var(--space2);
        }
        .commit {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: var(--space2);
        }
        .commit-message {
            flex-grow: 1;
        }
        .commit-last-updated {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
    `]
}
customElements.define('commit-search', CommitSearch)

declare global {
    interface HTMLElementTagNameMap {
        'commit-search': CommitSearch;
    }
}
