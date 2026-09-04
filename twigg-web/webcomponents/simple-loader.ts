import { css, html, LitElement } from 'lit';
import { TwiggCss } from './css';

export class SimpleLoader extends LitElement {
    static styles = [
    TwiggCss,
        css`
    .spinner {
        width: var(--space5);
        height: var(--space5);
        border: 3px solid rgba(255, 255, 255, 0.2);
        border-top-color: var(--color-primary);
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
        margin: auto;
    }
    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }
  `];
    render() {
        return html`<div class="spinner"></div>`;
    }
}

customElements.define("simple-loader", SimpleLoader);
declare global {
    interface HTMLElementTagNameMap {
        'simple-loader': SimpleLoader;
    }
}