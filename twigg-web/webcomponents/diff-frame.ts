import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Commit, FileStatus, Thread, User } from './interfaces'
import { DiffDisplay, RequestLineSlot } from './diff-display'
import { IconName } from './icons'
import { UrlToGetFile, UrlToPostNewThread, UrlToPostNewThreadOnLine } from "./routes"

/**
* Element inside which a diff will be displayed.
* It takes care of lazily fetching the diff element
* only when it'll be displayed. I.e. it makes a get request and puts
* the returned html inside the frame.
*/
export class DiffFrame extends LitElement {
    declare RepoOwnerName: string
    declare RepoName: string
    // Commits being displayed
    declare LeftCommit: Commit
    declare RightCommit: Commit
    // Name of the file of which the diff is displayed
    declare filename: string;
    // Url that will provide the html fragment that will be displayed
    // inside the frame
    declare diffGetUrl: string;
    // Comment threads related to this diff
    declare LeftThreads: Thread[];
    declare RightThreads: Thread[];
    declare CurrentUser: User;
    declare FileStatus: FileStatus;
    static properties = {
        filename: { type:String },
        isCollapsed: { type:Boolean },
        diffGetUrl: { type:String },
        isLoading: { type: Boolean },
        unifiedDiffString: { type: String },
        LeftThreads: { type:Array },
        RightThreads: { type: Array },
        RepoName: { type: String },
        RepoOwnerName: { type: String },
        LeftCommit: {type: Object},
        RightCommit: { type: Object },
        CurrentUser: { type: Object },
        FileStatus: { type: String },

        requestedLeftSlotLines: { type: Array, state: true },
        requestedRightSlotLines: { type: Array, state: true },
        expandedSlots: { type: Array, state: true },
        expandedUnresolvedInlineThreads: { type: Boolean, state: true },
    };
    constructor() {
        super();
        this.isCollapsed = true;
        this.filename = "";
        this.diffGetUrl = "";
        this.isLoading = false;
        this.fetchedContent = false;
        this.unifiedDiffString = ""
        this.LeftThreads = [];
        this.RightThreads = [];
        this.requestedLeftSlotLines = [];
        this.requestedRightSlotLines = [];
        this.expandedSlots = [];
        this.expandedUnresolvedInlineThreads = false;
    }
    declare private expandedSlots: string[];
    declare private expandedUnresolvedInlineThreads: boolean;
    declare private isCollapsed: boolean;
    declare private isLoading: boolean;
    declare private fetchedContent: boolean;
    declare private unifiedDiffString: string;
    declare private requestedLeftSlotLines: number[];
    declare private requestedRightSlotLines: number[];

    willUpdate() {
        this.expandUnresolvedInlineThreadsIfNotExpanded()
    }

    // Unresolved inline threads must start expanded.
    // This function expands them if they are not yet expanded.
    private expandUnresolvedInlineThreadsIfNotExpanded() {
        if (this.expandedUnresolvedInlineThreads) {
            return
        }
        // Only set expandedUnresolvedInlineThreads after we're sure to have init
        if (this.LeftThreads.length == 0 && this.RightThreads.length == 0){
            return
        }
        this.expandedUnresolvedInlineThreads = true
        this.expandedSlots = [...this.expandedSlots,
            ...getUnresolvedInlineThreadsSlotNames(this.LeftThreads,
                DiffDisplay.LeftLineSlotName),
            ...getUnresolvedInlineThreadsSlotNames(this.RightThreads,
                DiffDisplay.RightLineSlotName)]
    }

    render() {
        let leftInlineThreads: InlineThreads[] = []
        if (!this.leftCommitIsParent()){
            leftInlineThreads = getInlineThreads(
                this.LeftThreads,
                this.requestedLeftSlotLines)
        }
        const rightInlineThreads = getInlineThreads(
            this.RightThreads,
            this.requestedRightSlotLines)
        return html`
        <div class="main">
            <div
            class="header"
            @click=${this.openOrDoNothing}
            >
                ${this.icon()}
                <h3>${this.filename}</h3>
                ${this.renderStatusPill()}
                <viewed-button class="viewed-button"></viewed-button>
                ${this.renderInlineThreadsTag(leftInlineThreads,
                    rightInlineThreads)}
                <a class="download-btn" 
                href="${UrlToGetFile(this.RepoOwnerName, this.RepoName,
                    this.RightCommit.L, this.RightCommit.Version, this.filename)}" 
                    download="${this.nameOfFileToDownload()}">
                    <twigg-icon icon="Download">
                        Download v${this.RightCommit.Version}
                    </twigg-icon>
                </a>
            </div>
            ${this.loader()}
            <div style=${this.fetchedContent && !this.isCollapsed ?
                    "" :
                    "display:none"}>
                <diff-display
                    @request-line-slot=${this.onRequestLineSlot}
                    .unifiedDiff=${this.unifiedDiffString}
                    .leftFilename=${this.filename}
                    .rightFilename=${this.filename}
                    .LeftSlotLines=${leftInlineThreads.map((t) => t.Line)}
                    .RightSlotLines=${rightInlineThreads.map((t) => t.Line)}
                >
                    ${this.renderLeftThreadsSlot(leftInlineThreads)}
                    ${this.renderRightThreadsSlot(rightInlineThreads)}
                </diff-display>
            </div>
        </div>
        `;
    }

