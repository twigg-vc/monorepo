import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { GetSubscriptionDescription, OrgPermissionMember, OrgPermissionOwner, RenderRepo, Repo, User, UsernameIsValid } from './interfaces';
import { GetCsrfHeaders, PathToCreateRepoForOrg, PathToGrantOwnerOrMemberPermToUserPattern, PathToManageOrgSubscriptionWithStripe, PathToRevokeOwnerOrMemberPermToUserPattern, PermissionParamName, PlansPagePath, UrlToRepo } from './routes';



export class OrganizationPage extends LitElement {
    static properties = {
        Org: { type: Object },
        OrgMaxTrackJobs: { type: Number },
        OrgMaxTrackMilliseconds: { type: Number },
        UsersWithOwnerPermission: { type: Array },
        UsersWithMemberPermission: { type: Array },
        CurrentUserIsOrgOwner: { type: Boolean },
        OrgRepos: { type: Array },
        inviteUserIsLoading: { type: Boolean, state: true },
    };

    constructor() {
        super();

        this.UsersWithOwnerPermission = [];
        this.UsersWithMemberPermission = [];
        this.CurrentUserIsOrgOwner = false;
        this.OrgRepos = [];
        this.inviteUserIsLoading = false;
    }

    declare Org: User;
    declare OrgMaxTrackJobs: number;
    declare OrgMaxTrackMilliseconds: number;
    declare UsersWithOwnerPermission: User[];
    declare UsersWithMemberPermission: User[];
    declare CurrentUserIsOrgOwner: boolean;
    declare OrgRepos: Repo[];
    declare private inviteUserIsLoading: boolean;

    private goToManageOrgSubscriptionWithStripe() {
        window.location.href = PathToManageOrgSubscriptionWithStripe(this.Org);
    }

    private goToCreateRepoForOrg() {
        if (this.Org.PaymentPlan == "Free") {
            this.goToManageOrgSubscriptionWithStripe()
            return;
        }
        window.location.href = PathToCreateRepoForOrg(this.Org);
    }
    
    private async invite(ev: Event) {
        ev.stopPropagation();

        if (this.inviteUserIsLoading) return;
        this.inviteUserIsLoading = true;

        const input = this.renderRoot.querySelector<HTMLInputElement>('.input');
        const select = this.renderRoot.querySelector<HTMLSelectElement>('.select');

        if (!input || !select) {
            this.inviteUserIsLoading = false;
            return;
        }

        const username = input.value.trim();
        const permission = select.value;
        const { isValid, errorMsg } = UsernameIsValid(username)
        if (!isValid) {
            alert(errorMsg);
            this.inviteUserIsLoading = false;
            return;
        }

        const formData = new FormData();
        formData.append("username", username);
        formData.append(PermissionParamName, permission);

        const url = PathToGrantOwnerOrMemberPermToUserPattern(this.Org.Username);
        try {
            const resp = await fetch(url, {
                method: "POST",
                body: formData,
                headers: GetCsrfHeaders(),
            });
            const text = (await resp.text()).trim()
            // Copies backend's NoSeatsLeftErrMsg
            if (text === "organization has no available seats"){
                if (confirm("Organization has no seats left. Do you want to manage the subscription?")){
                    this.goToManageOrgSubscriptionWithStripe()
                    return;
                }
                return;
            }
            // Copies backend's PermissionAlreadyExitsErrMsg
            if (text === "permission already exists") {
                alert("User already has this permission");
                return;
            }
            // Handle unhandled errors here
            if (!resp.ok) {
                alert(text)
                return;
            }

            const newUser = { Username: username } as User;

            if (permission === "3") {
                this.UsersWithOwnerPermission = [
                    ...this.UsersWithOwnerPermission,
                    newUser,
                ];
            } else {
                this.UsersWithMemberPermission = [
                    ...this.UsersWithMemberPermission,
                    newUser,
                ];
            }

            input.value = "";

        } catch (err) {
            console.log("error posting invite:", err);
            alert("Failed to invite user");
        } finally {
            this.inviteUserIsLoading = false;
        }
    }

    private async revoke(username: string, permission: string) {
        if (!confirm(`Remove ${username} from this organization?`)) return;

        const formData = new FormData();
        formData.append("username", username);
        formData.append(PermissionParamName, permission);

        const url = PathToRevokeOwnerOrMemberPermToUserPattern(this.Org.Username);
        try {
            const resp = await fetch(url, {
                method: "POST",
                body: formData,
                headers: GetCsrfHeaders(),
            });

            if (!resp.ok) {
                const errorMsg = await resp.text();
                alert(errorMsg);
                return;
            }

            if (permission === OrgPermissionOwner) {
                this.UsersWithOwnerPermission = this.UsersWithOwnerPermission.filter(u => u.Username !== username);
            } else {
                this.UsersWithMemberPermission = this.UsersWithMemberPermission.filter(u => u.Username !== username);
            }
        } catch (err) {
            console.log("error revoking permission:", err);
            alert("Failed to remove user");
        }
    }

