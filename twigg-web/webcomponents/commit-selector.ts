import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Commit } from './interfaces';
import { PathToSearchCommits } from './routes';
import { MinDurationTimer } from './min-duration-timer';


declare global {
    interface HTMLElementEventMap {
        "selection-changed": CustomEvent<Commit | null>;
    }
}

// Dropdown to select commits
// @fires commit-selected - See `commit-selected` above
export class CommitSelector extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        searchQuery: { type: String },
        isLoading: { type: Boolean },
        loadingFailed: { type: Boolean },
        commits: { type: Array },
        selectedCommit: { type: Object },
        isOpen: { type: Boolean },
    };

    constructor() {
        super();
        this.searchQuery = "";
        this.isLoading = false;
        this.loadingFailed = false;
        this.commits = [];
        this.selectedCommit = null;
        this.isOpen = false;
        this._debounceTimer = null;
    }

    declare public RepoOwnerName: string;
    declare public RepoName: string;
    declare private searchQuery: string;
    declare private isLoading: boolean;
    declare private loadingFailed: boolean;
    declare private commits: Commit[];
    declare private selectedCommit: Commit | null;
    declare private isOpen: boolean;
    private _debounceTimer: ReturnType<typeof setTimeout> | null;

    // onOutsideClick must be constructed inside `connectedCallback`
    private onOutsideClick: (e: MouseEvent) => void;
    connectedCallback() {
        super.connectedCallback();
        this.onOutsideClick = (e: MouseEvent) => {
            // Close the dropdown on outside clicks
            if (!e.composedPath().includes(this)) this.isOpen = false;
        };
        document.addEventListener('click', this.onOutsideClick, { capture: true });
    }
    disconnectedCallback() {
        super.disconnectedCallback();
        document.removeEventListener('click', this.onOutsideClick, { capture: true });
    }

    render() {
        const c = this.selectedCommit;
        if (c) {
            return html`
            <div class="wrap">
                <div class="chip">
                    ${this.renderCommit(c)}
                    <button class="clear-btn" @click=${this.clearSelection}>✕</button>
                </div>
            </div>`
        }

        return html`
            <div class="wrap">
                <div class="input-row">
                    <input
                        type="text"
                        placeholder="c/123, c/123v2…"
                        .value=${this.searchQuery}
                        @input=${this.onInput}
                        @focus=${this.onFocus}
                        autocomplete="off" spellcheck="false"
                    />
                    ${this.isLoading ? html`<simple-loader></simple-loader>` : ''}
                    ${this.loadingFailed ? html`<span class="fetch-error">Failed</span>` : ''}
                </div>
                ${this.isOpen ? html`
                    <div class="dropdown">
                        ${this.commits.map(c => html`
                            <div class="dropdown-row" tabindex="0" @click=${() => this.selectCommit(c)} @keydown=${(e: KeyboardEvent) => e.key === 'Enter' && this.selectCommit(c)}>
                                ${this.renderCommit(c)}
                            </div>`)}
                    </div>` : ''}
            </div>`;
    }

    private renderCommit(c: Commit){
        return html`
        <span class="commit-id">c/${c.L}v${c.Version}</span>
        <span class="commit-msd">${c.Message}</span>
        ${c.IsSubmitted ? html`<commit-status Status="submitted"></commit-status>` : html``}
        `
    }

    private onFocus() {
        this.scheduleFetch();
    }
    private onInput(e: InputEvent) {
        this.searchQuery = (e.target as HTMLInputElement).value;
        this.selectedCommit = null;
        this.loadingFailed = false;
        if (this.searchQuery && !this.searchQueryIsGood()) {
            this.commits = [];
            this.isOpen = false;
            return;
        }
        this.scheduleFetch()
    }
    // Schedules a fetch to be done after a while.
    // If one is already scheduled, it is descheduled.
    private scheduleFetch(){
        if (this._debounceTimer) clearTimeout(this._debounceTimer);
        this._debounceTimer = setTimeout(() => this.fetchCommits(), 300);
    }
    // Makes the request and populates the fields with the commits
    private async fetchCommits() {
        if (this.isLoading){return}
        this.isLoading = true;
        this.isOpen = false;
        try {
            const tw = new MinDurationTimer()
            const resp = await fetch(PathToSearchCommits(
                this.RepoOwnerName,
                this.RepoName, this.searchQuery),
                { method: 'GET' }
            );
            if (!resp.ok) {
                console.warn("Search commits failed:", resp);
                throw new Error("request failed");
            }
            await tw.Wait()
            this.commits = await resp.json();
            this.isOpen = this.commits.length > 0;
        } catch {
            this.loadingFailed = true;
            this.commits = [];
        } finally {
            this.isLoading = false;
        }
    }

    private selectCommit(commit: Commit) {
        this.selectedCommit = commit;
        this.isOpen = false;
        this.commits = [];
        this.dispatchEvent(
            new CustomEvent('selection-changed',
                { detail: commit, bubbles: true, composed: true }
            ));
    }
    private clearSelection() {
        this.selectedCommit = null;
        this.searchQuery = "";
        this.commits = [];
        this.isOpen = false;
        this.updateComplete.then(() => this.shadowRoot?.querySelector('input')?.focus());
        this.dispatchEvent(
            new CustomEvent('selection-changed',
                { detail: null, bubbles: true, composed: true }
            ));
    }

    private searchQueryIsGood(): boolean {
        return /^(?:c\/|c)?(\d+)(?:v(\d+))?$/.test(this.searchQuery);
    }

    static styles = [TwiggCss, css`
        .wrap { position: relative; display: block; }

        .input-row {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: var(--space2);
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius0);
        }
        .input-row:focus-within {
            border-color: var(--color-primary);
            box-shadow: var(--shadow-pop);
        }

        input {
            background: transparent;
            border: none;
            outline: none;
            color: var(--color-text);
            font-family: var(--font-family);
            flex: 1;
        }
        input::placeholder {
            color: var(--color-text-muted);
            opacity: 0.6;
        }

        .chip {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: var(--space1);
            background: var(--color-surface);
            border: 1px solid var(--color-primary);
            border-radius: var(--radius0);
        }
        .clear-btn {
            background: none;
            border: none;
            cursor: pointer;
            padding: var(--space1);
            color: var(--color-text-muted);
            border-radius: var(--radius0);
        }
        .clear-btn:hover { color: var(--color-danger); background: var(--color-danger-soft); }

        .dropdown {
            position: absolute;
            top: calc(100% + var(--space1));
            left: 0;
            width: 100%;
            box-sizing: border-box;
            max-height: 40vh;
            overflow-y: auto;
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            box-shadow: var(--shadow-surface-alt);
            z-index: 100;
        }
        .dropdown-row {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: var(--space2);
            cursor: pointer;
            border-bottom: 1px solid var(--color-border);
        }
        .dropdown-row:last-child { border-bottom: none; }
        .dropdown-row:hover, .dropdown-row:focus { background: var(--color-surface-alt); outline: none; }

        .commit-id { color: var(--color-primary-pop); flex-shrink: 0; }
        .commit-msd { color: var(--color-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0; }

        .fetch-error { color: var(--color-danger); flex-shrink: 0; }
    `];
}
customElements.define('commit-selector', CommitSelector);
declare global {
    interface HTMLElementTagNameMap {
        'commit-selector': CommitSelector;
    }
}