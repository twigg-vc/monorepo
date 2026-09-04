import { css, html, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Commit, Thread, User } from './interfaces';
import { FirstCommitMsg } from './commit-display';
import { FormatRelativeTime } from './helpers';

export class VersionSelector extends LitElement {
    // Username of the repository owner (used to determine URLs).
    declare public RepositoryOwner: string;
    // Name of the repository (used to determine URLs).
    declare public RepositoryName: string;
    // Contains each version of the commit v0, v1, ... until
    // the latest version (included).
    declare public Versions: Commit[];
    // Contains the parent of each commit version
    declare public Parents: Commit[];
    // Number identifying what's selected on the left. Use -1 to select parent.
    declare public LeftVersion: number;
    // Number identifying what's selected on the right. Use -1 to select latest.
    declare public RightVersion: number;
    // Contains the threads for all versions
    declare Threads: Thread[];
    static properties = {
        Versions: { type: Array },
        Parents: { type: Array },
        LeftVersion: { type: String },
        RightVersion: { type: String },
        RepositoryName: { type: String },
        RepositoryOwner: { type: String },
        Threads: { type: Array },
    };
    constructor() {
        super();
        this.Versions = []
        this.Parents = []
        this.RepositoryName = ""
        this.RepositoryOwner = ""
        this.LeftVersion = -1;
        this.RightVersion = -1;
        this.Threads = []
    }


    render() {
        var parentAndVersions = this.getParentAndVersions()
        var left : Commit
        if (this.LeftVersion == -1){
            left = parentAndVersions[0]
        }else{
            left = this.Versions[this.LeftVersion]
        }
        var right: Commit
        if (this.RightVersion == -1) {
            right = this.Versions[this.Versions.length -1]
        } else {
            right = this.Versions[this.RightVersion]
        }
        const threadsByVersion = this.getThreadsByVersion()
        return html`
        <div class="main">
            <span class="dropdown-label-span">Left:</span>
            <div class="dropdown">
                ${this.renderLeftDropBtn()}
                <div class="dropdown-content">
                    ${parentAndVersions.map(
                        (c, i) => {
                            const isParent = i == 0
                            if (!isParent && c.Version >= right.Version){
                                return html``
                            }
                            
                            var threads: Thread[] | undefined = undefined
                            if (!isParent) {
                                threads = threadsByVersion.get(c.Version)
                            }
                            return this.renderDropdownEntry(left, right, c, true, threads)
                        }
                    )}
                </div>
            </div>
            <span class="dropdown-label-span">Right:</span>
            <div class="dropdown">
                ${this.renderRightDropBtn(right)}
                <div class="dropdown-content">
                    ${parentAndVersions.map(
                        (c, i) => {
                            if (i == 0 || c.Version <= this.LeftVersion) {
                                return html``
                            }
                            return this.renderDropdownEntry(left, right, c, false, threadsByVersion.get(c.Version))
                    })}
                </div>
            </div>
        </div>
    `;
    }

    private renderLeftDropBtn() {
        const isParent = this.LeftVersion == -1
        if (isParent) {
            return html`
                <button class="btn dropbtn" aria-haspopup="true">
                    Parent
                    <twigg-icon icon="ChevronDown"></twigg-icon>
                </button>
            `
        }
        return html`
            <button class="btn dropbtn" aria-haspopup="true">
                Version ${this.Versions[this.LeftVersion].Version} · ${FormatRelativeTime(this.Versions[this.LeftVersion].CreatedOn)}
                <twigg-icon icon="ChevronDown"></twigg-icon>
            </button>
        `
    }
    private renderRightDropBtn(rightCommit: Commit) {
        return html`
            <button class="btn dropbtn" aria-haspopup="true">
                Version ${rightCommit.Version} · ${FormatRelativeTime(rightCommit.CreatedOn)}
                <twigg-icon icon="ChevronDown"></twigg-icon>
            </button>
        `
    }

