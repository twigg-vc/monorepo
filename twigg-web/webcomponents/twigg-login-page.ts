import { css, html, LitElement } from 'lit';
import { TwiggCss } from './css';
import { CsrfFormName, GetCsrfFormValue, PostLoginUrl, PrivacyPage, StartLoginWithGoogleOAuth, StartLoginWithMicrosoftOAuth, TermsPage, TwiggLogoBlackUrl, TwiggLogoWhiteUrl } from './routes';
import { Theme, ThemeStoreSingleton } from './theme-store';


/**
 * Element that contains the whole login page
 */
class LoginPage extends LitElement {
    static properties = {
        Theme: { type: String },
        WrongLoginInfo: { type: Boolean },
        PasswordIsHidden: { type: Boolean, state: true },
    };
    constructor() {
        super()
        ThemeStoreSingleton.Init()
        this.Theme = ThemeStoreSingleton.GetTheme();
        this.WrongLoginInfo = false;
        this.PasswordIsHidden = true;
        ThemeStoreSingleton.AddObserver(this);
    }
    declare Theme: Theme;
    declare WrongLoginInfo: boolean;
    declare private PasswordIsHidden: boolean;


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
        .login-container {
            background: var(--color-surface);
            border-radius: var(--radius1);
            border: 1px solid var(--color-primary);
            box-shadow: var(--shadow-surface);
            width: 100%;
            max-width: var(--size1);
            padding-top: var(--space4);
            padding-left: var(--space4);
            padding-right: var(--space4);
            padding-bottom: var(--space4);
        }
        .login-container h1 {
            text-align: center;
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
            padding: var(--space2);
            border-radius: var(--fixedSpace2);
        }
        .form-group input:focus {
            outline: none;
            box-shadow:var(--shadow-pop);
            border: 1px solid var(--color-primary-pop);
        }
        .btn {
            width: 100%;
            margin-top: var(--space4);
            padding: var(--space2);
            background: var(--color-surface);
            color: var(--color-text);
            font-size: var(--space4);
            border: 1px solid var(--color-primary)
        }
        .btn:hover {
            background: var(--color-surface-alt);
        }
        .oauth-provider-signin-btn {
            margin-top: var(--space5);
            margin-bottom: var(--space4);
            text-align: center;
            margin-inline: auto; 
            width: max-content;
        }
        .auth-legal {
            margin-top: var(--space4);
            text-align: center;
            font-size: var(--space3);
            color: var(--color-text-muted);
        }
        .auth-legal a {
            color: var(--color-primary-pop);
            text-decoration: none;
        }
        .auth-legal a:hover {
            text-decoration: underline;
        }
        .brand { 
            display:flex; 
            align-items:center; 
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
        }
        .signup{
            padding-left: var(--space2);
        }
        .top-right {
            position: absolute;
            top: var(--fixedSpace3);
            right: var(--fixedSpace4);
        }
        .form-login{
            padding-right: var(--space7);
            padding-left: var(--space7);
        }
        .input-login{
            background-color: var(--color-surface-alt)!important;
            color: var(--color-text-muted)!important;        
        }
    `];
    
    render() {
        return html`
        <div class="page">
            <div class="top-right">
                <theme-selector></theme-selector>
            </div>
            <div class="login-container">
                <div class="brand-stack">
                    <div class="brand">
                        <img .src=${(this.getLogoUrl())} alt="Twigg logo">
                    </div>
                </div>
                <form class="form-login" action="${PostLoginUrl}" method="post">
                <input type="hidden" name="${CsrfFormName}" value="${GetCsrfFormValue()}">
                    <div class="form-group">
                        <label for="email">Email address</label>
                        <input class="input-login"
                        type="email" 
                        id="email" 
                        name="email" 
                        placeholder="your@email.com">
                    </div>
                    <div class="form-group">
                        <label for="password">
                            <div style="position: relative; display: flex;">
                                <twigg-icon 
                                .icon=${this.passwordIcon()} 
                                @click=${this.onPasswordIconClick}>
                                Password</twigg-icon>
                            </div>
                        </label>
                        <input class="input-login"
                        type="password" 
                        id="password" 
                        name="password" 
                        type="${this.passwordInputType()}" 
                        placeholder="password">
                    </div>
                    ${this.wrongLoginMessage()}
                    <button class="btn">Log In</button>
                </form>
                <div class="oauth-provider-signin-btn">
                    <google-signin-btn
                        LogInWithGoogleUrl="${StartLoginWithGoogleOAuth}"
                        BtnWidth=320>
                    </google-signin-btn>
                </div>
                <div class="oauth-provider-signin-btn">
                    <microsoft-signin-btn
                        LogInWithMicrosoftUrl="${StartLoginWithMicrosoftOAuth}"
                        BtnWidth=320>
                    </microsoft-signin-btn>
                </div>
                <div class="auth-legal">                   
                    <p>By continuing, you agree to Twigg's</p>
                    <a href="${TermsPage}">Terms of Service</a> and 
                    <a href="${PrivacyPage}">Privacy Policy</a>.
                </div>
            </div>
        </div>
      `
    }

    private onPasswordIconClick() {
        this.PasswordIsHidden = !this.PasswordIsHidden
    }
    private passwordIcon() {
        if (this.PasswordIsHidden) {
            return "EyeSlashIcon"
        }
        return "EyeIcon"
    }
    private passwordInputType() {
        if (this.PasswordIsHidden) {
            return "password"
        }
        return "text"
    }
    private wrongLoginMessage() {
        if (!this.WrongLoginInfo) {
            return html``
        }
        return html`
        <p class="warn-msg">Invalid email or password. Please try again.</p>
        `
    }
    public OnThemeChanged(oldTheme: Theme, newTheme: Theme){
        this.Theme = newTheme
    }
    private getLogoUrl() {
        if (this.Theme == "dark") return TwiggLogoWhiteUrl;
        return TwiggLogoBlackUrl;
    }

}
customElements.define('twigg-login-page', LoginPage);

declare global {
    interface HTMLElementTagNameMap {
        'twigg-login-page': LoginPage;
    }
}