    // Asking twice for the same line closes the box that was opened for it.
    private onRequestLineSlot(e: CustomEvent<RequestLineSlot>) {
        e.stopPropagation()
        const leftLine = e.detail.LeftLine
        const rightLine = e.detail.RightLine
        let isToggle = false 
        if (isRequested(this.requestedRightSlotLines, rightLine)) {
            isToggle = true
            this.requestedRightSlotLines = withoutLine(
                this.requestedRightSlotLines, rightLine)
        }
        if (isRequested(this.requestedLeftSlotLines, leftLine)){
            isToggle = true
            this.requestedLeftSlotLines = withoutLine(
                this.requestedLeftSlotLines, leftLine)
        }
        if (isToggle){
            return
        }
        this.requestedLeftSlotLines = withLine(
            this.requestedLeftSlotLines, leftLine)
        this.requestedRightSlotLines = withLine(
            this.requestedRightSlotLines, rightLine)
        this.expandedSlots = [...this.expandedSlots,
            ...getRequestedSlotNames(leftLine, rightLine)]
    }
    // Shows the tag with number of inline threads
    private renderInlineThreadsTag(left: InlineThreads[],
        right: InlineThreads[]) {
        if (left.length + right.length == 0){
            return html``
        }
        
        const allInlineThreads = [...left, ...right]
        let hasUnresolved = false
        for (const inlineThread of allInlineThreads) {
            for (const threads of inlineThread.Threads){
                if (!threads.IsResolved){
                    hasUnresolved = true
                    break
                }
            }
            if (hasUnresolved){break}
        }
        let nThreads = 0
        for (const inlineThread of [...left, ...right]){
            nThreads += inlineThread.Threads.length
        }
        return html`
        <comments-tag class="inline-threads-tag"
            title="Comment threads on the lines of this file"
            .HasUnresolvedComments=${hasUnresolved}
            .CommentsCount=${nThreads}></comments-tag>
        `
    }

    private renderLeftThreadsSlot(inline: InlineThreads[]) {
        if (this.leftCommitIsParent()) {
            return html``
        }
        const nonInlineThreads = getNonInlineThreads(this.LeftThreads)
        return html`
        ${this.renderThreadsSlot("left", this.LeftCommit, nonInlineThreads,
            /*line=*/0)}

        ${inline.map((inlineThreads) => this.renderInlineThreadsSlot(
            DiffDisplay.LeftLineSlotName(inlineThreads.Line), this.LeftCommit,
            inlineThreads))}
        `
    }
    private renderRightThreadsSlot(inline: InlineThreads[]){
        const nonInlineThreads = getNonInlineThreads(this.RightThreads)
        return html`
        ${this.renderThreadsSlot("right", this.RightCommit, nonInlineThreads,
            /*line=*/0)}

        ${inline.map((inlineThreads) => this.renderInlineThreadsSlot(
            DiffDisplay.RightLineSlotName(inlineThreads.Line), this.RightCommit,
            inlineThreads))}
        `
    }
    private renderInlineThreadsSlot(slotName: string, c: Commit,
        inlineThreads: InlineThreads) {

        return html`
        ${this.renderExpandBtn(slotName, inlineThreads)}
        ${this.renderExpandedThreads(slotName, c, inlineThreads)}
        `
    }

    private renderExpandBtn(slotName: string, inlineThreads: InlineThreads) {
        var icon: IconName = "ChevronRight"
        var title = "Show the comment threads of this line"
        if (this.slotIsExpanded(slotName)) {
            icon = "ChevronDown"
            title = "Hide the comment threads of this line"
        }
        return html`
        <button slot=${slotName} title=${title}
            class=${this.expandBtnClass(inlineThreads)}
            @click=${()=>this.toggleSlotIsExpanded(slotName)}>
            <twigg-icon .icon=${icon}></twigg-icon>
            ${this.renderExpandBtnContent(inlineThreads)}
        </button>
        `
    }