    private renderDropdownEntry(
        left: Commit, right: Commit,
        c: Commit, isLeft: boolean, cThreads: Thread[] | undefined){
        if (!cThreads) {
            cThreads = []
        }
        var isSelected: boolean
        if (isLeft){
            isSelected = c.L == left.L && c.Version == left.Version
        }else{
            isSelected = c.L == right.L && c.Version == right.Version
        }

        let url = ""
        if (isLeft) {
            url = urlToCommit(
                this.RepositoryOwner,
                this.RepositoryName,
                c,
                right,
                this.Versions[this.Versions.length - 1],
            )
        } else {
            url = urlToCommit(
                this.RepositoryOwner,
                this.RepositoryName,
                left,
                c,
                this.Versions[this.Versions.length - 1],
            )
        }

        let hasUnresolvedComments = false
        let textualThreadsCount = 0
        for (const t of cThreads) {
            if (!t.IsResolved) {
                hasUnresolvedComments = true
            }
            if (t.Type != "AddLGTM" && t.Type != "RemoveLGTM"){
                textualThreadsCount += 1
            }
        }
        const commentsTag = html`
        <comments-tag
            .HasUnresolvedComments=${hasUnresolvedComments}
            .CommentsCount=${textualThreadsCount}
            title="Comment threads on this version"
        >
        </comments-tag>
        `
        
        var elementContents = html``
        var isParent = c.L != right.L
        if (isParent){
            let parentMsg = ""
            if (c.L == 0){
                parentMsg = FirstCommitMsg
            }else{
                parentMsg = c.Message
            }
            elementContents = html`
            <div class="dropdown-entry ${isSelected ? "selected": ""}">
                <span class="parent-span">(Parent)</span>
                c/${c.L} v${c.Version} - ${parentMsg}
            </div>
            `
        }else{
            elementContents = html`
            <div class="dropdown-entry ${isSelected ? "selected" : ""}">
                Version ${c.Version}
                <span class="version-time">${FormatRelativeTime(c.CreatedOn)}</span>
                ${commentsTag}
            </div>
            `
        }
        if (isSelected){
            return elementContents
        }
        return html`
            <a href=${url}>
                ${elementContents}
            </a>
        `
    }
    // Returns an array that contains the parent of the right version followed
    // by all the versions
    private getParentAndVersions(): Commit[]{
        if (this.RightVersion == -1){
            return [this.Parents[this.Parents.length-1], ...this.Versions]        
        }
        return [this.Parents[this.RightVersion], ...this.Versions]
    }
    // returns the threads of each version
    private getThreadsByVersion(): Map<number, Thread[]> {
        const m = new Map<number, Thread[]>()
        for (const t of this.Threads) {
            const threads = m.get(t.CommitVersion)
            if (threads) {
                threads.push(t)
            } else {
                m.set(t.CommitVersion, [t])
            }
        }
        return m
    }

    static styles = [
        TwiggCss,
        css`
        .main{
            padding: var(--space2)  var(--space4) 0 var(--space4);
        }
        .dropdown-label-span{
            font-size: var(--space4);
            font-weight: var(--weight-bold);
        }
        .dropdown {
            position: relative;
            display: inline-block;
        }
        .dropbtn {
            white-space: nowrap;
            padding: var(--space2) var(--space3);
            cursor: pointer;
            background: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        .dropbtn twigg-icon {
            margin-left: var(--space1);
        }
        .dropdown-content {
            display: none;
            position: absolute;
            background: var(--color-surface);
            border: 1px solid var(--color-primary-pop);
            border-radius: var(--space2);
            padding: var(--space2);
            z-index: 9999;
            min-width: max-content; /* expand to fit widest entry */
            white-space: nowrap;    /* prevent text breaking */
        }
        .dropdown-entry{
            padding: var(--space2);
        }
        .parent-span{
            padding-right: var(--space2);
        }
        .version-time{
            color: var(--color-text-muted);
            font-size: var(--space3);
            margin-left: var(--space1);
            margin-right: var(--space2);
        }
        .dropdown-entry.selected{
            background: var(--color-surface-alt);
        }
        .dropdown:hover .dropdown-content {
            display: block;
        }
    `];
}

customElements.define("version-selector", VersionSelector);
declare global {
    interface HTMLElementTagNameMap {
        'version-selector': VersionSelector;
    }
}

// This must match the route to the code review page given the
// left/right commit and the versions. See the Commit route in to go code.
// Use -1 for the versions to not specify it in the url
function urlToCommit(owner: string,
    repository: string, left: Commit, right: Commit, latest: Commit): string{
    if (latest.L != right.L){
        throw "wrong use of urlToCommit"
    }
    let query = "?"

    // If ommites, left is considered to be the parent by default
    const leftIsParent = left.L != right.L
    if (!leftIsParent){
        query += `left=${left.Version}`
    }
    // If ommited, right is considered the latest by default
    const rightIsLatest = right.Version == latest.Version
    if (!rightIsLatest) {
        if (query != "?"){
            query += "&"
        }
        query += `right=${right.Version}`
    }
    if (query != "?") { 
        query += "&"
    }
    query += "tab=changes"
    return `/${owner}/${repository}/c/${latest.L}${query}`
}