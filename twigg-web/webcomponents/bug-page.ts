import { html, css, LitElement } from 'lit';
import { Bug } from './interfaces';
import { UrlToRepoBugsTab } from './routes';
import './bug-status-badge';
import './cl-description';

/**
* Page of a single bug
*/
export class BugPage extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        Bug: { type: Object },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Bug: Bug | undefined;

    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.Bug = undefined;
    }

    render() {
        if (this.Bug === undefined) {
            return html``
        }
        return html`
            <div>
                <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs Name=${this.RepoName} Link=${UrlToRepoBugsTab(this.RepoOwnerName, this.RepoName)}></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs Name="b/${this.Bug.Number}" Link=""></bread-crumbs>
            </div>
            <h1 class="bug-title">
                <bug-status-badge .Status=${this.Bug.Status}></bug-status-badge>
                ${this.Bug.Title} <span class="bug-number">b/${this.Bug.Number}</span>
            </h1>
            <h3>Description</h3>
            <cl-description .description=${this.displayedDescription()} .canEdit=${false}></cl-description>
        `
    }

    private displayedDescription(): string {
        if (this.Bug === undefined || this.Bug.Body === "") {
            return "`[no description]`"
        }
        return this.Bug.Body
    }

    static styles = css`
        .bug-title {
            display: flex;
            align-items: center;
            gap: var(--space3);
        }
        .bug-number {
            color: var(--color-text-muted);
        }
    `;
}
customElements.define('bug-page', BugPage);
declare global {
    interface HTMLElementTagNameMap {
        'bug-page': BugPage;
    }
}
