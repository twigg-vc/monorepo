import { LitElement, html, css } from 'lit';
import { TwiggCss } from './css';
import { GetCsrfHeaders, HomeUrl, PathToAddRepoPermission, PathToArchiveRepo, 
    PathToRemoveRepoPermission, PathToRepoSettings, UrlToRepo, PathToSetGitMirrorUrl, 
    PathToSetGitMirrorEnabled,
    PathToSetRepoDescription,
    RepoDescriptionParamName,
    GitMirrorEnabledParamName,
    RepoSecretValueParamName,
    RepoSecretNameParamName,
    PathToSetRepoSecret,
    UrlToDeleteRepoSecret,
    PathToSetRepoPublic,
    PathToSetRepoPrivate,
    GitMirrorUrlSecretName} from './routes';
import { GetFeatureFlags } from './feature-flags';
import { Secret } from './interfaces';

type Role = 'Read/Write'  | "Owner";
type Member = { Username: string; Role: Role };

export class RepoSettings extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        Description: { type: String },
        IsGitMirrorEnabled: { type: Boolean },
        GitMirrorUrl: { type: String },
        IsPublic: { type: Boolean },
        isLoadingVisibility: { type: Boolean, state: true },
        Members: { type: Array },
        Secrets: { type: Array },
        showArchiveModal: { type: Boolean, state: true },
        inputValue: { type: String, state: true },
        inputGitUrl: { type: String, state: true },
        inputDescription: { type: String, state: true },
        isLoading: { type: Boolean, state: true },
        isLoadingSaveGitUrl: { type: Boolean, state: true },
        isLoadingSaveDescription: { type: Boolean, state: true },
        isLoadingArchive: { type: Boolean, state: true },
        showCreateSecretModal: { type: Boolean, state: true },
        isLoadingCreateSecretBtn: { type: Boolean, state: true },
        secretsWithDeleteBtnLoading: { type: Array, state: true },
    };

    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Description: string;
    declare IsGitMirrorEnabled: boolean;
    declare GitMirrorUrl: string;
    declare private isLoadingVisibility: boolean;
    declare IsPublic: boolean;
    declare Members: Member[];
    declare Secrets: Secret[];
    declare private showArchiveModal: boolean;
    declare private inputValue: string;
    declare private inputGitUrl: string;
    declare private inputDescription: string;
    declare private isLoading: boolean;
    declare private isLoadingSaveGitUrl: boolean;
    declare private isLoadingSaveDescription: boolean;
    declare private isLoadingArchive: boolean;
    declare private showCreateSecretModal: boolean;
    declare private isLoadingCreateSecretBtn: boolean;
    declare private secretsWithDeleteBtnLoading: string[];


    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.Description = "";
        this.IsGitMirrorEnabled = false;
        this.GitMirrorUrl = "";
        this.IsPublic = false;
        this.isLoadingVisibility = false;
        this.Members = [];
        this.Secrets = [];
        this.showArchiveModal = false;
        this.inputValue = "";
        this.inputGitUrl = "";
        this.inputDescription = "";
        this.isLoading = false
        this.isLoadingSaveGitUrl = false
        this.isLoadingSaveDescription = false;
        this.isLoadingArchive = false;
        this.showCreateSecretModal = false;
        this.isLoadingCreateSecretBtn = false;
        this.secretsWithDeleteBtnLoading = [];
    }

    openModal() {
        this.showArchiveModal = true;
    }

    closeModal() {
        this.showArchiveModal = false;
    }
    openCreateSecretModal() {
        this.showCreateSecretModal = true;
    }

    closeCreateSecretModal() {
        this.showCreateSecretModal = false;
    }

    render() {
        return html`
        <div class="main">
            <div class="crumbs">
                <bread-crumbs Name="Home" Link="/home"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs Name=${this.RepoName} Link="${UrlToRepo(this.RepoOwnerName, this.RepoName)}"></bread-crumbs>
                <bread-crumbs-space></bread-crumbs-space>
                <bread-crumbs id="current-crumb" Name=Settings Link="${PathToRepoSettings(this.RepoOwnerName, this.RepoName)}"></bread-crumbs>
            </div>
            <h1>Repository Settings</h1>
            <div class="twigg-card">
                <h2>Collaborators</h2>
                <!-- Add collaborator -->
                <div class="add">
                    <input
                    class="input"
                    placeholder="username"
                    .value=${this.inputValue}
                    @input=${this.onInputValueChange}
                    @keydown=${this.onInputKeyDown}
                    />
                    <select
                    class="select"
                    >
                    <option>Read/Write</option>
                    </select>
                    <div>
                        ${this.isLoading
                            ? html`<simple-loader></simple-loader>`
                            : html`<button @click=${this.onInviteClick}  class="btn btn-primary">Invite</button>`
                        }
                    </div>
                </div>

                <!-- Current collaborators -->
                <div class="list" role="list">
                    ${this.Members.map((m, i) => html`
                        <div class="row" role="listitem">
                            <div class="user">
                            <div>
                                <username-tag username=${m.Username}></username-tag>
                            </div>
                        </div>

                        
                        <div class="actions">
                            <div class="permissions" title="To change permissions, remove the user and add them again.">
                                ${m.Role}
                            </div>
                            ${m.Role !== 'Owner'
                        ? html`<button @click=${this.getRemoveMemberFunc(i)} class="btn">X</button>`
                            : null}
                        </div>
                    </div>
                    `)}
                </div>
            </div>
            ${this.renderDescriptionCard()}
            ${this.renderGitMirrorCard()}
            ${GetFeatureFlags().RepoSecretsIsEnabled ?
                this.renderRepoSecretsCard() :
                html``
            }
            ${this.renderVisibilityCard()}
            <div id="archive-btn-container">
                ${this.renderArchiveBtn()}
            </div>
            ${this.showArchiveModal
                ? html`
            <div class="modal-backdrop" @click=${this.closeModal}>
              <div class="modal" @click=${(e) => e.stopPropagation()}>
                <h4>Are you sure you want to archive repository:</h4>
                <h3>${this.RepoOwnerName}/${this.RepoName}</h3>
                <p>This action can't be undone.</p>
                
                <div class="modal-buttons">
                    <button @click=${this.closeModal}>Cancel</button>
                    <button class="btn btn-archive" @click=${this.postArchive}>
                      Yes, archive forever!
                    </button>
                </div>
              </div>
            </div>
          `
                : ''}
        </div class="main">        
    `;
    }
    private renderArchiveBtn(){
        if (this.isLoadingArchive){
            return html`<simple-loader></simple-loader>`
        }
        return html`
        <button class="btn btn-archive"  @click=${this.openModal}>
            Archive <twigg-icon icon="Delete">
            </twigg-icon>
        </button>`
    }
    private renderVisibilityCard(){
        if (this.IsPublic) {
            return html`
                <div class="twigg-card">
                    <div class="flex-container">
                        <h2>Visibility</h2>
                        <div class="visibility-tag">Public</div>
                    </div>
                    <p>
                        Anyone can read this repository. 
                        Only collaborators can write to it.
                    </p>
                    <div class="save-btn-container">
                        ${this.renderVisibilityBtn()}
                    </div>
                </div>
            `
        } 
        return html`
            <div class="twigg-card">
                <div class="flex-container">
                    <h2>Visibility</h2>
                    <div class="visibility-tag">Private</div>
                </div>
                <p>Only collaborators can read this repository.</p>
                <div class="save-btn-container">
                    ${this.renderVisibilityBtn()}
                </div>
            </div>
        `
    }
    private renderVisibilityBtn(){
        if (this.isLoadingVisibility) {
            return html`<simple-loader></simple-loader>`;
        }
        if (this.IsPublic) {
            return html`
            <button class="btn btn-save" @click=${this.onMakePrivateClicked}>
                Make private
            </button>`;
        }
        return html`
        <button class="btn btn-primary btn-save" @click=${this.onMakePublicClicked}>
            Make public
        </button>`;
    }
    private async onMakePrivateClicked() {
        if (this.isLoadingVisibility) {
            return;
        }
        this.isLoadingVisibility = true;
        try {
            const resp = await fetch(
                PathToSetRepoPrivate(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            if (resp.ok) {
                this.IsPublic = false;
            } else {
                console.error("request to make repo private failed: ", resp);
                alert("Failed to make repository private");
            }
        } catch (error) {
            console.error("error=", error, " making repository private");
            alert("Failed to make repository private");
        }
        this.isLoadingVisibility = false;
    }
    private async onMakePublicClicked() {
        if (this.isLoadingVisibility) {
            return;
        }
        this.isLoadingVisibility = true;

        if (!confirm(`Are you sure you want to make repository: ${this.RepoName} public?`)) {
            return;
        }
        
        try {
            const resp = await fetch(
                PathToSetRepoPublic(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            if (resp.ok) {
                this.IsPublic = true;
            } else {
                console.error("request to make repo public failed: ", resp);
                alert("Failed to make repository public");
            }
        } catch (error) {
            console.error("error=", error, " making repository public");
            alert("Failed to make repository public");
        }
        this.isLoadingVisibility = false;
    }
    private renderDescriptionCard() {
        var currentDescription = undefined;
        if (this.Description === "") {
            currentDescription = "No description";
        } else {
            currentDescription = this.Description;
        }
        return html`
        <div class="twigg-card">
            <h2>Description</h2>
            <label>
                <p>Current description:</p>
                <div class="current-description">${currentDescription}</div>
                <p>New description:</p>
                <input
                class="input description-input"
                placeholder="A short description of the repository"
                .value=${this.inputDescription}
                @input=${this.onInputDescriptionChange}
                />
            </label>
            <div class="save-btn-container">
                ${this.renderSaveDescriptionBtn()}
            </div>
        </div>
        `
    }
    private renderSaveDescriptionBtn() {
        if (this.isLoadingSaveDescription) {
            return html`<simple-loader></simple-loader>`
        }
        return html`
        <button class="btn btn-primary btn-save" @click=${this.onSaveDescriptionClicked}>
            Save description
        </button>`
    }
    private renderGitMirrorCard(){
        const hasUrlNotSetError = this.IsGitMirrorEnabled && this.GitMirrorUrl === "";

        const hasUrlInvalidError =
            this.inputGitUrl.trim() !== "" &&
            !this.isValidGitUrl(this.inputGitUrl);

        return html`
        <div class="twigg-card" @toggle-switch-fired=${this.toggleGitMirrorFired}>
            <div class="flex-container">
                <h2>Git Mirror</h2>
                <toggle-switch ?Checked=${this.IsGitMirrorEnabled}></toggle-switch>
                ${this.IsGitMirrorEnabled ? `Enabled` : `Disabled`}
            </div>
            <div class=${this.IsGitMirrorEnabled ? `` : `is-disabled`}>
                <p>Submitted commits will be pushed to the twigg branch of the provided git repository.</p>
                <label>
                    <p>Current repository URL:</p>
                    <div class="repo-url">
                    ${(this.GitMirrorUrl ?? '') || 'Not configured'}
                    </div>
                    <p>New URL:</p>
                    <input 
                    class="input git-https-input ${hasUrlNotSetError ? 'input-error' : ''}" 
                    placeholder="https://<token>@my-git-server.com/username/repo-name.git" 
                    .value=${this.inputGitUrl}
                    @input=${this.onInputGitUrlValueChange}
                    />
                    <p class="error-message ${hasUrlNotSetError ? 'visible' : ''}">
                        Repository URL is required when Git Mirror is enabled
                    </p>
                    <div class="error-message ${hasUrlInvalidError ? 'visible' : ''}">
                        Invalid format. Expected examples: 
                        <P>Github: https://&lt;token&gt;@github.com/your-org/your-repo.git</P>
                        <P>GitLab: https://oauth2:&lt;token&gt;@gitlab.com/your-repo/path.git</P>
                        <P>Others: https://&lt;token&gt;@git.mycompany.com/projects/tooling/repo.git</P>
                    </div>
                </label>
                <div class="save-btn-container">
                    <a class="git-mirror-docs-anchor" href="/docs/v/2/git-mirror">
                        Git mirror docs <twigg-icon icon="Doc"></twigg-icon>
                    </a>
                    ${this.isLoadingSaveGitUrl
                        ? html`<simple-loader></simple-loader>`
                        : html`<button class="btn btn-primary btn-save" @click=${this.onSaveGitUrlClicked}>Save URL</button>`
                    }
                </div>
            </div>
        </div>
        `
    }
    private renderRepoSecretsCard() {
        return html`
        <div class="twigg-card">
            <h2>Repository Secrets</h2>

            <button class="btn btn-primary" @click=${this.openCreateSecretModal}>
                Create secret
            </button>

            <div class="list">
                ${this.Secrets.map(
                    (secret) => this.renderSecretRow(secret)
                )}
            </div>
        </div>

        ${this.showCreateSecretModal ? 
        html`
        <div class="modal-backdrop" @click=${this.closeCreateSecretModal}>
            <div class="modal" @click=${(e) => e.stopPropagation()}>
                <h2 class="create-new-secret-modal-title">Create a new Secret</h2>
                <form class="form-of-create-secret-modal" id="create-secret-form-id">
                    <p>Name:</p>
                    <input
                        class="input"
                        name=${RepoSecretNameParamName}
                    />
                    <p>Secret:</p>
                    <textarea class="input" name=${RepoSecretValueParamName}></textarea>
                </form>

                <div class="modal-buttons">
                    <button @click=${this.closeCreateSecretModal}>Cancel</button>
                    <button class="btn btn-primary" 
                        ?disabled=${this.isLoadingCreateSecretBtn} 
                        @click=${this.onCreateSecretClicked}
                    >
                        Create secret
                    </button>
                </div>
            </div>
        </div>`
        : html``}
    `;
    }
    private renderSecretRow(secret: Secret) {
        const isLoading = this.secretsWithDeleteBtnLoading.includes(secret.Name);
        
        if (secret.Name== GitMirrorUrlSecretName){
            return html``
        }
        
        return html`
        <div class="row">
            <div>${secret.Name}</div>
            <button
                    class="btn"
                    ?disabled=${isLoading}
                    @click=${() => this.onDeleteSecretClicked(secret)}
            >
                ${isLoading
                    ? html`<simple-loader></simple-loader>`
                    : html`Delete`
                }
            </button>
        </div>
        `
    }
    private isValidGitUrl(url: string): boolean {
        // Regex:
        // https://<token>@<anything>.git
        const re = /^https:\/\/.+@.+\.git$/
        return re.test(url.trim());
    }

    private normalizeGitUrl(url: string): string {
        return url.replace(/(\.git)\/$/, "$1");
    }
    private onInputGitUrlValueChange(e: Event) {
        const input = e.currentTarget as HTMLInputElement;
        const normalized = this.normalizeGitUrl(input.value);

        // Update state
        this.inputGitUrl = normalized;

        // Ensure DOM stays in sync (prevents cursor glitches in some browsers)
        if (input.value !== normalized) {
            input.value = normalized;
        }
    };
    private onInputDescriptionChange(e: Event) {
        const input = e.currentTarget as HTMLInputElement;
        this.inputDescription = input.value;
    }
    private async onSaveDescriptionClicked() {
        const maxDescriptionLength = 100;
        if (this.inputDescription.length > maxDescriptionLength) {
            alert("Description: must be 100 characters or less");
            return;
        }
        this.isLoadingSaveDescription = true;

        const formData = new FormData();
        formData.append(RepoDescriptionParamName, this.inputDescription);
        try {
            const resp = await fetch(PathToSetRepoDescription(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                alert("Failed to save the description");
                this.isLoadingSaveDescription = false;
                return;
            }
            this.Description = this.inputDescription;
            this.inputDescription = "";
        } catch (error) {
            console.log("error saving repo description: ", error);
        }
        this.isLoadingSaveDescription = false;
    }
    private async onSaveGitUrlClicked() {
        this.isLoadingSaveGitUrl = true;

        const gitUrlParameterName = "url"
        const formData = new FormData();
        if (!this.inputGitUrl || this.inputGitUrl.trim() === "") {
            alert("Please enter the Git repository URL.");
            this.isLoadingSaveGitUrl = false;
            return;
        }
        formData.append(gitUrlParameterName, this.inputGitUrl)
        try {
            const resp = await fetch(PathToSetGitMirrorUrl(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok){
                alert("Bad URL");
                this.isLoadingSaveGitUrl = false;
                return;
            }
            this.GitMirrorUrl = this.inputGitUrl
            this.inputGitUrl = ""
        } catch (error) {
            console.log("error saving GitHub url: ", error)
        }
        this.isLoadingSaveGitUrl = false;
    }

    private async toggleGitMirrorFired(e){
        e.stopPropagation()
        if (this.IsGitMirrorEnabled === e.detail.Checked){
            throw new Error("invalid state: IsGitMirrorEnabled already equals new value");
        }

        const original = this.IsGitMirrorEnabled 
        this.IsGitMirrorEnabled = e.detail.Checked

        const formData = new FormData();
        formData.append(GitMirrorEnabledParamName, this.IsGitMirrorEnabled ? "on" : "off")
        try {
            const resp = await fetch(PathToSetGitMirrorEnabled(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                console.error("Request to update is git mirror enabled failed: ", resp)
                alert("Failed to update");
                this.IsGitMirrorEnabled = original
                return;
            }
        } catch (error) {
            console.error("error=", error," setting is git mirror enabled to=", this.IsGitMirrorEnabled)
            this.IsGitMirrorEnabled = original
        } 
    }

    private async onCreateSecretClicked() {
        if (this.isLoadingCreateSecretBtn) {
            return
        }
        this.isLoadingCreateSecretBtn = true

        const form = this.renderRoot.querySelector("#create-secret-form-id") as HTMLFormElement
        if (!form){
            console.error("could not find create secret form")
            alert("something is off, please reload page")
            return
        }
        const formData = new FormData(form);

        try {
            const resp = await fetch(PathToSetRepoSecret(this.RepoOwnerName, this.RepoName), {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                console.error("Request to create secret failed: ", resp)
                const errorMsg = await resp.text();
                throw new Error(errorMsg);
            }
            const newSecrets = await resp.json() as Secret;
            this.Secrets = [...this.Secrets, newSecrets]
            this.closeCreateSecretModal()
        } catch (error) {
            console.error("field to create secret: ", error)
            alert(error);
        } finally{
            this.isLoadingCreateSecretBtn = false
        }
    }

    private async onDeleteSecretClicked(secret: Secret) {
        this.markSecretDeleteBtlLoading(secret.Name)

        try {
            const resp = await fetch(
                UrlToDeleteRepoSecret(this.RepoOwnerName, this.RepoName, secret.Name), 
                {method: 'DELETE', headers: GetCsrfHeaders()}
            );
            if (!resp.ok) {
                console.error("Request to delete secret failed: ", resp)
                throw new Error("response was not ok!");
            }
            this.Secrets = this.Secrets.filter(s => s.Id !== secret.Id)
        } catch (error) {
            console.error("failed to delete secret: ", error)
            alert("failed to delete secret, please try again")
        } finally {
            this.unmarkSecretDeleteBtlLoading(secret.Name)
        }
    }

    private onInputKeyDown(e: KeyboardEvent) {
        if (e.key === 'Enter') {
            e.preventDefault();
            this.onInviteClick(e);
        }
    }
    private onInputValueChange(e){
        this.inputValue = e.currentTarget.value;
    };
    private clearInput() {
        this.inputValue = "";
    };

    private async onInviteClick(ev){
        ev.stopPropagation()
        this.isLoading = true;

        const usernameParameterName = "username"
        const notFoundMsg = "not found"
        const hasPermissionMsg = "Already had"
        const okMsg = "ok"
        const formData = new FormData();
        formData.append(usernameParameterName, this.inputValue)
        let url: string = PathToAddRepoPermission(this.RepoOwnerName, this.RepoName)
        try {
            const resp = await fetch(url, {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok){
                const msg = await resp.text();
                alert(msg);
                this.isLoading = false;
                return
            }
            const responseText = await resp.text();
            if (responseText == notFoundMsg){
                alert("Username not found")
            }
            if (responseText == hasPermissionMsg) {
                alert("Username already has permission")
            }
            if (responseText == okMsg){
                
                this.Members = [...this.Members,{
                    Username: this.inputValue,
                    Role: "Read/Write",
                }]
                this.clearInput()
            }

        } catch (error) {
            console.log("error posting invite:", error)
            alert("Failed to post invite");
        }

        this.isLoading = false;

    }

    private getRemoveMemberFunc(i: number): () => void{
        return ()=>{
            this.removeMember(i)
        }
    }
    private async removeMember(i: number) {
        const usernameParameterName = "username"
        const okMsg = "ok"
        const notFoundMsg = "not found"

        const formData = new FormData();
        formData.append(usernameParameterName, this.Members[i].Username)
        let url: string = PathToRemoveRepoPermission(this.RepoOwnerName, this.RepoName)
        try {
            const resp = await fetch(url, {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                const msg = await resp.text();
                alert(msg);
                return
            }
            const responseText = await resp.text();
            if (responseText == notFoundMsg) {
                alert("username not found")
            }
            if (responseText == okMsg) {
                this.Members = this.Members.filter((_, idx) => idx !== i);
            }
        } catch (error) {
            console.log("error posting invite:", error)
        }
    }

    private async postArchive(){
        const okMsg = "ok"
        this.isLoadingArchive = true
        let url: string = PathToArchiveRepo(this.RepoOwnerName, this.RepoName)
        try {
            const resp = await fetch(url, {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            if (resp.ok) {
                const responseText = await resp.text();
                if (responseText == okMsg) {
                    window.location.href = HomeUrl
                } else {
                    console.log("unexpected response from archive: ",
                        responseText)
                }
            }else{
                console.log("got non ok response from post archive")
            }
        } catch (error) {
            console.log("error posting archive:", error)
        }
        this.isLoadingArchive = false
    }

    private markSecretDeleteBtlLoading(name: string) {
        if (this.secretsWithDeleteBtnLoading.includes(name)) return;

        this.secretsWithDeleteBtnLoading = [
            ...this.secretsWithDeleteBtnLoading,
            name,
        ];
    }
    private unmarkSecretDeleteBtlLoading(name: string) {
        this.secretsWithDeleteBtnLoading =
            this.secretsWithDeleteBtnLoading.filter(n => n !== name);
    }

    static styles = [
        TwiggCss,
        css`
            .main { max-width: var(--size4); margin: 0 auto; }

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

            .btn-archive {
                border-color: var(--color-danger);
            }
            #archive-btn-container{
                margin-top: var(--space4);
                display: flex;
                justify-content: end;
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
            
            .git-https-input, .description-input {
                width: 100%;
            }
            .save-btn-container{
                margin-top: var(--space1);
                display: flex;
                align-items: center;
            }
            .btn-save {
                margin-left: auto;
            }
            .git-mirror-docs-anchor{
                display: inline-flex;
                align-items: center;
                color: var(--color-primary)
            }
            .repo-url, .current-description {
                font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
                color: var(--color-text);
                background: var(--color-surface-alt);
                padding: var(--fixedSpace1) var(--fixedSpace2);
                border-radius: var(--radius1);
                margin-bottom: var(--fixedSpace2);
                white-space: normal; 
                overflow-wrap: anywhere;
            }

            .flex-container {
                display: flex;
                align-items: center; 
                justify-content: start;
                gap: 1rem;
            }
            .is-disabled {
                opacity: var(--disable-opacity-value);
            }
            .visibility-tag {
                background: var(--color-surface-alt);
                border: 1px solid var(--color-border);
                border-radius: var(--radius1);
                padding: var(--fixedSpace1) var(--fixedSpace2);
                font-weight: var(--weight-semi-bold);
            }
            .error-message {
                color: var(--color-danger);
                font-size: var(--space3);
                display: none;
            }
            .error-message.visible{
                display: inline
            }
            .input-error {
                border: 1px solid var(--color-danger);
            }
            .form-of-create-secret-modal{
                text-align: left;
            }
            .form-of-create-secret-modal input{
                width: 100%;
            }
            .form-of-create-secret-modal textarea{
                resize: vertical;
                min-width: var(--size0);
                min-height: var(--fixedSpace8);
                border: 1px solid var(--color-border);
                border-radius: var(--radius1);
                font-family: monospace;
                overflow-wrap: break-word;
            }
        `,
    ];
}

customElements.define('repo-settings', RepoSettings);

declare global {
    interface HTMLElementTagNameMap {
        'repo-settings': RepoSettings;
    }
}