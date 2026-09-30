import { html, css, LitElement } from 'lit';
import { Bug } from './interfaces';
import './bug-status-badge';

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
            <h1 class="bug-title">
                <bug-status-badge .Status=${this.Bug.Status}></bug-status-badge>
                ${this.Bug.Title} <span class="bug-number">b/${this.Bug.Number}</span>
            </h1>
        `
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
