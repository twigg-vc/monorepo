import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Thread } from './interfaces';
import { FormatDateTime } from './helpers';

/**
 * Displays a reviewer add/remove "thread".
 * go-to - Element shown that links back to the commit/version this
 * thread belongs to.
 */
export class ReviewerThread extends LitElement {
    static properties = {
        Thread: { type: Object },
    };
    declare Thread: Thread

    render() {
        if (this.Thread.Type !== "AddReviewer" && this.Thread.Type !== "RemoveReviewer") {
            throw new Error(`bad thread type for ReviewerThread: ${this.Thread.Type}`)
        }
        var isAdd = this.Thread.Type === "AddReviewer"
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
                        class="${isAdd ? 'add' : 'remove'}"
                        icon="${isAdd ? 'Check' : 'XMark'}">
                        <span class="text">${this.renderText(isAdd)}</span>
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

    private renderText(isAdd: boolean){
        if (isAdd){
            return html`added <username-tag username=${this.Thread.TargetUsername}></username-tag> as a reviewer`
        }
        return html`removed <username-tag username=${this.Thread.TargetUsername}></username-tag> as a reviewer`
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
            gap: var(--space4);
        }
        .first-column {
            flex: auto;
            display: flex;
            align-items: center;
            gap: var(--space2);
        }
        .lgtm-time {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .second-column {
            flex: 0 1 auto;
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
customElements.define('reviewer-thread', ReviewerThread);
declare global {
    interface HTMLElementTagNameMap {
        'reviewer-thread': ReviewerThread;
    }
}
