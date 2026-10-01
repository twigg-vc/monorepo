import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Bug, BugEvent, BugStatus } from './interfaces';
import { GetCsrfHeaders, PathToBugComments, PathToBugDescription, PathToBugStatus, UrlToRepoBugsTab } from './routes';
import { FormatDateTime, FormatRelativeTime } from './helpers';
import { IconName } from './icons';
import { MdInput2, MdInputSubmit } from './md-input2';
import { DescriptionSaved } from './cl-description';
import './bug-status-badge';
import './comments';

/**
* Page of a single bug
*/
export class BugPage extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        Bug: { type: Object },
        Events: { type: Array },
        CanWrite: { type: Boolean },

        commentDraft: { type: String, state: true },
        isChangingStatus: { type: Boolean, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Bug: Bug | undefined;
    declare Events: BugEvent[];
    declare CanWrite: boolean;
    declare private commentDraft: string;
    declare private isChangingStatus: boolean;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.Bug = undefined;
        this.Events = [];
        this.CanWrite = false;
        this.commentDraft = "";
        this.isChangingStatus = false;
    }

    render() {
        if (this.Bug === undefined) {
            return html``
        }
        return html`
            <div class="main">
                <div class="crumbs">
                    <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                    <bread-crumbs-space></bread-crumbs-space>
                    <bread-crumbs Name=${this.RepoName} Link=${UrlToRepoBugsTab(this.RepoOwnerName, this.RepoName)}></bread-crumbs>
                    <bread-crumbs-space></bread-crumbs-space>
                    <bread-crumbs id="current-crumb" Name="b/${this.Bug.Number}" Link=""></bread-crumbs>
                </div>
                <div class="bug-header">
                    <h1>${this.Bug.Title}</h1>
                    <bug-status-badge .Status=${this.Bug.Status}></bug-status-badge>
                </div>
                <div class="layout">
                    <div class="content">
                        <h2 class="section-title">Description</h2>
                        ${this.renderDescription(this.Bug)}
                        <h2 class="section-title activity-title">Activity</h2>
                        ${this.renderTimeline(this.Bug)}
                        ${this.renderComposer(this.Bug)}
                    </div>
                    ${this.renderSidebar(this.Bug)}
                </div>
            </div>
        `
    }

    private renderDescription(b: Bug) {
        if (!this.CanWrite) {
            return html`<md-display .content=${this.bodyOrPlaceholder(b.Body)}></md-display>`
        }
        return html`
            <cl-description
                .description=${b.Body}
                .postDescriptionUrl=${PathToBugDescription(this.RepoOwnerName, this.RepoName, b.Number)}
                placeholder="Enter description (markdown supported)"
                @description-saved=${this.onDescriptionSaved}>
            </cl-description>
        `
    }

    private renderSidebar(b: Bug) {
        return html`
            <aside class="sidebar card">
                <div class="meta-item">
                    <span class="meta-label">Assignee</span>
                    ${this.renderAssignee(b)}
                </div>
                <div class="meta-item">
                    <span class="meta-label">Author</span>
                    <username-tag .Username=${b.AuthorUsername}></username-tag>
                </div>
                <div class="meta-item">
                    <span class="meta-label">Opened</span>
                    <span title=${FormatDateTime(b.CreatedOn)}>${FormatRelativeTime(b.CreatedOn)}</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">Last updated</span>
                    <span title=${FormatDateTime(b.UpdatedOn)}>${FormatRelativeTime(b.UpdatedOn)}</span>
                </div>
                <div class="meta-item">
                    <span class="meta-label">Comments</span>
                    <span>${b.CommentCount}</span>
                </div>
            </aside>
        `
    }

    private renderAssignee(b: Bug) {
        if (b.AssigneeUsername === "") {
            return html`<span class="no-assignee">No assignee</span>`
        } else {
            return html`<username-tag .Username=${b.AssigneeUsername}></username-tag>`
        }
    }

    private renderTimeline(b: Bug) {
        if (this.Events.length === 0) {
            return html`<p class="empty-activity">No activity yet.</p>`
        }
        const titlesAfterEdits = this.titlesAfterEdits(b.Title)
        return html`
            <div class="timeline">
                ${this.Events.map((e, i) => this.renderEvent(e, titlesAfterEdits[i]))}
            </div>
        `
    }

    private renderEvent(e: BugEvent, titleAfterEdit: string) {
        switch (e.Kind) {
            case "comment":
                return this.renderComment(e)
            case "status-change":
                return this.renderStatusChange(e)
            case "description-edit":
                return this.renderDescriptionEdit(e)
            case "title-edit":
                return this.renderTitleEdit(e, titleAfterEdit)
        }
    }

    // Returns the bug title right after each event
    private titlesAfterEdits(currentTitle: string): string[] {
        const titles: string[] = new Array(this.Events.length)
        var title = currentTitle
        for (let i = this.Events.length - 1; i >= 0; i--) {
            titles[i] = title
            const e = this.Events[i]
            if (e.Kind === "title-edit") {
                title = e.TitleEdit!.OldTitle
            }
        }
        return titles
    }

    private renderTitleEdit(e: BugEvent, newTitle: string) {
        return html`
            <div class="event">
                <span class="event-icon"><twigg-icon icon="None"></twigg-icon></span>
                <username-tag .Username=${e.AuthorUsername}></username-tag>
                <span title=${FormatDateTime(e.CreatedOn)}>
                    changed the title <s class="old-title">${e.TitleEdit!.OldTitle}</s>
                    <span class="new-title">${newTitle}</span>
                    ${FormatRelativeTime(e.CreatedOn)}
                </span>
            </div>
        `
    }

    private renderComment(e: BugEvent) {
        return html`
            <div class="post card">
                <comment-display .Comment=${{ AuthorUsername: e.AuthorUsername, Text: e.Comment!.Body, T: e.CreatedOn }}>
                </comment-display>
            </div>
        `
    }

    private renderStatusChange(e: BugEvent) {
        const newStatus = e.StatusChange!.NewStatus
        var verb: string | undefined = undefined
        var icon: IconName | undefined = undefined
        if (newStatus === "open") {
            verb = "reopened"
            icon = "Refresh"
        } else {
            verb = "closed"
            icon = "Check"
        }
        return html`
            <div class="event">
                <span class="event-icon ${newStatus}"><twigg-icon .icon=${icon}></twigg-icon></span>
                <username-tag .Username=${e.AuthorUsername}></username-tag>
                <span title=${FormatDateTime(e.CreatedOn)}>${verb} this ${FormatRelativeTime(e.CreatedOn)}</span>
            </div>
        `
    }

    private renderDescriptionEdit(e: BugEvent) {
        return html`
            <details class="event-details">
                <summary class="event" title="Show the previous description">
                    <span class="event-icon expand"><twigg-icon icon="ChevronRight"></twigg-icon></span>
                    <username-tag .Username=${e.AuthorUsername}></username-tag>
                    <span>edited the description ${FormatRelativeTime(e.CreatedOn)}</span>
                </summary>
                <md-display .content=${this.bodyOrPlaceholder(e.DescriptionEdit!.OldBody)}></md-display>
            </details>
        `
    }

    private renderComposer(b: Bug) {
        if (!this.CanWrite) {
            return html``
        }
        return html`
            <div class="composer">
                <md-input2
                    id="composer"
                    .InputIsOpen=${true}
                    .CloseInputBtnIsHidden=${true}
                    ContentPlaceholder="Leave a comment (markdown supported)"
                    SubmitBtnText="Comment"
                    SubmitBtnIcon="ChatBubbleLeft"
                    @md-input-changed=${(e: CustomEvent) => { this.commentDraft = e.detail.NewContent }}
                    @md-input-submit=${this.postComment}>
                    ${this.renderStatusBtn(b)}
                </md-input2>
            </div>
        `
    }

    private renderStatusBtn(b: Bug) {
        const hasComment = this.commentDraft.trim() !== ""
        var label: string | undefined = undefined
        var icon: IconName | undefined = undefined
        if (b.Status === "open") {
            icon = "Check"
            if (hasComment) {
                label = "Close with comment"
            } else {
                label = "Close bug"
            }
        } else {
            icon = "Refresh"
            if (hasComment) {
                label = "Reopen with comment"
            } else {
                label = "Reopen bug"
            }
        }
        return html`
            <button slot="extra-btn" class="status-btn" ?disabled=${this.isChangingStatus}
                @click=${this.toggleStatus}>
                <twigg-icon .icon=${icon}>${label}</twigg-icon>
            </button>
        `
    }

    private composer(): MdInput2 {
        return this.shadowRoot!.getElementById("composer") as MdInput2
    }

    private resetComposer() {
        const c = this.composer()
        c.UpdateContent("")
        c.InputIsOpen = true
        this.commentDraft = ""
    }

    private async onDescriptionSaved(e: CustomEvent<DescriptionSaved>) {
        try {
            const data = await e.detail.Response.json() as { Bug: Bug, Event: BugEvent }
            this.Bug = data.Bug
            this.Events = [...this.Events, data.Event]
        } catch (err) {
            console.log("failed to read the saved description: ", err)
            location.reload()
        }
    }

    private async postComment(e: CustomEvent<MdInputSubmit>) {
        const b = this.Bug!
        try {
            const resp = await fetch(PathToBugComments(this.RepoOwnerName, this.RepoName, b.Number), {
                method: 'POST',
                body: JSON.stringify({ Body: e.detail.NewContent }),
                headers: { ...GetCsrfHeaders(), "Content-Type": "application/json" },
            })
            if (!resp.ok) {
                alert(await resp.text())
                this.composer().StopLoading()
                return
            }
            const event = await resp.json() as BugEvent
            this.Bug = { ...b, CommentCount: b.CommentCount + 1, UpdatedOn: event.CreatedOn }
            this.Events = [...this.Events, event]
            this.resetComposer()
        } catch (err) {
            console.log("failed to post comment: ", err)
            alert("Failed to post comment :(")
            this.composer().StopLoading()
        }
    }

    private async toggleStatus() {
        const b = this.Bug!
        var newStatus: BugStatus | undefined = undefined
        if (b.Status === "open") {
            newStatus = "closed"
        } else {
            newStatus = "open"
        }
        this.isChangingStatus = true
        try {
            const resp = await fetch(PathToBugStatus(this.RepoOwnerName, this.RepoName, b.Number), {
                method: 'POST',
                body: JSON.stringify({ Status: newStatus, Comment: this.commentDraft }),
                headers: { ...GetCsrfHeaders(), "Content-Type": "application/json" },
            })
            if (!resp.ok) {
                alert(await resp.text())
                return
            }
            const data = await resp.json() as { Bug: Bug, Events: BugEvent[] }
            this.Bug = data.Bug
            this.Events = [...this.Events, ...data.Events]
            this.resetComposer()
        } catch (err) {
            console.log("failed to change status: ", err)
            alert("Failed to change the bug status :(")
        } finally {
            this.isChangingStatus = false
        }
    }

    private bodyOrPlaceholder(body: string): string {
        if (body === "") {
            return "_No description provided._"
        } else {
            return body
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
        .main {
            max-width: var(--size4);
            margin: auto;
        }
        .bug-header {
            display: flex;
            align-items: center;
            flex-wrap: wrap;
            gap: var(--space3);
            margin-top: var(--space2);
            margin-bottom: var(--space4);
        }
        .bug-header h1 {
            overflow-wrap: anywhere;
        }
        .layout {
            display: grid;
            grid-template-columns: minmax(0, 1fr) var(--size0);
            align-items: start;
            gap: var(--space5p);
        }
        .content {
            min-width: 0;
        }
        .section-title {
            font-size: var(--space4);
            color: var(--color-text-muted);
            margin-bottom: var(--space2);
        }
        .activity-title {
            margin-top: var(--space5p);
        }
        .empty-activity {
            color: var(--color-text-muted);
            font-style: italic;
        }
        .sidebar {
            display: flex;
            flex-direction: column;
            gap: var(--space3);
            padding: var(--space4);
            position: sticky;
            top: var(--space4);
        }
        .meta-item {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            gap: var(--space1);
        }
        .meta-item + .meta-item {
            border-top: 1px solid var(--color-border);
            padding-top: var(--space3);
        }
        .no-assignee {
            color: var(--color-text-muted);
            font-style: italic;
        }
        .meta-label {
            color: var(--color-text-muted);
            font-size: var(--space3);
            font-weight: var(--weight-semi-bold);
            text-transform: uppercase;
            letter-spacing: 0.04em;
        }
        .timeline {
            display: flex;
            flex-direction: column;
            gap: var(--space3);
        }
        .post {
            padding: var(--space3);
        }
        .event {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: 0 var(--space4);
            color: var(--color-text-muted);
            font-size: var(--space3p);
        }
        .event-icon {
            display: inline-flex;
            border-radius: 50%;
            padding: var(--space1);
        }
        .event-icon.closed {
            background: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        .event-icon.open {
            background: var(--color-success);
            color: var(--color-status-text);
        }
        .old-title {
            color: var(--color-text-muted);
        }
        .new-title {
            color: var(--color-text);
            font-weight: var(--weight-semi-bold);
        }
        .event-details summary {
            cursor: pointer;
            list-style: none;
        }
        .event-details summary:hover {
            color: var(--color-text);
        }
        .event-icon.expand twigg-icon {
            transition: transform .15s;
        }
        .event-details[open] .event-icon.expand twigg-icon {
            transform: rotate(90deg);
        }
        .event-details md-display {
            display: block;
            margin: var(--space2) var(--space4) 0 var(--space6);
        }
        .composer {
            margin-top: var(--space4);
        }
        .status-btn {
            background: var(--color-surface);
            color: var(--color-text);
            font-size: var(--space5);
        }
        .status-btn[disabled] {
            opacity: var(--disable-opacity-value);
            cursor: not-allowed;
        }
        @media (max-width: 760px) {
            .layout {
                grid-template-columns: minmax(0, 1fr);
            }
            .sidebar {
                position: static;
                order: -1;
            }
        }
        `
    ];
}
customElements.define('bug-page', BugPage);
declare global {
    interface HTMLElementTagNameMap {
        'bug-page': BugPage;
    }
}