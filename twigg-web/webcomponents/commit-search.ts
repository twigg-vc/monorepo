import { html, LitElement, css } from 'lit';
import { TwiggCss } from './css';
import { Commit } from './interfaces';
import { FirstCommitMsg, IsWipCommit } from './commit-display';
import { PathToCommitSearch, UrlToCanSubmitCommits, UrlToCommit } from './routes';
import { FormatRelativeTime } from './helpers';
import { fetchGetWithRetry } from './fetch-get-with-retry';

// How long the search waits for the typing to stop before it runs.
const searchDebounceMs = 300;

// Where the search is kept in the page url, so that one can be shared.
const searchQueryParamName = "q";

// Terms that can not hold at the same time, because the search keeps the last
// of them. Adding one takes the others out.
const exclusiveTermGroups: string[][] = [
    ["is:pending", "is:submitted"],
    ["is:missing-lgtm", "is:unresolved", "is:ready"],
];

// The buttons under the search bar. Each one adds its term to the search or
// takes it out.
const termButtons: { term: string, label: string }[] = [
    { term: "is:pending", label: "Pending" },
    { term: "is:submitted", label: "Submitted" },
    { term: "author:me", label: "Mine" },
    { term: "reviewer:me", label: "To review" },
    { term: "is:unresolved", label: "Unresolved" },
    { term: "is:missing-lgtm", label: "Missing LGTM" },
    { term: "is:ready", label: "Ready" },
    { term: "-is:wip", label: "No WIP" },
    { term: "is:archived", label: "Archived" },
];

// How many commits the can-submit endpoint is asked about at a time.
const maxCanSubmitCommitsPerRequest = 20;

type CanSubmitByCommitId = Record<string, {
    CanSubmit: boolean
    CantSubmitReason: string
}>

interface CommitSearchResponse {
    Commits: Commit[]
    NextCursor: string
}

/**
 * Searches the commits of a repository with a query of terms such as
 * `is:pending author:me queue`.
 */
// The terms that can not hold at the same time as the given one.
function termsExcludedBy(term: string): string[] {
    for (const group of exclusiveTermGroups) {
        if (group.includes(term)) {
            return group
        }
    }
    return []
}

// Splits a search the way the server does, keeping each token exactly as it
// was typed so that taking one out leaves the rest as it was written.
function searchTokens(query: string): string[] {
    const tokens: string[] = []
    var current = ""
    var inQuotes = false
    var hasToken = false
    for (const c of query) {
        if (c === '"') {
            inQuotes = !inQuotes
            current += c
            hasToken = true
        } else if (c === " " && !inQuotes) {
            if (hasToken) {
                tokens.push(current)
            }
            current = ""
            hasToken = false
        } else {
            current += c
            hasToken = true
        }
    }
    if (hasToken) {
        tokens.push(current)
    }
    return tokens
}

