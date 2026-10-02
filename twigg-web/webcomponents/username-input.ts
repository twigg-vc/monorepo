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
 * Input for picking a user by username. For now it's a plain text input, so
 * every place that asks for a username can later get a dropdown from here.
 *
 * @fires user-selected CustomEvent<UserSelected>
 * @description Emitted with the trimmed username when the user presses Enter
 * or clicks the button. Not emitted for an empty username.
 */
export class UsernameInput extends LitElement {
    static properties = {
        Placeholder: { type: String },
        ButtonText: { type: String },
        Disabled: { type: Boolean },

        value: { type: String, state: true },
    };
    declare Placeholder: string;
    declare ButtonText: string;
    declare Disabled: boolean;
    declare private value: string;

    constructor() {
        super();
        this.Placeholder = "Username";
        this.ButtonText = "Select";
        this.Disabled = false;
        this.value = "";
    }

    render() {
        return html`
            <input
                .value=${this.value}
                placeholder=${this.Placeholder}
                ?disabled=${this.Disabled}
                @input=${(e: Event) => { this.value = (e.target as HTMLInputElement).value }}
                @keydown=${this.onKeyDown}
            />
            <button ?disabled=${this.Disabled || this.value.trim() === ""} @click=${this.select}>
                ${this.ButtonText}
            </button>
        `
    }

    private onKeyDown(e: KeyboardEvent) {
        if (e.key === "Enter") {
            this.select()
        }
    }

    private select() {
        const username = this.value.trim()
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
        button[disabled] {
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
