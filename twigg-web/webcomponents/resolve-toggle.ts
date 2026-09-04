import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';


/**
 * "Checkbox" btn to select to resolve/unresolve a thread
 * @fires toggled - When toggled
 */
export class ResolveToggle extends LitElement {
    static properties = {
        IsCurrentlyResolved: { type: String },

        WillChangeResolveStatus: { type: Boolean },
    };
    constructor() {
        super();
        this.IsCurrentlyResolved = false
        this.WillChangeResolveStatus = false
    }
    declare IsCurrentlyResolved: boolean
    declare WillChangeResolveStatus: boolean


    render() {
        let prefixText = ""
        if (this.IsCurrentlyResolved) {
            prefixText = "Unresolve"
        } else {
            prefixText = "Resolve"
        }
        let finalStatus: "resolved"|"unresolved" = undefined
        if (this.IsCurrentlyResolved) {
            if (this.WillChangeResolveStatus){
                finalStatus = "unresolved"
            }else{
                finalStatus = "resolved"
            }
        } else {
            if (this.WillChangeResolveStatus) {
                finalStatus = "resolved"
            } else {
                finalStatus = "unresolved"
            }
        }
        return html`
        <span
        @click=${this.onToggleClicked}
        class="btn ${finalStatus}">
            <input
            type="checkbox"
            .checked=${this.WillChangeResolveStatus}
            @click=${this.onToggleClicked}>
            <span class="resolve-text">${prefixText}</span>
        </span>
        `
    }

    private onToggleClicked(ev){
        this.WillChangeResolveStatus = !this.WillChangeResolveStatus
        ev.stopPropagation()
        this.dispatchEvent(new Event('toggled', {
            bubbles: true,
            composed: true
        }));
    }

    static styles = [
        TwiggCss,
        css`
        .btn{
            padding: var(--space0) var(--space2);
            border-radius: var(--radius2);
            display: flex;
            align-items: center;
            font-size: var(--space4m);
            gap: var(--space1);
        }
        .resolve-text{
            user-select: none;
        }
        .resolved{
            background-color: var(--color-surface-alt);
            color: var(--color-text-muted);
        }
        .unresolved{
            background-color: var(--color-surface-alt);
            box-shadow: var(--shadow-pop);
            color: var(--color-text);
        }
    `];
}
customElements.define('resolve-toggle', ResolveToggle);
declare global {
    interface HTMLElementTagNameMap {
        'resolve-toggle': ResolveToggle;
    }
}