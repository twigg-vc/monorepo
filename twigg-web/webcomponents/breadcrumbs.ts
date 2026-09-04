import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';

export class BreadCrumbs extends LitElement {
    static properties = {
        Name: { type: String },
        Link: { type: String },
    };
    constructor() {
        super();
        this.Name = ""
        this.Link = ""
    }
    declare Name: string
    declare Link: string

    render() {
        return html`<a part="link" href=${this.Link}>${this.Name}</a>`
    }

    static styles = [
        TwiggCss,
        css`
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