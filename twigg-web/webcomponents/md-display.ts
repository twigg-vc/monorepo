import { LitElement, html, css } from 'lit';
import { TwiggCss } from './css';
import { marked } from 'marked';
import DOMPurify from 'dompurify';

class MarkdownDisplay extends LitElement {
    static properties = {
        content: { type: String }
    };
    declare content: string;
    declare renderedHtml: string;

    static styles = [
        TwiggCss,
        css`
        .main {
            font-family: monospace;
            background-color: var(--color-surface);
            border: 1px solid var(--color-border);
            padding: var(--space1) var(--space2);
            border-radius: var(--radius1);
            color: var(--color-text);
            font-size: var(--space4m);
            overflow-wrap: break-word;
            resize: vertical;
        }
        a {
            color: var(--color-primary-pop);
            text-decoration: underline;
        }
        code {
            background-color: var(--color-surface-alt);
            padding: 0.2em 0.4em;
            border-radius: 3px;
            font-size: 0.99em;
        }
        pre code {
            display: block;
            margin: 0.25em 0;
            padding: 1em;
            border-radius: 5px;
            overflow-x: auto;
        }`];

    constructor() {
        super();
        this.content = '';
    }

    render() {
        return html`<div
        class="main" .innerHTML=${DOMPurify.sanitize(marked.parse(this.content, { async: false }))}></div>`;
    }
}

customElements.define('md-display', MarkdownDisplay);
declare global {
    interface HTMLElementTagNameMap {
        'md-display': MarkdownDisplay;
    }
}