import { css, html, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Theme, ThemeStoreSingleton } from './theme-store';
import { CsrfFormName, GetCsrfFormValue, GetCsrfHeaders, SetUsernameParamName, TwiggLogoBlackUrl, TwiggLogoWhiteUrl } from './routes';
import { SetUsernameUrl } from './routes';
import { UsernameIsValid } from './interfaces';

/**
 * Element for setting a username
 */
class SetUsernamePage extends LitElement {
    static properties = {
        Theme: { type: String },
        UsernameErrorMsg: { type: String },
    };

    constructor() {
        super();
        ThemeStoreSingleton.Init();
        this.Theme = ThemeStoreSingleton.GetTheme();
        this.UsernameErrorMsg = "";
        ThemeStoreSingleton.AddObserver(this);
    }

    declare Theme: Theme;
    declare UsernameErrorMsg: string;

    static styles = [
        TwiggCss,
        css`
        .page{
            flex: 1;
            height: 90vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: var(--color-bg);
        }
        .container {
            background: var(--color-surface);
            border-radius: var(--radius1);
            border-color: var(--color-border);
            box-shadow: var(--shadow-surface);
            width: 100%;
            max-width: var(--size1);
            padding: var(--space4);
            transition: transform .2s, box-shadow .2s, border-color .2s;
        }
        .container:hover {
            transform: translateY(-2px);
            border: 1px solid var(--color-primary);
            box-shadow: var(--shadow-pop);
        }
        .container h1 {
            text-align: center;
            margin-bottom: var(--space4);
        }
        .form-group label {
            display: block;
            font-size: var(--space4);
            margin-bottom: var(--space1);
            margin-top: var(--space2);
        }
        .form-group input {
            width: 100%;
            justify-content: center;
            padding: var(--space3);
            border-radius: var(--fixedSpace2);
        }
        .form-group input:focus {
            outline: none;
            box-shadow: var(--shadow-pop);
            border: 1px solid var(--color-primary-pop);
        }
        .btn {
            width: 100%;
            margin-top: var(--space4);
            padding: var(--space3);
            background: var(--color-primary);
            color: var(--color-text);
            font-size: var(--space5);
        }
        .btn:hover {
            background: var(--color-primary-pop);
        }
        .brand { 
            display:flex; 
            align-items:center; 
            justify-content:center;
        }
        .brand img {
            height: var(--fixedSpace7);
            width: auto;
        }
        .brand-stack {
            display: flex;
            flex-direction: column;
            align-items: center;
        }
        .warn-msg {
            font-size: var(--space3);
            color: var(--color-danger);
            margin-top: var(--space1);
            text-align: center;
        }
        .top-right {
            position: absolute;
            top: var(--fixedSpace3);
            right: var(--fixedSpace4);
        }
    `];


    async onSubmit(e: Event) {
        e.preventDefault()
        const form = e.target as HTMLFormElement
        const data = new FormData(form)
    
        const username = (data.get(SetUsernameParamName) as string) || ""

        const { isValid, errorMsg } = UsernameIsValid(username)
        if (!isValid){
            this.UsernameErrorMsg = errorMsg 
            return
        }

        
        try {
            const resp = await fetch(SetUsernameUrl, {
                method: "POST",
                body: data,
            });
            if (resp.redirected) {
                window.location.href = resp.url
            }
        } catch (error) {
            console.log("error setting username", error)
        }
    }

    render() {
        return html`
        <div class="page">
            <div class="top-right">
                <theme-selector></theme-selector>
            </div>
            <div class="container">
                <div class="brand-stack">
                    <div class="brand">
                        <img .src=${this.getLogoUrl()} alt="Twigg logo">
                    </div>
                    <h1>Choose a Username</h1>
                </div>
                <form @submit=${this.onSubmit}>
                    <input type="hidden" name="${CsrfFormName}" value="${GetCsrfFormValue()}">
                    <div class="form-group">
                        <label for="username">Username</label>
                        <input type="text" id="username"
                            style="text-transform: lowercase;"
                            name="${SetUsernameParamName}"
                            @input=${(e: Event) => {
                                const input = e.target as HTMLInputElement
                                input.value = input.value.toLowerCase()
                            }}
                            placeholder="e.g. johndoe">
                    </div>
                    ${this.usernameWarning()}
                    <button class="btn">Save Username</button>
                </form>
            </div>
        </div>
        `;
    }

    private usernameWarning() {
        if (!this.UsernameErrorMsg) return html``;
        return html`<p class="warn-msg">${this.UsernameErrorMsg}</p>`;
    }

    public OnThemeChanged(oldTheme: Theme, newTheme: Theme) {
        this.Theme = newTheme
    }
    private getLogoUrl() {
        if (this.Theme == "dark") return TwiggLogoWhiteUrl;
        return TwiggLogoBlackUrl;
    }
}

customElements.define('set-username-page', SetUsernamePage);

declare global {
    interface HTMLElementTagNameMap {
        'set-username-page': SetUsernamePage;
    }
}