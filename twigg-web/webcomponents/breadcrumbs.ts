import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

/**
 * Link in a breadcrumb trail. With CopyOnClick, clicking copies the Link's
 * full url instead of navigating to it.
 */
export class BreadCrumbs extends LitElement {
    static properties = {
        Name: { type: String },
        Link: { type: String },
        CopyOnClick: { type: Boolean },
    };
    constructor() {
        super();
        this.Name = ""
        this.Link = ""
        this.CopyOnClick = false
    }
    declare Name: string
    declare Link: string
    declare CopyOnClick: boolean

    render() {
        if (!this.CopyOnClick) {
            return html`<a part="link" href=${this.Link}>${this.Name}</a>`
        }
        return html`<a part="link" class="copy" href=${this.Link} title="Copy link" @click=${this.copyLink}>${this.Name}</a>`
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
        }
    }

    static styles = [
        TwiggCss,
        css`
        .copy {
            cursor: copy;
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