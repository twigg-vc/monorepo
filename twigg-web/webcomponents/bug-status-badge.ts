import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { BugStatus } from './interfaces';

/**
* Open/closed status of a bug: a colored pill, or only its icon with IconOnly
*/
export class BugStatusBadge extends LitElement {
    static properties = {
        Status: { type: String },
        IconOnly: { type: Boolean },
    };
    declare Status: BugStatus
    declare IconOnly: boolean

    constructor() {
        super();
        this.Status = "open"
        this.IconOnly = false
    }

    render() {
        var icon: string | undefined = undefined
        var label: string | undefined = undefined
        if (this.Status === "open") {
            icon = "Bug"
            label = "Open"
        } else {
            icon = "Check"
            label = "Closed"
        }
        if (this.IconOnly) {
            return html`<twigg-icon class="icon ${this.Status}" .icon=${icon} title=${label}></twigg-icon>`
        }
        return html`<span class="pill ${this.Status}"><twigg-icon .icon=${icon}></twigg-icon>${label}</span>`
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            display: inline-flex;
        }
        .icon {
            font-size: var(--space5p);
        }
        .icon.open {
            color: var(--color-success);
        }
        .icon.closed {
            color: var(--color-primary-pop);
        }
        .pill {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            border-radius: 999px;
            padding: var(--space1) var(--space3);
            font-weight: var(--weight-semi-bold);
            font-size: var(--space3p);
        }
        .pill.open {
            background: var(--color-success);
            color: var(--color-status-text);
        }
        .pill.closed {
            background: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        `,
    ];
}
customElements.define('bug-status-badge', BugStatusBadge);
declare global {
    interface HTMLElementTagNameMap {
        'bug-status-badge': BugStatusBadge;
    }
}
