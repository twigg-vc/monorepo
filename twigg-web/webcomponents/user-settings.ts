import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { GetPlanSeats, GetSubscriptionDescription, User } from './interfaces';
import { CLIKeyPramName, DeleteCLIKey, GenerateCLIKey, ManageSubscriptionPath, GetCsrfHeaders } from './routes';

export interface UserSettingsUser {
    Email: string
    Username?: string
}

/**
 * Shows page that displays user settings
 */
export class UserSettings extends LitElement {
    static properties = {
        User: { type: Object }, 
        Keys: { type: Array },
        GeneratedKey: { type: Boolean }, // indicates if a key was generated 
        Loading: { type: Boolean },

        MaxTrackJobs: { type: Number },
        MaxTrackMilliseconds: { type: Number },

        // Animation only purposes
        NewlyAddedKey: { type: String },
        RemovingKey: { type: String },
    };
    constructor() {
        super();
        this.Keys = [];
        this.GeneratedKey =false;
        this.Loading = false;
        this.NewlyAddedKey=""
        this.RemovingKey=""
    }
    declare private User: User;
    declare private Keys: String[];
    declare private GeneratedKey: boolean;
    declare private Loading: boolean;
    declare private MaxTrackJobs: number;
    declare private MaxTrackMilliseconds: number;
    declare private NewlyAddedKey: string;
    declare private RemovingKey: string;


    firstUpdated(){
        if (this.User.HasOldCliKey){
            this.Keys = ["Old Key"]
        }
    }
    
    private async generateKey() {
        this.GeneratedKey = true;
        this.Loading = true;
        try {
            const res = await fetch(
                GenerateCLIKey,
                {
                    method: 'POST',
                    headers: GetCsrfHeaders(),
                },
            );
            if (!res.ok) throw new Error(`Request failed: ${res.status}`);

            const key = await res.text();
            this.NewlyAddedKey = key;
            this.Keys = [key, ...this.Keys];

            // Clear the "new" mark after animation duration (400ms)
            setTimeout(() => {
                this.NewlyAddedKey = '';
            }, 400);
        } catch (err) {
            console.error('Failed to generate CLI key:', err);
        } finally {
            this.Loading = false;
        }
    }
    private async deleteKey(key: string) {
        this.RemovingKey = key;
        try {
            const url = `${DeleteCLIKey}?${CLIKeyPramName}=${encodeURIComponent(key)}`;
            const res = await fetch(url, { method: 'DELETE', headers: GetCsrfHeaders() });
            if (!res.ok) throw new Error(`Failed: ${res.status}`);

            // Wait for animation end before removing from list
            const el = this.renderRoot.querySelector(`.key-card.removing`);
            if (el) {
                // wait for animation to end 
                el.addEventListener('animationend', () => {
                    // Remove from this.Keys
                    this.Keys = this.Keys.filter(k => k !== key);
                    this.RemovingKey = '';
                    if (this.Keys.length==0){
                        this.GeneratedKey = false
                    }
                }, { once: true });
            } else {
                // fallback
                this.Keys = this.Keys.filter(k => k !== key);
                this.RemovingKey = '';
                if (this.Keys.length==0){
                    this.GeneratedKey = false
                }
            }
        } catch (err) {
            console.error(err);
            this.RemovingKey = '';
        }
    }
   
    private onManageSubscriptionClick() {
        window.location.href = ManageSubscriptionPath(this.User);
    }

