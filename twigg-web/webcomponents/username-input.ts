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
        // Embedded mode, for an input that sits inside another bordered
        // control (e.g. a segmented button group) which draws the outer border:
        // - the input has no border or background of its own,
        // - the button is split from the input by a divider and only shows
        //   once a username is typed.
        Embedded: { type: Boolean, reflect: true },

        value: { type: String, state: true },
    };
    declare Placeholder: string;
    declare ButtonText: string;
    declare Disabled: boolean;
    declare Embedded: boolean;
    declare private value: string;

    constructor() {
        super();
        this.Placeholder = "Username";
        this.ButtonText = "Select";
        this.Disabled = false;
        this.Embedded = false;
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
            ${this.renderButton()}
        `
    }

    private renderButton() {
        if (this.Embedded && this.value.trim() === "") {
            return html``
        }
        return html`
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
        :host([embedded]) {
            gap: 0;
        }
        :host([embedded]) input {
            flex: 1 1 auto;
            width: calc(2 * var(--space7));
            border: none;
            border-radius: 0;
            background: none;
            padding: var(--space1) var(--space3);
        }
        :host([embedded]) input:focus {
            background: var(--color-surface-alt);
            box-shadow: inset 0 0 0 var(--border-width) var(--color-primary-pop);
        }
        :host([embedded]) input:disabled {
            opacity: var(--disable-opacity-value);
            cursor: not-allowed;
        }
        :host([embedded]) button {
            border: none;
            border-left: var(--border-width) solid var(--color-border);
            border-radius: 0;
            background: none;
            color: var(--color-primary-pop);
            padding: var(--space1) var(--space3);
        }
        :host([embedded]) button:hover {
            transform: none;
            background: var(--color-surface-alt);
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
