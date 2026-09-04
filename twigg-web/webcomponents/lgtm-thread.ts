import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Thread } from './interfaces';
import { FormatDateTime } from './helpers';

/**
 * Displays an lgmt "thread".
 * LGTM threads are not really threads in the same way as comment threads,
 * but we represent them as threads for convenience; since they're displayed
 * together with comment threads. This component renders an LGTM thread.
 */
export class LgtmThread extends LitElement {
    static properties = {
        Thread: { type: Object },
    };
    declare Thread: Thread

    render() {
        return html`
        <div class="main">
            <div class="go-to-row">
                <slot name="go-to"></slot>
            </div>
            <div class="columns-row">
                <div class="first-column">
                    <username-tag username=${this.Thread.AuthorUsername}></username-tag>
                    ${this.renderTime()}
                </div>
                <div class="second-column">
                    <twigg-icon
                        class="${this.Thread.IsLgtm ? 'add' : 'remove'}"
                        icon="${this.Thread.IsLgtm ? 'Check' : 'XMark'}">
                        <span class="text">${this.renderText()}</span>
                    </twigg-icon>
                </div>
                <div class="third-column"></div>
            </div>
        </div>
    `
    }

    private renderTime() {
        var createdOn = this.Thread.CreatedOn
        return html`<span class="lgtm-time">${FormatDateTime(createdOn)}</span>`
    }

    private renderText(){
        if (this.Thread.IsLgtm){
            return "LGTM"
        }
        return "LGTM"
    }

    static styles = [
        TwiggCss,
        css`
        .main{
            display:flex;
            flex-direction: column; 
            padding: var(--space2) var(--space4);
            background: var(--color-surface);
            border-radius: var(--radius1);
            border: 1px solid var(--color-border);
        }
        .go-to-row {
            display: flex;
            justify-content: center;
        }
        .columns-row {
            display: flex;
            justify-content: center;
            align-items: center;
        }
        .first-column {
            flex: 1;
            display: flex;
            align-items: center;
            gap: var(--space2);
        }
        .lgtm-time {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .second-column {
            flex: 1;
            font-weight: var(--weight-semi-bold);
        }

        .third-column {
            flex: 1;
        }
        .text {
            color: var(--color-text);
        }
        .add{
            color: var(--color-success);
        }
        .remove{
            color: var(--color-warning);
        }
    `];
}
customElements.define('lgtm-thread', LgtmThread);
declare global {
    interface HTMLElementTagNameMap {
        'lgtm-thread': LgtmThread;
    }
}