    private expandBtnClass(inlineThreads: InlineThreads): string {
        const hasUnresolved = inlineThreads.Threads.some((t) => !t.IsResolved)
        if (hasUnresolved) {
            return "expand-btn not-resolved"
        }
        return "expand-btn"
    }

    private renderExpandBtnContent(inlineThreads: InlineThreads) {
        const n = inlineThreads.Threads.length
        if (n === 1) {
            return html`<span>1 comment thread</span>`
        }
        return html`<span>${n} comment threads</span>`
    }

    private renderExpandedThreads(slotName: string, c: Commit,
        inlineThreads: InlineThreads) {

        if (!this.slotIsExpanded(slotName)) {
            return html``
        }
        return this.renderThreadsSlot(slotName, c, inlineThreads.Threads,
            inlineThreads.Line)
    }

    private slotIsExpanded(slotName: string): boolean {
        return this.expandedSlots.includes(slotName)
    }

    private toggleSlotIsExpanded(slotName: string) {
        if (this.slotIsExpanded(slotName)) {
            this.expandedSlots = this.expandedSlots.filter(
                (s) => s !== slotName)
            return
        }
        this.expandedSlots = [...this.expandedSlots, slotName]
    }

    // Use line!=0 for inline threads and line=0 for threads anchored to a file
    private renderThreadsSlot(slotName: string, c: Commit, threads: Thread[],
        line: number) {

        return html`
        <comment-threads
            slot=${slotName}
            .newThreadPostUrl=${this.getNewThreadPostUrl(c, line)}
            .Threads=${threads}
            .RepoOwnerName=${this.RepoOwnerName}
            .RepoName=${this.RepoName}
            .Commit=${c}
            .CurrentUser=${this.CurrentUser}
            >
        </comment-threads>
        `
    }
    private getNewThreadPostUrl(c: Commit, line: number): string {
        if (line === 0) {
            return UrlToPostNewThread(this.RepoOwnerName, this.RepoName,
                c.L, c.Version, this.filename)
        }
        return UrlToPostNewThreadOnLine(this.RepoOwnerName, this.RepoName,
            c.L, c.Version, this.filename, line)
    }

    private async fetchDiff(){
        this.isLoading = true;
        try {
            const resp = await fetch(this.diffGetUrl, {
                method: 'GET',
            });
            this.unifiedDiffString = await resp.text();
            this.fetchedContent = true;
            this.isLoading = false;
        } catch (error) {
            console.log("Error when making request: ", error)
            this.isCollapsed = true;
            this.isLoading = false;
            return
        }
    }

    private async openOrDoNothing() {
        // Dont do anything if already open
        if (!this.isCollapsed){
            return;
        }
        this.isCollapsed = !this.isCollapsed
        if (!this.fetchedContent){
            await this.fetchDiff()
        }
    }
    
    private async onIconClicked(event){
        event.stopPropagation()
        if (!this.fetchedContent){
            this.openOrDoNothing()
            return;
        }
        this.isCollapsed = !this.isCollapsed
    }
    
    private loader(){
        if (this.isCollapsed){
            return html``
        }
        if (this.isLoading){
            return html`
                <simple-loader></simple-loader>
            `
        }
        return html``
    }

    private leftCommitIsParent(): boolean{
        return this.LeftCommit.L != this.RightCommit.L
    }

    private nameOfFileToDownload(): string{
        return "c"+this.RightCommit.L+"v"+this.RightCommit.Version+"-"+this.filename
    }

