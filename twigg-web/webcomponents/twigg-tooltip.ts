import { LitElement, html, css } from 'lit';
import { TwiggCss } from './css';

/**
 * Hover/focus tooltip: the default slot is the trigger, the `content` slot is
 * the popup. The popup is a top-layer `popover` placed next to the trigger,
 * below it when there is room in the viewport, above it otherwise.
 */
export class TwiggTooltip extends LitElement {
    static properties = {
        // "right" aligns the popup's left edge with the trigger, "left" its right edge.
        Side: { type: String },
        opensUp: { type: Boolean, state: true },
    };
    declare Side: "left" | "right"
    declare private opensUp: boolean

    constructor() {
        super()
        this.Side = "right"
        this.opensUp = false
    }

    private tooltipEl(): HTMLElement | null {
        return this.renderRoot.querySelector<HTMLElement>('.tooltip')
    }

    private show() {
        const tooltip = this.tooltipEl()
        if (tooltip === null) {
            return
        }
        if (!tooltip.matches(':popover-open')) {
            tooltip.showPopover()
            window.addEventListener('scroll', this.hide, { capture: true, passive: true })
        }
        this.position(tooltip)
    }

    private readonly hide = () => {
        window.removeEventListener('scroll', this.hide, { capture: true })
        const tooltip = this.tooltipEl()
        if (tooltip !== null && tooltip.matches(':popover-open')) {
            tooltip.hidePopover()
        }
    }

    disconnectedCallback() {
        this.hide()
        super.disconnectedCallback()
    }

    private position(tooltip: HTMLElement) {
        const trigger = this.renderRoot.querySelector('.trigger')
        if (trigger === null) {
            return
        }
        const triggerRect = trigger.getBoundingClientRect()
        const width = tooltip.offsetWidth
        const height = tooltip.offsetHeight

        const roomBelow = window.innerHeight - triggerRect.bottom
        const roomAbove = triggerRect.top
        var opensUp = false
        if (roomBelow < height) {
            opensUp = roomAbove > roomBelow
        }
        this.opensUp = opensUp

        var top = triggerRect.bottom
        if (opensUp) {
            top = triggerRect.top - height
        }
        var left = triggerRect.left
        if (this.Side === "left") {
            left = triggerRect.right - width
        }
        left = Math.max(0, Math.min(left, window.innerWidth - width))
        tooltip.style.top = `${top}px`
        tooltip.style.left = `${left}px`
    }

    render() {
        var classes = "tooltip"
        if (this.opensUp) {
            classes += " up"
        }
        return html`
            <span class="container"
                tabindex="0"
                @mouseenter=${this.show}
                @mouseleave=${this.hide}
                @focusin=${this.show}
                @focusout=${this.hide}>
                <span class="trigger"><slot></slot></span>
                <div class=${classes} popover="manual" role="tooltip">
                    <slot name="content"></slot>
                </div>
            </span>
        `
    }

    static styles = [TwiggCss, css`
        :host {
            display: inline-block;
        }
        .container {
            display: inline-block;
            cursor: help;
        }
        .container:focus-visible {
            outline: var(--fixedSpace0) solid var(--color-primary-pop);
            outline-offset: var(--fixedSpace0);
            border-radius: var(--radius2);
        }
        /* Undo the browser's popover defaults; top/left are set by position() */
        .tooltip {
            position: fixed;
            inset: auto;
            margin: 0;
            overflow: visible;
            width: max-content;
            max-width: 100vw;
            translate: 0 var(--fixedSpace1);
            padding: var(--space2) var(--space5p);
            border-radius: var(--radius1);
            border: var(--border-width) solid var(--twigg-tooltip-border, var(--color-border));
            background-color: var(--color-surface-alt);
            box-shadow: var(--shadow-surface-alt);
            color: var(--color-text);
            text-align: left;
            font-family: var(--font-family);
            line-height: var(--line-height);
        }
        .tooltip.up {
            translate: 0 calc(-1 * var(--fixedSpace1));
        }
    `];
}

customElements.define("twigg-tooltip", TwiggTooltip);
declare global {
    interface HTMLElementTagNameMap {
        'twigg-tooltip': TwiggTooltip;
    }
}
