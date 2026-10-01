import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Bug } from './interfaces';
import { PathToBugDescription, UrlToRepoBugsTab } from './routes';
import { FormatDateTime, FormatRelativeTime } from './helpers';
import './bug-status-badge';

/**
* Page of a single bug
*/
export class BugPage extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        Bug: { type: Object },
        CanWrite: { type: Boolean },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Bug: Bug | undefined;
    declare CanWrite: boolean;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.Bug = undefined;
        this.CanWrite = false;
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
                placeholder="Enter description (markdown supported)">
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