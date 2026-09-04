import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { JobStatus } from './interfaces';

/**
* A simple status "LED" (a small colored circle)
*/
export class StatusLed extends LitElement {
    static properties = {
        Status: { type: String },
    };
    declare Status: JobStatus
    constructor() {
        super();
    }

    render() {
        return html`<div class="status-indicator ${this.Status}"></div>`;
    }
    static styles = [
        TwiggCss,
        css`
        .status-indicator {
            width: var(--fixedSpace3);
            height: var(--fixedSpace3);
            border-radius: 50%; /*Make it a circle*/
            flex-shrink: 0;
            background: var(--color-text-muted); /* Default for unknown statuses */
        }
        .status-indicator.waiting-manual-start,
        .status-indicator.waiting { 
            background: var(--color-warning); 
        }

        .status-indicator.queued,
        .status-indicator.posted { background: var(--color-info); }

        .status-indicator.running { 
            background: var(--color-primary); 
            box-shadow: var(--shadow-pop);
        }

        .status-indicator.success { background: var(--color-success); }

        .status-indicator.fail,
        .status-indicator.timeout,
        .status-indicator.cancel,
        .status-indicator.too-many-jobs,
        .status-indicator.bad-file-format,
        .status-indicator.bad-file-size,
        .status-indicator.exceeds-plan-limits { background: var(--color-danger); }
        `,
    ];
}
customElements.define('status-led', StatusLed);
declare global {
    interface HTMLElementTagNameMap {
        'status-led': StatusLed;
    }
}