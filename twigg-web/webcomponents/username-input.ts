import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

declare global {
    interface HTMLElementEventMap {
        "user-selected": CustomEvent<UserSelected>;
    }
}
export interface UserSelected {
    Username: string
}

/**
 * Username picker. Read or set Value; input events report edits.
 * Enter or a button click emits user-selected with a trimmed, nonempty username.
 * Embedded appearance relies on the container for its border and focus highlight.
 */
export class UsernameInput extends LitElement {
    static properties = {
        Placeholder: { type: String },
        ButtonText: { type: String },
        Disabled: { type: Boolean },
        Value: { type: String },
        Label: { type: String },
        HideButtonWhenEmpty: { type: Boolean },
        Appearance: { type: String, reflect: true },
    };
    declare Placeholder: string;
    declare ButtonText: string;
    declare Disabled: boolean;
    declare Value: string;
    declare Label: string;
    declare HideButtonWhenEmpty: boolean;
    declare Appearance: "standalone" | "embedded";

    constructor() {
        super();
        this.Placeholder = "Username";
        this.ButtonText = "Select";
        this.Disabled = false;
        this.Value = "";
        this.Label = "Username";
        this.HideButtonWhenEmpty = false;
        this.Appearance = "standalone";
    }

    override focus(options?: FocusOptions) {
        const input = this.renderRoot?.querySelector('input')
        if (input) {
            input.focus(options)
        } else {
            this.updateComplete.then(() => {
                this.renderRoot.querySelector('input')?.focus(options)
            })
        }
    }

    render() {
        return html`
            <input
                aria-label=${this.Label}
                .value=${this.Value}
                placeholder=${this.Placeholder}
                ?disabled=${this.Disabled}
                @input=${(e: Event) => { this.Value = (e.target as HTMLInputElement).value }}
                @keydown=${this.onKeyDown}
            />
            ${this.renderButton()}
        `
    }

    private renderButton() {
        if (this.HideButtonWhenEmpty && this.Value.trim() === "") {
            return html``
        }
        return html`
            <button type="button" ?disabled=${this.Disabled || this.Value.trim() === ""} @click=${this.select}>
                ${this.ButtonText}
            </button>
        `
    }

    private onKeyDown(e: KeyboardEvent) {
        if (e.key === "Enter" && !e.isComposing) {
            e.preventDefault()
            this.select()
        }
    }

    private select() {
        const username = this.Value.trim()
        if (this.Disabled || username === "") {
            return
        }
        this.dispatchEvent(new CustomEvent<UserSelected>('user-selected', {
            detail: { Username: username },
            bubbles: true,
            composed: true,
        }))
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            display: flex;
            gap: var(--space1);
        }
        input {
            flex: 1;
            min-width: 0;
            width: 0;
            font: inherit;
            padding: var(--space1) var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            background: var(--color-surface-alt);
            color: var(--color-text);
        }
        input:focus {
            outline: none;
            border-color: var(--color-primary-pop);
        }
        button {
            background: var(--color-surface);
            color: var(--color-text);
            font: inherit;
            padding: var(--space1) var(--space4);
        }
        :host([appearance="embedded"]) {
            gap: 0;
        }
        :host([appearance="embedded"]) input {
            border: none;
            border-radius: 0;
            background: none;
            padding: var(--space1) var(--space3);
        }
        :host([appearance="embedded"]) input:focus {
            background: var(--color-surface-alt);
        }
        :host([appearance="embedded"]) button {
            border: none;
            border-left: var(--border-width) solid var(--color-border);
            border-radius: 0;
            background: none;
            color: var(--color-primary-pop);
            padding: var(--space1) var(--space3);
        }
        :host([appearance="embedded"]) button:hover {
            transform: none;
            background: var(--color-surface-alt);
        }
        input:disabled,
        button:disabled {
            opacity: var(--disable-opacity-value);
            cursor: not-allowed;
        }
        `
    ];
}
customElements.define('username-input', UsernameInput);
declare global {
    interface HTMLElementTagNameMap {
        'username-input': UsernameInput;
    }
}
