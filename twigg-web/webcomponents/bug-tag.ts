import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { BugPriority } from './interfaces';

/**
* A small bordered label with a bug's priority, shown next to its title
*/
export class BugTag extends LitElement {
    static properties = {
        Priority: { type: String },
    };
    declare Priority: BugPriority

    constructor() {
        super();
        this.Priority = ""
    }

    render() {
        return html`<span class="tag">${this.Priority}</span>`
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            display: inline-flex;
        }
        .tag {
            padding: 0 var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            color: var(--color-text-muted);
            font-size: var(--space3p);
        }
        `,
    ];
}
customElements.define('bug-tag', BugTag);
declare global {
    interface HTMLElementTagNameMap {
        'bug-tag': BugTag;
    }
}
