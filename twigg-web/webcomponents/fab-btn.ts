import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { DiscordSupportUrl } from './routes';

export class HelpFab extends LitElement {
    static properties = {
        Label: { type: String },
        Title: { type: String },
        Position: { type: String }, // 'right' | 'left'
    };
    constructor() {
        super();
        this.Label = '?';
        this.Title = '';
        this.Position = 'right';
    }
    declare Label: string
    declare Title: string
    declare Position: 'right' | 'left'

    render() {
        return html`
            <a href="${DiscordSupportUrl}" class="help-btn" title=${this.Title}>
                <button part="btn">
                    <twigg-icon class="tab-icon" icon="Question"></twigg-icon>
                </button>
            </a>
        `
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            position: fixed;
            inset: auto 30px 60px auto; /* bottom-right */
            z-index: 1000;
        }
        :host([Position="left"]) {
            inset: auto auto 60px 30px; /* bottom-left */
        }
        button[part="btn"] {
            display: inline-grid;
            place-items: center;
            width: 44px;
            height: 44px;
            border-radius: 9999px;
            border: 1px solid var(--color-border, #d9d9d9);
            background: var(--color-primary, #7c5cff);
            color: var(--color-on-primary, #fff);
            cursor: pointer;
        }
        .tab-icon{
            font-size: var(--space5p);
        }

        .help-btn{
            transition: .15s transform;
        }
        .help-btn:hover {
            transform: translateY(-5px);
        }
        `
    ];
}
customElements.define('help-fab', HelpFab);
declare global {
    interface HTMLElementTagNameMap {
        'help-fab': HelpFab;
    }
}