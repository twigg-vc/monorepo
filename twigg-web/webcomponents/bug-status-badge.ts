import { html, css, LitElement } from 'lit';
import { BugStatus } from './interfaces';

/**
* Open/closed status of a bug, as a colored icon
*/
export class BugStatusBadge extends LitElement {
    static properties = {
        Status: { type: String },
    };
    declare Status: BugStatus

    constructor() {
        super();
        this.Status = "open"
    }

    render() {
        if (this.Status === "open") {
            return html`<twigg-icon class="open" icon="Bug" title="Open"></twigg-icon>`
        } else {
            return html`<twigg-icon class="closed" icon="Check" title="Closed"></twigg-icon>`
        }
    }

    static styles = css`
        :host {
            display: inline-flex;
            font-size: var(--space5p);
        }
        .open {
            color: var(--color-success);
        }
        .closed {
            color: var(--color-primary-pop);
        }
    `;
}
customElements.define('bug-status-badge', BugStatusBadge);
declare global {
    interface HTMLElementTagNameMap {
        'bug-status-badge': BugStatusBadge;
    }
}
