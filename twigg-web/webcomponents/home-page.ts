import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { NewRepoUrl, UrlToRepo, ManageSubscriptionPath, PlansPagePath, UserEducationUrl, UserEducationWelcomeWasShownUrl, GetCsrfHeaders } from './routes';
import { User, UserEducation, Repo, RenderRepo } from './interfaces';
import { GetFeatureFlags } from './feature-flags';

/**
 * Shows the home page that displays user data
 */
export class HomePage extends LitElement {
    static properties = {
        User: { type: Object},
        MyRepos: { type: Array },
        SharedRepos: { type: Array },
        showWelcomeTooltip: { state: true }
    };
    constructor() {
        super();
        this.MyRepos = [];
        this.SharedRepos = [];
        this.showWelcomeTooltip = false;
    }
    declare MyRepos: Repo[];
    declare SharedRepos: Repo[];
    declare User: User;
    declare showWelcomeTooltip: boolean;

    firstUpdated() {
        this.loadUserEducation();
    }

    render() {
        return html`
        <div class="main">
            <div class="crumbs">
                <bread-crumbs id="current-crumb" Name="Home" Link="/home"></bread-crumbs>
            </div>
            <section class="repo-section">
                <header class="repo-header">

                    <h2>Your repositories</h2>
                    
                    <div class="create-btn-wrapper">
                        <a
                        class="create-btn"
                        type="button"
                        href="${this.getNewRepoBtnHref()}"
                        aria-label="Create a new"
                        @click="${this.onCreateBtnClick}">
                            <twigg-icon
                                class="tab-icon"
                                icon="DocumentPlus">
                            </twigg-icon>
                            <span>Create new</span>
                        </a>
                        ${this.renderWelcomeTooltip()}
                    </div>

                </header>
               
                <div class="repo-list">
                    ${this.renderRepoList(this.MyRepos)}
                </div>
            </section>

            <section class="repo-section">
                <header class="repo-header">
                    <h2>Shared repositories</h2>
                </header>
                <div class="repo-list">
                    ${this.renderRepoList(this.SharedRepos)}
                </div>
            </section>
        </div>
      `
    }

    private renderRepoList(repos: Repo[]) {
        if (repos.length == 0) {
            return html`<p class="empty">No repositories yet.</p>`
        }
        return html`${repos.map((r) => RenderRepo(r))}`
    }

    private getNewRepoBtnHref(){
        if (this.User.PaymentPlan == "Free" && this.MyRepos.length >=1){
            return PlansPagePath
        }
        return NewRepoUrl
    }

    private async loadUserEducation() {
        if (!GetFeatureFlags().EnableUserEducation) {
            return;
        }
        try {
            const res = await fetch(UserEducationUrl, { method: "GET" });
            if (!res.ok) {
                console.error("failed to load user education", res.status);
                return;
            }
            const data: UserEducation = await res.json();
            if (!data.WelcomeWasShown) {
                this.showWelcomeTooltip = true;
            }
        } catch (err) {
            console.error("failed to load user education", err);
        }
    }

    private renderWelcomeTooltip() {
        if (this.showWelcomeTooltip) {
            return html`
                <div class="welcome-tooltip twigg-lift" role="tooltip">
                    <span>Welcome! Create your first repository here.</span>
                    <twigg-icon
                        class="close-icon"
                        icon="XMark"
                        @click="${this.dismissWelcomeTooltip}">
                    </twigg-icon>
                </div>
            `
        }
        return html``
    }

    private onCreateBtnClick() {
        if (this.showWelcomeTooltip) {
            this.dismissWelcomeTooltip();
        }
    }

    private dismissWelcomeTooltip() {
        this.showWelcomeTooltip = false;
        this.markWelcomeWasShown();
    }

    private async markWelcomeWasShown() {
        try {
            const res = await fetch(UserEducationWelcomeWasShownUrl, {
                method: "PUT",
                headers: {...GetCsrfHeaders()}
            });
            if (!res.ok) {
                console.error("failed to mark welcome as shown", res.status);
            }
        } catch (err) {
            console.error("failed to mark welcome as shown", err);
        }
    }

    static styles = [
        TwiggCss,
        css`
        .main {
            max-width: var(--size4);
            margin: auto;
        }

        .repo-section {
            margin-bottom: var(--space6);
        }

        .repo-list {
            display: flex;
            flex-direction: column;
            gap: var(--space1);
        }


        .empty {
            text-align: center;
            opacity: 0.8;
        }
        .repo-header{
            display: flex;
            align-items: center;
            justify-content: flex-start;
            gap: var(--space3);
            margin: var(--space1) 0 var(--space3) 0;
            position: relative;
        }

        .create-btn{
            background: var(--color-primary);
            color: var(--color-text-on-primary);
            border-color: transparent;
            padding: var(--space0) var(--space2);
            font-size: var(--space4m);
            line-height: var(--line-height);
            border-radius: var(--radius3);
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            text-decoration: none;
            box-shadow: var(--shadow-surface);
        }

        .create-btn-wrapper{
            position: relative;
        }

        .welcome-tooltip{
            position: absolute;
            top: calc(100% + var(--space2));
            left: 0;
            z-index: 10;
            width: max-content;
            background: var(--color-surface);
            border: 1px solid var(--color-primary);
            border-radius: var(--radius3);
            padding: var(--space1) var(--space2);
            display: flex;
            align-items: center;
            gap: var(--space1);
        }

        .close-icon{
            cursor: pointer;
        }

        .welcome-tooltip::before{
            content: "";
            position: absolute;
            bottom: 100%;
            left: var(--space3);
            border: var(--space1) solid transparent;
            border-bottom-color: var(--color-primary);
        }

        @media (max-width: 760px) {
            .create-btn-wrapper{
                position: static;
            }
            .welcome-tooltip{
                width: auto;
                right: 0;
            }
            .welcome-tooltip::before{
                left: calc(60% - var(--space2));
                border-width: var(--space2);
            }
        }

        .create-btn:hover{
            color: var(--color-text-on-primary);
            transform: translateY(-1px);
            box-shadow: var(--shadow-surface);
            text-decoration: none !important;
        }

        .manage-subscription-btn-wrapper {
            display: flex;
            justify-content: left;
            align-items: center;
            height: 100%;
            margin-top: var(--space3);
        }
        .manage-subscription-btn {
            font-size: var(--space4m);
            background-color: var(--color-surface);
            color: var(--color-primary);
            border: 1px solid var(--color-primary);
            border-radius: var(--radius3);
            padding: var(--space2) var(--space5);
            cursor: pointer;
            transition: background-color 0.3s ease, color 0.3s ease;
        }
        .manage-subscription-btn:hover {
            background-color: var(--color-primary);
            color: var(--color-surface);
        }
    `
    ];
}
customElements.define('home-page', HomePage);
declare global {
    interface HTMLElementTagNameMap {
        'home-page': HomePage;
    }
}