export class CommitSearch extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        query: { state: true },
        commits: { state: true },
        isSearching: { state: true },
        searchError: { state: true },
        willConflictByCommitId: { state: true },
        nextCursor: { state: true },
        isLoadingMore: { state: true },
        showHelp: { state: true },
    }
    declare RepoOwnerName: string
    declare RepoName: string
    declare private query: string
    declare private commits: Commit[]
    declare private isSearching: boolean
    declare private searchError: string
    declare private willConflictByCommitId: Record<string, boolean>
    declare private nextCursor: string
    declare private isLoadingMore: boolean
    declare private showHelp: boolean
    private debounceTimer: ReturnType<typeof setTimeout> | null = null

    constructor() {
        super()
        this.RepoOwnerName = ""
        this.RepoName = ""
        this.query = ""
        this.commits = []
        this.isSearching = false
        this.searchError = ""
        this.willConflictByCommitId = {}
        this.nextCursor = ""
        this.isLoadingMore = false
        this.showHelp = false
    }

    connectedCallback() {
        super.connectedCallback()
        const searched = new URLSearchParams(window.location.search)
            .get(searchQueryParamName)
        if (searched !== null) {
            this.query = searched
        }
        this.search()
    }

    // The search is written to the page url so that it can be shared and
    // survives a reload. It replaces the url instead of pushing a new one,
    // so that going back leaves the repo instead of undoing the typing.
    private writeSearchToUrl() {
        const url = new URL(window.location.href)
        if (this.query === "") {
            url.searchParams.delete(searchQueryParamName)
        } else {
            url.searchParams.set(searchQueryParamName, this.query)
        }
        history.replaceState(null, "", url)
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
            this.writeSearchToUrl()
            this.searchError = ""
            this.willConflictByCommitId = {}
            this.nextCursor = ""
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
            this.nextCursor = found.NextCursor
        } catch (e) {
            console.error("failed to search the commits:", e)
            this.searchError = "The search failed"
            this.commits = []
        } finally {
            this.isSearching = false
        }
        this.readWhichSubmitsConflict()
    }

    // Reads the commits after the ones already found, which the cursor of the
    // last search points at.
    private async loadMoreCommits() {
        if (this.isLoadingMore || this.nextCursor === "") {
            return
        }
        try {
            this.isLoadingMore = true
            const path = PathToCommitSearch(this.RepoOwnerName, this.RepoName,
                this.query, this.nextCursor)
            const resp = await fetchGetWithRetry(path)
            if (!resp.ok) {
                throw new Error(`request failed with status ${resp.status}`)
            }
            const found = await resp.json() as CommitSearchResponse
            // A page with no commits is the end of the search.
            if (found.Commits.length === 0) {
                this.nextCursor = ""
                return
            }
            this.commits = [...this.commits, ...found.Commits]
            this.nextCursor = found.NextCursor
        } catch (e) {
            // The commits already found stay, and the button comes back for
            // whoever wants to try again.
            console.error("failed to read more commits:", e)
            return
        } finally {
            this.isLoadingMore = false
        }
        this.readWhichSubmitsConflict()
    }

    // Reads whether submitting each commit would conflict. It costs a request
    // of its own, so it runs once the results settled and fills the badges in
    // when it lands.
    private async readWhichSubmitsConflict() {
        const searched = this.commits
        const idsToRead = searched
            .filter((c) => c.L !== 0 && !c.IsSubmitted && !c.HasRebaseConflicts)
            .map((c) => c.L)
        if (idsToRead.length === 0) {
            return
        }
        const willConflict: Record<string, boolean> = {}
        try {
            for (let i = 0; i < idsToRead.length; i += maxCanSubmitCommitsPerRequest) {
                const batch = idsToRead.slice(i, i + maxCanSubmitCommitsPerRequest)
                const resp = await fetchGetWithRetry(
                    UrlToCanSubmitCommits(this.RepoOwnerName, this.RepoName, batch))
                if (!resp.ok) {
                    throw new Error(`request failed with status ${resp.status}`)
                }
                const read = await resp.json() as CanSubmitByCommitId
                for (const commitId in read) {
                    const item = read[commitId]
                    willConflict[commitId] = !item.CanSubmit &&
                        item.CantSubmitReason === "would-cause-rebase-conflict"
                }
            }
        } catch (e) {
            // The badge is extra, so a search that got its commits still works.
            console.error("failed to read whether a submit conflicts:", e)
            return
        }
        if (this.commits !== searched) {
            // A newer search already replaced these commits.
            return
        }
        this.willConflictByCommitId = willConflict
    }

    render() {
        return html`
            <div class="search-bar-container">
                <input
                    class="search-bar"
                    type="search"
                    placeholder="is:pending author:me queue"
                    .value=${this.query}
                    @input=${this.onQueryInput}>
                ${this.renderClearSearchBtn()}
            </div>
            ${this.renderTermChips()}
            ${this.renderTermButtons()}
            ${this.renderHelp()}
            ${this.renderResults()}
            <div class="load-more-btn-container">
                ${this.renderLoadMoreBtn()}
            </div>
        `
    }

    private searchedTerms(): string[] {
        return searchTokens(this.query)
    }

    private toggleTerm(term: string) {
        var terms = this.searchedTerms()
        if (terms.includes(term)) {
            terms = terms.filter((t) => t !== term)
        } else {
            const exclusive = termsExcludedBy(term)
            terms = terms.filter((t) => !exclusive.includes(t))
            terms.push(term)
        }
        this.query = terms.join(" ")
        if (this.debounceTimer !== null) {
            clearTimeout(this.debounceTimer)
        }
        this.search()
    }

    // What the search is made of, so that one term of it can be taken out
    // without editing the text by hand.
    private renderTermChips() {
        const tokens = this.searchedTerms()
        if (tokens.length === 0) {
            return html`
                <div class="term-chips">
                    <span class="no-terms">Searching every commit</span>
                </div>
            `
        }
        return html`
            <div class="term-chips">
                ${tokens.map((t, i) => this.renderTermChip(t, i))}
            </div>
        `
    }

    private renderTermChip(token: string, index: number) {
        var chipClass = "term-chip"
        if (!token.startsWith(`"`) && token.includes(":")) {
            chipClass = "term-chip term-chip-of-key"
        }
        return html`
            <span class=${chipClass}>
                ${token}
                <button
                    class="remove-term-btn"
                    title="Take it out of the search"
                    @click=${() => this.removeTerm(index)}>
                    <twigg-icon icon="XMark"></twigg-icon>
                </button>
            </span>
        `
    }

    private removeTerm(index: number) {
        const tokens = this.searchedTerms()
        tokens.splice(index, 1)
        this.query = tokens.join(" ")
        if (this.debounceTimer !== null) {
            clearTimeout(this.debounceTimer)
        }
        this.search()
    }

    private renderTermButtons() {
        return html`
            <div class="term-btns">
                ${termButtons.map((b) => this.renderTermButton(b))}
                ${this.renderHelpBtn()}
            </div>
        `
    }

    private renderTermButton(b: { term: string, label: string }) {
        var onClass = ""
        if (this.searchedTerms().includes(b.term)) {
            onClass = "term-btn-on"
        }
        return html`
            <button
                class="term-btn ${onClass}"
                @click=${() => this.toggleTerm(b.term)}>
                ${b.label}
            </button>
        `
    }

    private renderHelpBtn() {
        var label = "Search help"
        if (this.showHelp) {
            label = "Hide help"
        }
        return html`
            <button class="help-btn" @click=${this.toggleHelp}>${label}</button>
        `
    }

    private toggleHelp() {
        this.showHelp = !this.showHelp
    }

    // What a search can be written with, for whoever does not know the terms
    // by heart.
    private renderHelp() {
        if (!this.showHelp) {
            return null
        }
        return html`
            <div class="help">
                <p>
                    <code>author:</code> and <code>reviewer:</code> take a
                    username, or <code>me</code> for your own.
                </p>
                <p>
                    <code>is:</code> takes <code>pending</code>,
                    <code>submitted</code>, <code>wip</code>,
                    <code>archived</code>, <code>ready</code>/<code>lgtm</code>,
                    <code>missing-lgtm</code>/<code>no-lgtm</code>,
                    <code>unresolved</code> or
                    <code>missing-owners-approval</code>.
                </p>
                <p>
                    <code>-is:wip</code> and <code>-is:archived</code> leave
                    those commits out instead of asking for them. Archived
                    commits are excluded by default.
                </p>
                <p>
                    Anything else searches the words of the commit message.
                    Quote text that has a colon or starts with a dash, as in
                    <code>"wip: rust rewrite"</code>.
                </p>
            </div>
        `
    }

    private renderClearSearchBtn() {
        if (this.query === "") {
            return null
        }
        return html`
            <button
                class="clear-search-btn"
                title="Clear the search"
                @click=${this.clearSearch}>
                <twigg-icon icon="XMark"></twigg-icon>
            </button>
        `
    }

    private clearSearch() {
        if (this.debounceTimer !== null) {
            clearTimeout(this.debounceTimer)
        }
        this.query = ""
        this.search()
    }

    private renderLoadMoreBtn() {
        if (this.isSearching || this.nextCursor === "") {
            return null
        }
        if (this.isLoadingMore) {
            return html`<simple-loader></simple-loader>`
        }
        return html`
            <button class="load-more-btn" @click=${this.loadMoreCommits}>
                View more
            </button>
        `
    }

    private renderResults() {
        if (this.isSearching) {
            return html`<simple-loader></simple-loader>`
        }
        if (this.searchError !== "") {
            return html`<p class="search-error">${this.searchError}</p>`
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
        var statusClass = "commit-pending"
        if (commit.IsSubmitted) {
            statusClass = "commit-submitted"
        }
        var message = commit.Message
        var urlToCommit = UrlToCommit(this.RepoOwnerName, this.RepoName,
            commit.L, "feed")
        if (commit.L === 0) {
            statusClass = "first-commit"
            message = FirstCommitMsg
            urlToCommit = ""
        }
        const isWip = commit.L != 0 && !commit.IsSubmitted && IsWipCommit(message)
        const lastUpdated = FormatRelativeTime(commit.CreatedOn);
        const submitWillConflict = this.willConflictByCommitId[String(commit.L)] === true
        const commitLift = html`
                <div class="commit twigg-lift ${statusClass}">
                    <span class="commit-size-tag-span">
                        <commit-size-tag .Commit=${commit}></commit-size-tag>
                    </span>
                    <span class="commit-author" ?hidden=${commit.L === 0}>
                        <username-tag username=${commit.AuthorUsername}></username-tag>
                    </span>
                    <commit-number .Number=${commit.L}></commit-number>
                    <span class="commit-message">${message}</span>
                    <span class="commit-last-updated" ?hidden=${commit.L == 0}>
                        ${!commit.IsSubmitted ? "Last updated: " : "Submitted: "}
                        ${lastUpdated}
                    </span>
                    <div>
                        ${this.renderCommitStatus(commit, submitWillConflict, isWip)}
                    </div>
                </div>
        `
        // The first commit is not a commit anybody can open.
        if (urlToCommit === "") {
            return html`<a>${commitLift}</a>`
        }
        return html`<a href=${urlToCommit}>${commitLift}</a>`
    }
    private renderCommitStatus(commit, submitWillConflict, isWip) {
        if (commit.IsSubmitted) {
            return html`<commit-status .Status=${"submitted"}></commit-status>`
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

    static styles = [TwiggCss, css`
        .search-bar-container {
            display: flex;
            align-items: center;
            position: relative;
        }
        /* The one the browser draws is not of this palette */
        .search-bar::-webkit-search-cancel-button {
            -webkit-appearance: none;
            appearance: none;
        }
        .clear-search-btn {
            position: absolute;
            right: var(--space2);
            display: flex;
            align-items: center;
            padding: 0;
            border: none;
            background: none;
            cursor: pointer;
            color: var(--color-text-muted);
            font-size: var(--space4);
        }
        .clear-search-btn:hover {
            color: var(--color-text);
        }
        .search-bar {
            width: 100%;
            padding: var(--space2) var(--space3);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background-color: var(--color-surface);
            color: var(--color-text);
            font-family: var(--font-family);
            font-size: var(--space4);
            padding-right: var(--space6m);
        }
        .search-error {
            color: var(--color-danger);
            font-size: var(--space3);
        }
        .no-commits {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .term-chips {
            display: flex;
            flex-wrap: wrap;
            align-items: center;
            gap: var(--space1);
            margin-top: var(--space2);
        }
        .no-terms {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .term-chip {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            padding: var(--space0) var(--space1) var(--space0) var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius3);
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .term-chip-of-key {
            border-color: var(--color-primary);
            color: var(--color-primary-pop);
        }
        .remove-term-btn {
            display: flex;
            padding: 0;
            border: none;
            background: none;
            color: inherit;
            cursor: pointer;
            font-size: var(--space3);
        }
        .term-btns {
            display: flex;
            flex-wrap: wrap;
            gap: var(--space1);
            margin: var(--space2) 0;
        }
        .term-btn {
            padding: var(--space0) var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background: var(--color-surface);
            color: var(--color-text-muted);
            font-family: var(--font-family);
            font-size: var(--space3);
            cursor: pointer;
        }
        .term-btn:hover {
            color: var(--color-text);
        }
        .term-btn-on {
            border-color: var(--color-primary);
            color: var(--color-text);
        }
        .help-btn {
            margin-left: auto;
            padding: var(--space0) var(--space2);
            border: none;
            background: none;
            color: var(--color-text-muted);
            font-family: var(--font-family);
            font-size: var(--space3);
            text-decoration: underline;
            cursor: pointer;
        }
        .help-btn:hover {
            color: var(--color-text);
        }
        .help {
            padding: var(--space2) var(--space3);
            margin-bottom: var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background: var(--color-surface);
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .help p {
            margin: var(--space1) 0;
        }
        .help code {
            padding: var(--space0) var(--space1);
            border-radius: var(--radius0);
            background: var(--color-surface-alt);
            color: var(--color-text);
        }
        .load-more-btn-container {
            display: flex;
            justify-content: center;
        }
        .load-more-btn {
            background: var(--color-surface);
            color: var(--color-text);
        }
        .commits {
            display: flex;
            flex-direction: column;
            gap: var(--space1);
        }
        a {
            color: inherit;
            text-decoration: none;
        }
        a:hover {
            color: inherit;
            text-decoration: underline;
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
        .commit-last-updated {
            margin-left: var(--space3);
            margin-right: var(--space3);
            font-size: var(--space3);
            color: var(--color-text-muted);
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
                font-size: var(--space5);
            }
            .commit-last-updated {
                margin: 0;
                margin-top: var(--space1);
                margin-right: auto;
                font-size: var(--space4);
            }
            .commit > div:last-child {
                margin-top: var(--space1);
                margin-left: 0;
            }
        }
    `]
}
customElements.define('commit-search', CommitSearch)

declare global {
    interface HTMLElementTagNameMap {
        'commit-search': CommitSearch;
    }
}
