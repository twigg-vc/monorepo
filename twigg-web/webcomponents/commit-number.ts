import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

/**
 * Simple span that shows the commit number with some style
 */
export class CommitNumber extends LitElement {
    static properties = {
        Number: { type: String },
    };
    constructor() {
        super();
        this.Number = 0
    }
    declare Number: number

    render() {
        return html`<span>c/${this.Number}</span>`
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            background-color: transparent;
            display: inline;
        }
        span {
            font-weight: var(--weight-bold);
            color: var(--color-text);
            background: var(--color-bg);
            padding: var(--space1) var(--space3);
            border-radius: var(--radius2);
            box-shadow: var(--shadow-pop);
        }
    `];
}
customElements.define('commit-number', CommitNumber);
declare global {
    interface HTMLElementTagNameMap {
        'commit-number': CommitNumber;
    }
}