import { LitElement, html, css } from 'lit';
import { ThemeStoreSingleton } from './theme-store';

export class ThemeSelector extends LitElement {
    static properties = {
        IsDay: { type: Boolean },
    };
    declare public IsDay: boolean;
    constructor() {
        super();
        ThemeStoreSingleton.Init()
        if (ThemeStoreSingleton.GetTheme() == "light"){
            this.IsDay = true
        }else{
            this.IsDay = false;
        }
    }

    static styles = css`
    :host{
        --circle-radius: var(--space2);
        --circle-diameter: calc(2*var(--circle-radius));
    }

    .slider {
        display: flex;
        align-items: center;      /* vertical centering */
        padding: var(--space1);
        width: calc(2 * var(--circle-diameter));
        cursor: pointer;
        background-color: var(--color-theme-selector-night-bg);
        border-radius: var(--circle-diameter);
        position: relative;
    }
    .slider.night {
        background-color: var(--color-theme-selector-night-bg);
    }
    .slider.day {
        background-color: var(--color-theme-selector-day-bg);
    }
    .circle {
        width: var(--circle-diameter);
        height: var(--circle-diameter);
        border-radius: var(--circle-radius);
        transition: all 0.8s;
        transform: translateX(0);
    }
    .circle.night{
        box-shadow: inset calc(0.5 * var(--circle-radius))
                calc(-0.4 * var(--circle-radius))
                0
                0
                var(--color-theme-selector-circle);
    }
    .circle.day {
        transform: translateX(100%);
        box-shadow:
            inset var(--circle-diameter) calc(-1 * var(--circle-diameter)) 0 0 var(--color-theme-selector-circle),
            0 0 calc(0.8 * var(--circle-radius)) calc(0.2 * var(--circle-radius)) var(--color-theme-selector-circle);
            ;
    }
    `;

    render() {
        return html`
          <div class="slider ${this.getClass()}" @click=${this.toggleDay}>
            <span class="circle ${this.getClass()}"></span>
          </div>
    `;
    }
    private getClass(): string{
        if (this.IsDay){
            return "day"
        }
        return "night"
    }
    private toggleDay(){
        this.IsDay = !this.IsDay
        this.dispatchEvent(new CustomEvent('themeChanged', {
            bubbles: true,
            composed: true
        }));
        if (this.IsDay){
            ThemeStoreSingleton.SetTheme("light")
        }else{
            ThemeStoreSingleton.SetTheme("dark")
        }
    }
}
customElements.define("theme-selector", ThemeSelector);
declare global {
    interface HTMLElementTagNameMap {
        'theme-selector': ThemeSelector;
    }
}