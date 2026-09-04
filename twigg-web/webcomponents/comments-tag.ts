import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

// Small tag that shows a comments icon with a count
export class CommentsTag extends LitElement {
    static properties = {
        HasUnresolvedComments: { type: Boolean },
        CommentsCount: { type: Number },
    };
    constructor() {
        super();
        this.HasUnresolvedComments = false
        this.CommentsCount = 0
    }
    declare HasUnresolvedComments: boolean
    declare CommentsCount: number

    render() {
        if (this.CommentsCount == 0) {
            return html``
        }
        var cls = "tag"
        if (this.HasUnresolvedComments) {
            cls = "tag not-resolved"
        }
        return html`
        <span class=${cls}>
            <twigg-icon icon="ChatBubbleLeftRight"></twigg-icon>
            ${this.CommentsCount}
        </span>
        `
    }

    static styles = [
        TwiggCss,
        css`
        :host {
            display: inline-flex;
        }
        .tag{
            display: flex;
            align-items: center;
            gap: var(--space0);
            font-size: var(--space3);
            color: var(--color-text-muted);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            padding: var(--space0) var(--space1);
            user-select: none;
        }
        .tag.not-resolved{
            border-color: var(--color-warning);
            box-shadow: var(--shadow-pop-yellow);
        }
    `];
}
customElements.define('comments-tag', CommentsTag);
declare global {
    interface HTMLElementTagNameMap {
        'comments-tag': CommentsTag;
    }
}
