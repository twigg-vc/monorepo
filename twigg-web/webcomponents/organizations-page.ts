import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import type { User } from './interfaces';
import { NewOrganizationPattern, OrganizationsPattern, PathToOrganization } from './routes';

export class OrganizationsPage extends LitElement {
    static properties = {
        Orgs: { type: Array },
    };

    constructor() {
        super();
        this.Orgs = [];
    }

    declare Orgs: User[];

    private goToOrg(org: User) {
        window.location.href = PathToOrganization(org.Username);
    }
    private goToCreateOrg() {
        window.location.href = NewOrganizationPattern;
    }

    render() {
        return html`
            <div class="main">
                <div class="crumbs">
                    <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                    <bread-crumbs-space></bread-crumbs-space>
                    <bread-crumbs id="current-crumb" Name="Organizations" Link="${OrganizationsPattern}"></bread-crumbs>
                </div>

                <div class="header">
                    <h1>Organizations</h1>
                    <button class="btn primary" @click=${this.goToCreateOrg}>
                        New Organization
                    </button>
                </div>

                <div class="twigg-card">
                    ${this.Orgs.length === 0 ? 
                        html`<p class="empty">You don't have permission to any organization yet.</p>`
                        : html`
                            <div class="list">
                                ${this.Orgs.map((org) => html`
                                    <div class="row" @click=${() => this.goToOrg(org)}>
                                        <div class="info">
                                            <username-tag username=${org.Username}></username-tag>
                                        </div>

                                        <div class="actions">
                                            <twigg-icon icon="ChevronRight"></twigg-icon>
                                        </div>
                                    </div>
                                `)}
                            </div>
                        `
                    }
                </div>
            </div>
        `;
    }

    static styles = [
        TwiggCss,
        css`
            .main {
                max-width: var(--size4);
                margin: 0 auto;
            }

            .header {
                display: flex;
                justify-content: space-between;
                align-items: center;
                margin-bottom: var(--space4);
            }

            .list {
                border-top: 1px solid var(--color-border);
            }

            .row {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: var(--space3) 0;
                border-bottom: 1px solid var(--color-border);
                cursor: pointer;
                transition: background 0.2s ease;
            }

            .row:hover {
                background: var(--color-surface-alt);
            }

            .info {
                display: flex;
                align-items: center;
                gap: var(--space3);
            }

            .actions {
                display: flex;
                align-items: center;
            }

            .btn {
                cursor: pointer;
                border-radius: var(--radius1);
                border: 1px solid var(--color-border);
                background: var(--color-surface);
                padding: 0 var(--space3);
                height: var(--space5p);
            }

            .btn.primary {
                background: var(--color-primary);
                color: var(--color-surface);
                border: 1px solid var(--color-primary);
            }

            .empty {
                padding: var(--space4);
                text-align: center;
            }
        `,
    ];
}

customElements.define('organizations-page', OrganizationsPage);
declare global {
    interface HTMLElementTagNameMap {
        'organizations-page': OrganizationsPage;
    }
}