    private isGenerateCliKeyBtnDisable(){
        let maxNumberOfKeys = 1
        return this.Keys.length  >= maxNumberOfKeys || this.Loading
    }
    render() {
        return html`
        <div class="main">
            <div class="crumbs">
                <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs id="current-crumb" Name="User settings" Link="/user-settings"></bread-crumbs>
            </div>
            <h1>User Settings</h1>

            <div class="twigg-card">
                <h2 class="section-title">Subscription</h2>

                <div class="field">
                    <label>Email</label>
                    <p class="field-value">${this.User?.Email ?? '-'}</p>
                </div>

                <div class="field">
                    <label>Username</label>
                    <p class="field-value">${this.User?.Username ?? '-'}</p>
                </div>

                <div class="field">
                    <label>Plan</label>
                    <p class="field-value">${GetSubscriptionDescription(this.User, this.MaxTrackJobs, this.MaxTrackMilliseconds)}</p>
                </div>
                <button class="manage-subscription-btn" @click=${this.onManageSubscriptionClick}>
                    Manage Subscription <twigg-icon icon="ContractEdit"></twigg-icon>
                </button>
            </div>
            <div class="twigg-card">
                <h2 class="section-title">Cli Keys</h2>

                <div class="generates-key-message ${this.GeneratedKey ? 'visible' : ''}">
                    <twigg-icon class="tab-icon" icon="Key"></twigg-icon>
                    New key created! Copy it now. It won't be shown again.
                </div>
                
                <button class="generate-btn"
                        @click=${this.generateKey}
                        ?disabled=${this.isGenerateCliKeyBtnDisable()}>
                    ${this.Loading ? 'Generating…' : 'Generate New Key'}
                    <twigg-icon class="tab-icon" icon="Key"></twigg-icon>
                </button>

                <div class="keys-list">
                    ${(this.Keys ?? []).map(
                        (key) => html`
                            <div class="key-card 
                                ${this.NewlyAddedKey === key ? 'new' : ''}
                                ${this.RemovingKey === key ? 'removing' : ''}">
                                <p class="key-value">${key}</p>
                                ${key !== 'Old Key' ? html`
                                <button class="copy-btn"
                                @click=${() => navigator.clipboard.writeText(key as string)}
                                title="Copy key">
                                    <twigg-icon class="tab-icon" icon="ContentCopy"></twigg-icon>
                                </button>
                                ` : null}
                                <button class="delete-btn"
                                    @click=${() => this.deleteKey(key as string)}
                                    title="Delete key">
                                    <twigg-icon icon="Delete"></twigg-icon>
                                </button>
                            </div>
                        `
                    )}
                </div>
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

            .page-title {
                font-size: var(--space5p);
                font-weight: var(--weight-bold);
                text-align: center;
            }

            .field label {
                display: block;
                font-weight: var(--weight-semi-bold);
                font-size: var(--space4m);
            }


            .field-value {
                cursor: not-allowed;
                font-size: 1rem;
                color: var(--color-text);
                background: var(--color-surface-alt);
                padding: var(--space3) var(--space4);
                border-radius: var(--radius1);
                box-shadow: inset 0 0 0 1px var(--color-border);
            }

            .section-title {
                font-size: var(--space5);
                font-weight: var(--weight-semi-bold);
            }

            .keys-list {
                display: flex;
                flex-direction: column;
                gap: var(--space4);
            }

            .key-card {
                background: var(--color-primary);
                padding: var(--space4);
                border-radius: var(--radius2);
                box-shadow: var(--shadow-surface);
                display: flex;
                align-items: center;
            }

            @keyframes fadeFromTop {
                0% { opacity: 0; transform: translateY(-20px); }
                100% { opacity: 1; transform: translateY(0); }
            }

            @keyframes fadeOutUp {
                0% { opacity: 1; transform: translateY(0); }
                100% { opacity: 0; transform: translateY(-20px); }
            }

            .key-card.removing {
                animation: fadeOutUp 0.4s ease forwards;
            }

            /* Only new key animates */
            .key-card.new {
            animation: fadeFromTop 0.4s ease forwards;
            }

            .key-value {
                width: 100%;
                color: var(--color-text-on-primary);
                overflow-wrap: anywhere;
            }
            .copy-btn {
                background: var(--color-surface);
                color: var(--color-primary);
                border: none;
                box-shadow: var(--shadow-surface-alt);
                margin-right: var(--space3);
                transition: background 0.2s, transform 0.1s;
            }

            .copy-btn:hover {
                background: var(--color-primary-pop);
                color: var(--color-text-on-primary);
                transform: translateY(-1px);
            }

            .copy-btn:active {
                transform: translateY(1px);
            }

            .manage-subscription-btn {
                margin-top: var(--space5);
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
            
            .generate-btn {
                padding: var(--space2) var(--space5);
                border-radius: var(--radius2);
                border: 1px solid var(--color-border);
                background: var(--color-primary);
                color: var(--color-text-on-primary);
                cursor: pointer;
                transition: background 0.3s ease, transform 0.15s ease;
                font-size: var(--space4m)
            }
            .generate-btn:disabled {
                background: var(--color-border);
                cursor: not-allowed;
            }

            .delete-btn {
                background: var(--color-surface);
                color: var(--color-primary);
                border: none;
                box-shadow: var(--shadow-surface-alt);
                transition: background 0.2s, transform 0.1s;
            }
            .delete-btn:hover {
                background: var(--color-primary-pop);
                color: var(--color-text-on-primary);
                transform: translateY(-1px);
            }

            .generates-key-message {
                opacity: 0;
                display: none;
                align-items: center;
                justify-content: center;
                background: var(--color-soft-warning);
                color: var(--color-status-text);
                font-weight: var(--weight-semi-bold);
                padding: var(--space2) var(--space4);
                border-radius: var(--radius1);
                box-shadow: var(--shadow-surface);
                margin-bottom: var(--space4);
            }
            .generates-key-message.visible {
                opacity: 1;
                display: flex
            }
            .add {
                display: flex;
                align-items:center;
                gap: var(--space2);
                margin-bottom: var(--space3);
            }
            .input, .select, .btn {
                height: var(--space6m);
                border-radius: var(--radius1);
                border: 1px solid var(--color-border);
                background: var(--color-surface);
                color: var(--color-text);
                padding: 0 var(--space3);
                font-weight: var(--weight-semi-bold);
            }
            .input:focus, .select:focus, .btn:focus {
                outline: 1px solid var(--color-primary);
                outline-offset: 1px;
            }
            .btn { cursor: pointer; }
            .btn-primary {
                background: var(--color-primary);
                border-color: var(--color-primary);
                color: var(--color-text-on-primary);
            }
            .list { border-top: 1px solid var(--color-border); }
            .row {
                display: flex;
                justify-content: space-between;
                gap: var(--space2);
                padding: var(--space3) 0;
                border-bottom: 1px solid var(--color-border);
            }

            .user { display:flex; align-items:center; gap: var(--space2); }

            .actions { 
                text-align: right; 
                display:flex; 
                gap: var(--space2);  
                align-items:center;
            }
            .actions .select { height: var(--space5p); padding: 0 var(--space2); }
            .actions .btn { height: var(--space5p); padding: 0 var(--space2); }

            .permissions{
                display:flex; 
                align-items:center;
                height: var(--space5p);
                border-radius: var(--radius1);
                border: 1px solid var(--color-border);
                background: var(--color-surface-alt);
                color: var(--color-text);
                padding: 0 var(--space3);
                font-weight: var(--weight-semi-bold);
                cursor: not-allowed;
            }
            .guest.twigg-card{
                margin-top: 0;
                box-shadow: none;
            }
            .email {
                padding: 0 var(--space2);
                color: var(--color-text);
                background:var(--color-surface-alt);
                border: 1px solid var(--color-border);
                border-radius: var(--radius1);
            }
        `,
    ];
}
customElements.define('user-settings', UserSettings);
declare global {
    interface HTMLElementTagNameMap {
        'user-settings': UserSettings;
    }
}