    render() {
        return html`
            <div class="main">
                <div class="crumbs">
                    <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                    <bread-crumbs-space></bread-crumbs-space>
                    <bread-crumbs Name="Organizations" Link="/orgs"></bread-crumbs>
                    <bread-crumbs-space></bread-crumbs-space>
                    <bread-crumbs id="current-crumb" Name=${this.Org.Username} Link=""></bread-crumbs>
                </div>

                <h1>Organization</h1>

                <div class="twigg-card">
                    <h2 class="section-title">Details</h2>

                    <div class="field">
                        <label>Name</label>

                        <p class="field-value">
                            ${this.Org.Username}
                        </p>
                    </div>

                    <div class="field">
                        <label>Plan</label>
                        <p class="field-value">${GetSubscriptionDescription(this.Org, this.OrgMaxTrackJobs, this.OrgMaxTrackMilliseconds)}</p>
                    </div>
                </div>

                <div class="twigg-card">
                    <h2 class="section-title">Repositories</h2>
                    
                    <button class="create-repo-btn" @click=${this.goToCreateRepoForOrg}>
                        Create Repository <twigg-icon icon="DocumentPlus"></twigg-icon>
                    </button>

                    <div class="repo-list">
                        ${this.OrgRepos.length === 0
                            ? html`<p class="empty">No repositories yet.</p>`
                            : this.OrgRepos.map(r => RenderRepo(r))
                        }
                    </div>
                </div>

                <div class="twigg-card">
                    <h2 class="section-title">Users</h2>

                    <div class="add">
                        <label>
                            Username:
                            <input
                                class="input"
                                placeholder="username"
                            />
                        </label>
                        <select class="select">
                            <option value="4">Member</option>
                            <option value="3">Owner</option>
                        </select>
                        <div>
                            <button ?disabled=${this.inviteUserIsLoading} @click=${this.invite}  class="btn btn-primary">Invite</button>
                        </div>
                    </div>

                    <!-- Owners list -->
                    <div class="list">
                        ${this.UsersWithOwnerPermission.map(userWithPermission => html`
                            <div class="row">
                                <div class="user">
                                    <username-tag
                                        username=${userWithPermission.Username}>
                                    </username-tag>
                                </div>

                                <div class="actions">
                                    <div class="permission">Owner</div>
                                    ${this.CurrentUserIsOrgOwner ?
                                        html`
                                            <button class="btn" @click=${() => this.revoke(userWithPermission.Username, OrgPermissionOwner)}>
                                                Remove
                                            </button>
                                        `
                                        : html``
                                    }
                                </div>
                            </div>
                        `)}
                    </div>

                    <!-- Members list -->
                    <div class="list">
                        ${this.UsersWithMemberPermission.map(userWithPermission => html`
                            <div class="row">
                                <div class="user">
                                    <username-tag
                                        username=${userWithPermission.Username}>
                                    </username-tag>
                                </div>

                                <div class="actions">
                                    <div class="permission">Member</div>
                                    ${this.CurrentUserIsOrgOwner ?
                                        html`
                                            <button class="btn" @click=${() => this.revoke(userWithPermission.Username, OrgPermissionMember)}>
                                                Remove
                                            </button>
                                        `
                                        : html``
                                    }
                                </div>
                            </div>
                        `)}
                    </div>

                    ${this.CurrentUserIsOrgOwner ?
                        html`
                            <button class="manage-subscription-btn" @click=${this.goToManageOrgSubscriptionWithStripe}>
                                Manage Subscription <twigg-icon icon="ContractEdit"></twigg-icon>
                            </button>
                        `
                        : html``
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

            .section-title {
                font-size: var(--space5);
                font-weight: var(--weight-semi-bold);
            }

            .field {
                margin-top: var(--space4);
            }
            .field label {
                display: block;
                font-weight: var(--weight-semi-bold);
                margin-bottom: var(--space2);
            }
            .field-value {
                cursor: not-allowed;
                font-size: var(--space4);
                color: var(--color-text);
                background: var(--color-surface-alt);
                padding: var(--space3) var(--space4);
                border-radius: var(--radius1);
                box-shadow: inset 0 0 0 1px var(--color-border);
            }

            .list {
                margin-top: var(--space4);
                border-top: 1px solid var(--color-border);
            }
            .row {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: var(--space3) 0;
                border-bottom: 1px solid var(--color-border);
            }

            .actions {
                display: flex;
                align-items: center;
                gap: var(--space2);
            }
            .permission {
                padding: 0 var(--space3);
                height: var(--space5p);
                display: flex;
                align-items: center;
                border-radius: var(--radius1);
                background: var(--color-surface-alt);
                border: 1px solid var(--color-border);
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


            .repo-list {
                display: flex;
                flex-direction: column;
                gap: var(--space1);
                margin-top: var(--space4);
            }
            .empty {
                opacity: 0.8;
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
        `,
    ];
}

customElements.define('organization-page', OrganizationPage);
declare global {
    interface HTMLElementTagNameMap {
        'organization-page': OrganizationPage;
    }
}