    static styles = [
        TwiggCss,
        css`
        .main{
            background-color: var(--color-bg);
            border-radius: var(--radius1);
            margin: var(--space2) var(--space2);
            border: 1px solid var(--color-border);
        }
        .header{
            position: sticky;
            top: 0;
            z-index: 1;
            display: flex;
            flex-wrap: wrap;
            align-items: center;
            background-color: var(--color-surface);
            border-radius: var(--radius1);
            padding: var(--space1);
        }
        .header h3{
            min-width: 0;
            overflow-wrap: anywhere;
        }
        .viewed-button{
            margin-left: var(--space4);
        }
        .toggle-icon{
            border-radius: var(--radius1);
            font-size: var(--space5);
            margin: var(--space1);
        }
        .toggle-icon:hover{
            background-color: var(--color-surface-alt);
        }

        .pill {
            margin-left: var(--space2); 
            padding: var(--space0) var(--space2);
            border-radius: var(--radius2);
            border: 1px solid currentColor;
            white-space: nowrap;
            font-size: var(--space4);
            line-height: 1.1;
        }
        .pill--created {
            background: var(--color-soft-success);
            border-color:var(--color-success);
        }
        .pill--deleted {
            background: var(--color-danger-soft);
            border-color:var(--color-danger);
        }
        .download-btn {
            margin-left: auto;
        }
        .expand-btn{
            display: flex;
            align-items: center;
            gap: var(--space1);
            margin: 0 auto;
            width: fit-content;
            user-select: none;
            font-size: var(--space3);
            background-color: var(--color-surface);
            color: var(--color-text-muted);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            padding: var(--space0) var(--space1);
            cursor: pointer;
        }
        .inline-threads-tag{
            margin-left: var(--space4);
        }
        .expand-btn.not-resolved{
            border-color: var(--color-warning);
            box-shadow: var(--shadow-pop-yellow);
        }
        .expand-btn:hover{
            color: var(--color-primary-pop);
            background-color: var(--color-surface-alt);
        }
    `];
    
    private icon(){
        if (this.isCollapsed){
            return html`<twigg-icon class="toggle-icon"
                @click=${this.onIconClicked}
                .icon=${"ChevronRight"}></twigg-icon>`
        }
        return html`<twigg-icon
        class="toggle-icon" @click=${this.onIconClicked}
        .icon=${"ChevronDown"}></twigg-icon>`
    }
    private renderStatusPill() {
        switch (this.FileStatus) {
        case "c":
            return html`<span class="pill pill--created">Created</span>`
        case "d":
            return html`<span class="pill pill--deleted">Deleted</span>`
        case "m":
            return html``
        default:
            console.error("invalid FileStatus value")
        }
    }
}
customElements.define('diff-frame', DiffFrame);

// InlineThreads maps line to inline threads
interface InlineThreads {
    Line: number
    Threads: Thread[]
}

// getInlineThreads returns the threads anchored to a line, grouped by line.
function getInlineThreads(threads: Thread[], requestedSlotLines: number[]): InlineThreads[] {
    // First create a map of line to the threads on each line
    const lineToThreads = new Map<number, Thread[]>()
    for (const t of threads) {
        const isInline = t.Line !== 0
        if (!isInline) {
            continue
        }
        const threads = lineToThreads.get(t.Line)
        if (threads === undefined) {
            lineToThreads.set(t.Line, [t])
        } else {
            threads.push(t)
        }
    }

    // Simply convert that map into a list
    const threadsList: InlineThreads[] = []
    for (const [line, threadsOnLine] of lineToThreads) {
        threadsList.push({ Line: line, Threads: threadsOnLine })
    }

    // Add empty threads for the requestedSlotLines
    for (const line of requestedSlotLines) {
        const hasThreads = threadsList.some((t) => t.Line === line)
        if (hasThreads) {
            continue
        }
        threadsList.push({ Line: line, Threads: [] })
    }
    return threadsList
}

// returns the names the slot of every line that has an unresolved thread on it
function getUnresolvedInlineThreadsSlotNames(
    threads: Thread[],
    slotNameFunc: (line: number) => string): string[] {
    const names: string[] = []
    for (const t of threads) {
        if (t.Line === 0 || t.IsResolved) {
            continue
        }
        names.push(slotNameFunc(t.Line))
    }
    return names
}

// A line of "" means the row has no line on that side.
function isRequested(lines: number[], line: number | ""): boolean {
    if (line === "") {
        return false
    }
    return lines.includes(line)
}

function withLine(lines: number[], line: number | ""): number[] {
    if (line === "") {
        return lines
    }
    return [...lines, line]
}

function withoutLine(lines: number[], line: number | ""): number[] {
    if (line === "") {
        return lines
    }
    return lines.filter((l) => l !== line)
}

function getRequestedSlotNames(
    leftLine: number | "",
    rightLine: number | ""): string[] {
    const names: string[] = []
    if (leftLine !== "") {
        names.push(DiffDisplay.LeftLineSlotName(leftLine))
    }
    if (rightLine !== "") {
        names.push(DiffDisplay.RightLineSlotName(rightLine))
    }
    return names
}

// getNonInlineThreads returns the threads that are anchored to the file as
// a whole and not to a line.
function getNonInlineThreads(threads: Thread[]): Thread[] {
    return threads.filter((t) => t.Line === 0)
}

declare global {
    interface HTMLElementTagNameMap {
        'diff-frame': DiffFrame;
    }
}