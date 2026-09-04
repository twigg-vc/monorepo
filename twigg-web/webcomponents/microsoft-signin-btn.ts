import { LitElement, html, css } from 'lit';

export class MicrosoftSigninBtn extends LitElement {
    static properties = {
        LogInWithMicrosoftUrl: { type: String }, // OAuth URL
        // (Optional) Width in px from 200 to 400.
        BtnWidth: { type: Number }
    };
    constructor() {
        super();
        this.LogInWithMicrosoftUrl = '';
        this.BtnWidth = -1
    }
    declare private LogInWithMicrosoftUrl: string;
    declare private BtnWidth: number;



    static styles = css`
    .gsi-material-button {
      -moz-user-select: none;
      -webkit-user-select: none;
      -ms-user-select: none;
      -webkit-appearance: none;
      background-color: WHITE;
      background-image: none;
      border: 1px solid #747775;
      border-radius: 20px;
      box-sizing: border-box;
      color: #1f1f1f;
      cursor: pointer;
      font-family: 'Roboto', arial, sans-serif;
      font-size: 14px;
      height: 40px;
      letter-spacing: 0.25px;
      outline: none;
      overflow: hidden;
      padding: 0 12px;
      position: relative;
      text-align: center;
      transition: background-color .218s, border-color .218s, box-shadow .218s;
      vertical-align: middle;
      white-space: nowrap;
      width: auto;
      max-width: 400px;
      min-width: min-content;
    }

    .gsi-material-button .gsi-material-button-icon {
      height: 20px;
      min-width: 20px;
      width: 20px;
    }

    .gsi-material-button .gsi-material-button-content-wrapper {
      align-items: center;
      display: flex;
      flex-direction: row;
      flex-wrap: nowrap;
      height: 100%;
      justify-content: space-between;
      position: relative;
      width: 100%;
    }

    .gsi-material-button .gsi-material-button-contents {
      flex-grow: 1;
      font-family: 'Roboto', arial, sans-serif;
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      vertical-align: top;
    }

    .gsi-material-button .gsi-material-button-state {
      transition: opacity .218s;
      bottom: 0;
      left: 0;
      opacity: 0;
      position: absolute;
      right: 0;
      top: 0;
    }

    .gsi-material-button:disabled {
      cursor: default;
      background-color: #ffffff61;
      border-color: #1f1f1f1f;
    }

    .gsi-material-button:disabled .gsi-material-button-contents,
    .gsi-material-button:disabled .gsi-material-button-icon {
      opacity: 38%;
    }

    .gsi-material-button:not(:disabled):active .gsi-material-button-state,
    .gsi-material-button:not(:disabled):focus .gsi-material-button-state {
      background-color: #303030;
      opacity: 12%;
    }

    .gsi-material-button:not(:disabled):hover {
      box-shadow: 0 1px 2px 0 rgba(60, 64, 67, .30), 
                  0 1px 3px 1px rgba(60, 64, 67, .15);
    }

    .gsi-material-button:not(:disabled):hover .gsi-material-button-state {
      background-color: #303030;
      opacity: 8%;
    }
  `;

    render() {
        if (this.BtnWidth !== -1 && (this.BtnWidth < 200 || this.BtnWidth > 400)) {
            console.warn("invalid width: ", this.BtnWidth)
            this.BtnWidth = -1
        }
        return html`
      <button class="gsi-material-button" 
        @click=${this.onClick} 
        style="${this.BtnWidth === -1 ? `` : `width:${this.BtnWidth}px`}">
        <div class="gsi-material-button-state"></div>
        <div class="gsi-material-button-content-wrapper">
            <div class="gsi-material-button-icon">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="24" height="24"  style="opacity:1;"><path fill="#F1511B" d="M121.666 121.666H0V0h121.666z"/><path fill="#80CC28" d="M256 121.666H134.335V0H256z"/><path fill="#00ADEF" d="M121.663 256.002H0V134.336h121.663z"/><path fill="#FBBC09" d="M256 256.002H134.335V134.336H256z"/></svg>
            </div>
            <span class="gsi-material-button-contents">Continue with Microsoft</span>
            <span style="display: none;">Continue with microsoft</span>
        </div>
      </button>
    `;
    }

    private onClick() {
        if (this.LogInWithMicrosoftUrl) {
            window.location.href = this.LogInWithMicrosoftUrl;
        }
    }
}

customElements.define('microsoft-signin-btn', MicrosoftSigninBtn);

declare global {
    interface HTMLElementTagNameMap {
        'microsoft-signin-btn': MicrosoftSigninBtn;
    }
}