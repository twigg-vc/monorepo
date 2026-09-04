import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

/**
 * Simple span that shows the username with some style
 */
export class UsernameTag extends LitElement {
    static properties = {
        Username: { type: String },
    };

    constructor() {
        super();
        this.Username = 'User';
    }

    declare Username: string;

    render() {
        return html`<span>@${this.Username}</span>`;
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            display: inline;
        }
        span {
            font-weight: var(--weight-semi-bold);
            font-size: var(--space4m);
            color: var(--color-text);
            background: var(--color-surface-alt);
            padding: var(--space0) var(--space2);
            border-radius: var(--radius0); /* small radius */
            box-shadow: var(--shadow-surface); /* subtle shadow */
        }
    `];
}

customElements.define('username-tag', UsernameTag);

declare global {
    interface HTMLElementTagNameMap {
        'username-tag': UsernameTag;
    }
}
