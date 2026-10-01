import { html, css, LitElement, TemplateResult } from 'lit';
import { TwiggCss } from './css';
import { IconName } from './icons';

const copiedFeedbackMs = 1500

/**
 * Link in a breadcrumb trail. With CopyOnClick, clicking copies the Link's
 * full url instead of navigating to it.
 */
export class BreadCrumbs extends LitElement {
    static properties = {
        Name: { type: String },
        Link: { type: String },
        CopyOnClick: { type: Boolean },

        copied: { type: Boolean, state: true },
    };
    constructor() {
        super();
        this.Name = ""
        this.Link = ""
        this.CopyOnClick = false
        this.copied = false
    }
    declare Name: string
    declare Link: string
    declare CopyOnClick: boolean
    declare private copied: boolean

    render() {
        if (!this.CopyOnClick) {
            return html`<a part="link" href=${this.Link}>${this.Name}</a>`
        }
        var icon: IconName | undefined = undefined
        var title: string | undefined = undefined
        var iconCls: string | undefined = undefined
        var copiedLabel: TemplateResult | undefined = undefined
        if (this.copied) {
            iconCls = "copy-icon shown"
            icon = "Check"
            title = "Copied!"
            copiedLabel = html`<span class="copied-label">Copied!</span>`
        } else {
            iconCls = "copy-icon"
            icon = "ContentCopy"
            title = "Copy link"
            copiedLabel = html``
        }
        return html`
            <a part="link" class="copy" href=${this.Link} title=${title} @click=${this.copyLink}>
                ${this.Name}
                <twigg-icon class=${iconCls} .icon=${icon}></twigg-icon>
            </a>
            ${copiedLabel}
        `
    }

    private async copyLink(e: MouseEvent) {
        if (e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) {
            return
        }
        e.preventDefault()
        try {
            await navigator.clipboard.writeText(new URL(this.Link, window.location.href).href)
        } catch (err) {
            console.log("failed to copy link: ", err)
            alert("Failed to copy the link :(")
            return
        }
        this.copied = true
        setTimeout(() => { this.copied = false }, copiedFeedbackMs)
    }

    static styles = [
        TwiggCss,
        css`
        :host([copyonclick]) {
            display: inline-flex;
            align-items: center;
            gap: var(--space2);
        }
        .copy {
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            cursor: copy;
        }
        .copy-icon {
            opacity: 0;
            transition: opacity .15s;
        }
        .copy:hover .copy-icon,
        .copy:focus-visible .copy-icon,
        .copy-icon.shown {
            opacity: 1;
        }
        .copied-label {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
    `];
}
customElements.define('bread-crumbs', BreadCrumbs);
declare global {
    interface HTMLElementTagNameMap {
        'bread-crumbs': BreadCrumbs;
    }
}

export class BreadCrumbsSpace extends LitElement {
    static properties = {
    };
    constructor() {
        super();
    }

    render() {
        return html`<twigg-icon icon="ChevronRight"></twigg-icon>`
    }

    static styles = [
        TwiggCss,
        css`
    `];
}
customElements.define('bread-crumbs-space', BreadCrumbsSpace);
declare global {
    interface HTMLElementTagNameMap {
        'bread-crumbs-space': BreadCrumbsSpace;
    }
}