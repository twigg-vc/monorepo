import { LitElement, html, css } from 'lit';
import { TwiggCss } from './css';
import { ThemeStoreSingleton } from './theme-store';
import { SetFeatureFlags, FeatureFlags } from './feature-flags';

// This is the "main" web-component that will contain all the others.
export class TwiggApp extends LitElement {
    static properties = {
        Theme: { type: String },
        HideNavBar: { type: Boolean },
        FeatureFlags: { type: Object },
        Version: { type: String },
    };
    declare HideNavBar: boolean;
    declare FeatureFlags: FeatureFlags
    declare Version: string;

    static styles = [
        TwiggCss,
        css`
        :host {
            display: flex;
            flex-direction: column;
            min-height: 100vh;
            min-width: 100%; /* fill parent width */
            background: var(--color-bg);
            color: var(--color-text);
        }
        main {
            flex: 1;
            margin: var(--fixedSpace1);
        }
        footer {
            border-top: 1px solid var(--color-border);
            padding: var(--space3) 0;
            text-align:center;
            color:var( --color-text-muted);
            font-size:.95rem;
            max-height: 10vh;
        }
        #version-container{
            display:flex;
            justify-content:flex-end;
            align-items:flex-end;
        }
        #version-p{
            color:var(--color-bg);
            font-size: var(--space2);
        }
    `,
    ];
    constructor(){
        super()
        ThemeStoreSingleton.Init()
        this.HideNavBar = false;
    }
    firstUpdated() {
        SetFeatureFlags(this.FeatureFlags)
    }

    render() {
    return html`
    ${this.HideNavBar ? html``:html`<nav-bar></nav-bar>`}
      <main>
        <slot></slot>
      </main>
        <div id="version-container">
            <p id="version-p">${this.Version}</p>
        </div>
      <footer>
        <help-fab Position="right" Label="Help" Title="Any questions?"></help-fab>
        <p>&copy; ${new Date().getFullYear()} Twigg</p>
      </footer>
    `;
    }
}

customElements.define("twigg-app", TwiggApp);
declare global {
    interface HTMLElementTagNameMap {
        'twigg-app': TwiggApp;
    }
}