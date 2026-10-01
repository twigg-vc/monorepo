import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Bug } from './interfaces';
import { PathToBugDescription, UrlToRepoBugsTab } from './routes';
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
                <h2 class="section-title">Description</h2>
                ${this.renderDescription(this.Bug)}
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
        .section-title {
            font-size: var(--space4);
            color: var(--color-text-muted);
            margin-bottom: var(--space2);
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