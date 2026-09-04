import { LitElement, html, css } from 'lit';
import { TwiggCss } from './css';

/**
* A stylized toggle switch Web Component with:
* - three sizes `small`, `medium` (Default), `large`
* - form association (works inside <form> and supports FormData)
* - a custom event when toggled
*
* ## Events
*
* ### `toggle-switch-fired`
* Fired whenever the toggle changes state.
*
* `event.detail = { Checked: boolean }`
*
* Example:
* ```js
* document.querySelector("toggle-switch")
*   .addEventListener("toggle-switch-fired", e => {
*     console.log("toggled:", e.detail.Checked);
*   });
* ```
*
* ## Form Support
*
* This component is **form-associated**, meaning:
* - It participates in `<form>` submission
* - Works with `new FormData(form)`
* - Supports `form.reset()`
*
* The submitted value is:
* - `"on"` when checked
* - omitted (null) when unchecked
*
* Example:
* ```html
* <form id="f">
*   <toggle-switch name="newsletter"></toggle-switch>
*   <button>Submit</button>
* </form>
*
* <script>
* document.querySelector("#f").addEventListener("submit", e => {
*   e.preventDefault();
*   console.log([...new FormData(e.target)]);
* });
* </script>
* ```
*
* ## Usage Example
*
* ```html
* <toggle-switch
*   name="darkMode"
*   size="large"
*   Checked
* ></toggle-switch>
* ```
*/
export class ToggleSwitch extends LitElement {

    // Add this to make <form> integration.
    static formAssociated = true;

    static properties = {
        Checked: { type: Boolean, reflect: true },
        Size: { type: String },
    };
    constructor() {
        super();
        this.Size = 'medium';
        this.internals = this.attachInternals();
    }
    declare Checked: boolean;
    declare Size: 'large' | 'medium' | 'small';
    private internals: ElementInternals;


    private onToggle(e: Event) {
        const input = e.target as HTMLInputElement;
        this.Checked = input.checked;

        this.internals.setFormValue(this.Checked ? "on" : null);

        this.dispatchEvent(new CustomEvent("toggle-switch-fired", {
            detail: { Checked: this.Checked },
            bubbles: true,
            composed: true
        }));
    }


    render() {
        return html`
                <label class="switch">
                    <input type="checkbox" 
                        class="checkbox"
                        .checked=${this.Checked}
                        @change=${this.onToggle}
                    >
                    <div class="slider"></div>
                </label>
            `;
    }

    static styles = [
        TwiggCss,
        css`
            /* MEDIUM Default */
            :host {
                --switch-width: 48px;
                --switch-height: 24px;
                --switch-radius: 16px;
                --switch-border: 3px;
                --knob-translate: 24px;
            }
            /* LARGE */
            :host([size="large"]) {
                --switch-width: 60px;
                --switch-height: 30px;
                --switch-radius: 20px;
                --switch-border: 4px;
                --knob-translate: 30px;
            }
            /* SMALL */
            :host([size="small"]) {
                --switch-width: 36px;
                --switch-height: 18px;
                --switch-radius: 12px;
                --switch-border: 2px;
                --knob-translate: 18px;
            }

            .checkbox {
                display: none;
            }

            .slider {
                width: var(--switch-width);
                height: var(--switch-height);
                background-color: lightgray;
                border-radius: var(--switch-radius);
                overflow: hidden;
                display: flex;
                align-items: center;
                border: var(--switch-border) solid transparent;
                transition: .3s;
                box-shadow: 0 0 10px 0 rgb(0 0 0 / 25%) inset;
                cursor: pointer;
            }

            .slider::before {
                content: '';
                display: block;
                width: 100%;
                height: 100%;
                background-color: #fff;
                transform: translateX(calc(-1 * var(--knob-translate)));
                border-radius: var(--switch-radius);
                transition: .3s;
                box-shadow: 0 0 10px 3px rgb(0 0 0 / 25%);
            }

            .checkbox:checked ~ .slider::before {
                transform: translateX(var(--knob-translate));
            }

            .checkbox:checked ~ .slider {
                background-color: var(--color-primary-pop);
            }

            .checkbox:active ~ .slider::before {
                transform: translate(0);
            }
        `]
}

customElements.define("toggle-switch", ToggleSwitch);

declare global {
    interface HTMLElementTagNameMap {
        'toggle-switch': ToggleSwitch;
    }
}