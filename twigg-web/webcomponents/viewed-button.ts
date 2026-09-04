import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

/**
* Simple btn to mark "Viewed"
*/
export class ViewedButton extends LitElement {
    declare private viewed: boolean;
    static properties = {
        viewed: { type: Boolean },
    };
    constructor() {
        super();
        this.viewed = false;
    }
    render() {
        return html`
            <div
            class="${this.getBtnClass()}"
            @click=${this.onClick}
            >
                <input type="checkbox" .checked=${this.viewed}>
                <span class="unselectable-text">Viewed</span>
            </div>
        `;
    }

    private getBtnClass() {
        if (this.viewed) {
            return "btn selected"
        }
        return "btn not-selected"
    }
    private onClick(event){
        event.stopPropagation();
        this.viewed = !this.viewed
    }

    static styles = [
        TwiggCss,
        css`
        .btn{
            appearance: none;
            border: 1px solid var(--color-border);
            border-radius: 999px;
            padding: var(--space1) var(--space3);
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: var(--space2);
            line-height: 1.1;
            transition: .15s transform;
        }
        .selected{
            background-color: var(--color-surface-alt);
        }
        .not-selected{
            background-color: var(--color-surface);
            color: var(--color-text);
        }
        .unselectable-text{
            user-select: none;   /* Standard */
            -webkit-user-select: none; /* Safari */
            -moz-user-select: none;    /* Firefox */
            -ms-user-select: none;     /* IE10+ */
        }
    `];
}
customElements.define('viewed-button', ViewedButton);
declare global {
    interface HTMLElementTagNameMap {
        'viewed-button': ViewedButton;
    }
}